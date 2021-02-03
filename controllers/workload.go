package controllers

import (
	"context"
	"io/ioutil"
	"path"
	"strconv"
	"strings"

	cnsbench "github.com/cnsbench/cnsbench/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes/scheme"
	utilptr "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/cnsbench/cnsbench/pkg/podutils"
)

/* Helper functions for modifying a workload pod (e.g. adding the emptyDir output
 * volume, the parser container, etc.)
 */
func (r *BenchmarkReconciler) volInSpec(vols []corev1.Volume, name string) bool {
	for _, v := range vols {
		if v.Name == name {
			return true
		}
	}
	return false
}

func (r *BenchmarkReconciler) newCMVol(name, cmName string) corev1.Volume {
	vol := corev1.Volume{}
	vol.Name = name
	cm := corev1.ConfigMapVolumeSource{}
	cm.DefaultMode = utilptr.Int32Ptr(0777)
	cm.Name = cmName
	vol.ConfigMap = &cm
	return vol
}

func (r *BenchmarkReconciler) buildContainer(containerName, imageName, cmdline string) corev1.Container {
	c := corev1.Container{}
	c.Name = containerName
	c.Image = imageName
	c.Command = []string{"sh", "-c", cmdline}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/scripts/countdone.sh",
			SubPath:   "countdone.sh",
			Name:      "helper",
		},
		{
			MountPath: "/var/run/secrets/kubernetes.io/podwatcher",
			Name:      "pod-watcher-token",
		},
		{
			MountPath: "/output",
			Name:      "output-vol",
		},
	}
	c.Env = []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	}

	return c
}

/* TODO: Figure out how to cache this so we don't read these files each time they're
 * added to a pod
 */
func (r *BenchmarkReconciler) loadScript(scriptName string) (string, error) {
	if b, err := ioutil.ReadFile(path.Join(r.ScriptsDir, scriptName)); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}

/* We use some helper scripts (e.g. countdown.sh) that run in the parser and output containers.
 * These containers are in pods that are in the default namespace, so the scripts like countdown.sh
 * must be in configmaps that are stored in the default namespace.  To avoid polluting the default
 * namespace, we create the configmaps for these scripts on demand and delete them when the benchmark
 * completes.
 *
 * cloneParser() also creates a temp config map, but it clones a config map from the cnsbench-library
 * namespace rather than one on disk
 */
func (r *BenchmarkReconciler) createTmpConfigMapFromDisk(bm *cnsbench.Benchmark, scriptName string) (string, error) {
	script, err := r.loadScript(scriptName)
	if err != nil {
		r.Log.Error(err, "Error creating tmp configmap", "script", scriptName)
		return "", err
	}
	newCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.NameGenerator.GenerateName(names.SimpleNameGenerator, "helper-"),
			Namespace: "default",
		},
		Data: map[string]string{
			scriptName: script,
		},
	}

	if err := r.createObj(bm, client.Object(&newCm), true); err != nil {
		r.Log.Error(err, "Creating temp ConfigMap for countdown.sh")
	}

	return newCm.ObjectMeta.Name, err
}

//////////////////////////////////////////////////////////

/* 1. Create a config map with the parser script in the default namespace,
 *    where the workload will run
 * 2. Get the name of the container image that the parser script will run in
 * 3. Add the parser container to the workload object
 */
func (r *BenchmarkReconciler) addParserContainer(bm *cnsbench.Benchmark, obj client.Object, parser string, outfile string, num int) (client.Object, error) {
	if parser == "" {
		parser = "null-parser"
	}

	if tmpCmName, err := r.cloneParser(bm, parser); err != nil {
		r.Log.Error(err, "Error adding parser container", "parser", parser)
		return obj, err
	} else {
		imageName, err := r.getParserContainerImage(parser)
		if err != nil {
			r.Log.Error(err, "Error getting parser container image", "parser", parser)
			return obj, err
		}
		r.Log.Info("Creating parser", "cmname", parser, "image", imageName)
		obj, err = r.addParserContainerToObj(bm, obj, tmpCmName, outfile, imageName, num)
		if err != nil {
			r.Log.Error(err, "Error adding parser container", "outfile", outfile)
			return obj, err
		}
		r.Log.Info("Created temp parser", "name", tmpCmName)
	}
	return obj, nil
}

func (r *BenchmarkReconciler) getParserContainerImage(parserName string) (string, error) {
	parserCm := &corev1.ConfigMap{}

	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: parserName, Namespace: LIBRARY_NAMESPACE}, parserCm)
	if err != nil {
		r.Log.Error(err, "Error getting ConfigMap", "spec", parserName)
		return "", err
	}

	// The parser ConfigMap should indicate what container image to use to run the parser, but
	// default to busybox if none is specified
	if imageName, exists := parserCm.ObjectMeta.Annotations["container"]; !exists {
		r.Log.Info("Container annotation does not exist for parser", "parser", parserName)
		return "busybox", nil
	} else {
		return imageName, nil
	}
}

/* The ConfigMaps that contain the parsers are in the library namespace, but the workload pods that
 * will need access to the parsers are created in the default namespace.  Since pods can't attach to
 * ConfigMaps in different namespaces, we create a copy of the parser ConfigMap in the default namespace
 */
func (r *BenchmarkReconciler) cloneParser(bm *cnsbench.Benchmark, parserName string) (string, error) {
	parserCm := &corev1.ConfigMap{}

	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: parserName, Namespace: LIBRARY_NAMESPACE}, parserCm)
	if err != nil {
		r.Log.Error(err, "Error getting ConfigMap", "spec", parserName)
		return "", err
	}

	newCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.NameGenerator.GenerateName(names.SimpleNameGenerator, parserName+"-"),
			Namespace: "default",
		},
		Data: make(map[string]string, 0),
	}
	for k, v := range parserCm.Data {
		newCm.Data[k] = v
	}

	if err := r.createObj(bm, client.Object(&newCm), true); err != nil {
		r.Log.Error(err, "Creating temp ConfigMap")
	}

	return newCm.ObjectMeta.Name, err
}

func (r *BenchmarkReconciler) addParserContainerToObj(bm *cnsbench.Benchmark, obj client.Object, parserCMName, logFilename, imageName string, num int) (client.Object, error) {
	spec, err := podutils.PodSpec(obj)
	if err != nil {
		r.Log.Error(err, "podSpec")
		return nil, err
	}

	cmdline := "/scripts/countdone.sh 1 && /scripts/parser/* " + logFilename + " > " + logFilename + ".parsed"
	container := r.buildContainer("parser-container", imageName, cmdline)
	parserMount := corev1.VolumeMount{MountPath: "/scripts/parser", Name: "parser"}
	container.VolumeMounts = append(container.VolumeMounts, parserMount)
	spec.Containers = append(spec.Containers, container)

	if !r.volInSpec(spec.Volumes, "parser") {
		spec.Volumes = append(spec.Volumes, r.newCMVol("parser", parserCMName))
	}

	return podutils.UpdatePodSpec(obj, *spec)
}

//////////////////////////////////////////////////////////

func (r *BenchmarkReconciler) addOutputContainer(bm *cnsbench.Benchmark, obj client.Object, outputName string, outputFile string) (client.Object, error) {
	// Default to sending to cnsbench-output-collector
	//outputScript := "null-output.sh"
	outputScript := "http-output.sh"
	outputArgs := "http://cnsbench-output-collector.cnsbench-system.svc.cluster.local:8888/" + bm.ObjectMeta.Name + "/"

	// If no output sink specified
	if outputName == "" {
		outputName = bm.Spec.AllWorkloadOutput
	}
	for _, output := range bm.Spec.Outputs {
		if output.Name == outputName {
			r.Log.Info("Matched an output", "output", output)
			if output.HttpPostSpec.URL != "" {
				outputScript = "http-output.sh"
				outputArgs = output.HttpPostSpec.URL
			}
		}
	}

	obj, err := r.addOutputContainerToObj(bm, obj, outputArgs, outputScript, outputFile)
	if err != nil {
		r.Log.Error(err, "Error adding output container", "outfile", outputFile)
		return obj, err
	}
	r.Log.Info("Added output container", "name", outputFile)

	return obj, nil
}

func (r *BenchmarkReconciler) addOutputContainerToObj(bm *cnsbench.Benchmark, obj client.Object, outputArgs, outputScript, outputFilename string) (client.Object, error) {
	spec, err := podutils.PodSpec(obj)
	if err != nil {
		r.Log.Error(err, "podSpec")
		return nil, err
	}
	cmdline := "/scripts/countdone.sh 2 && /scripts/" + outputScript + " " + outputFilename + ".parsed " + outputArgs
	container := r.buildContainer("output-container", "cnsbench/utility:latest", cmdline)
	outputScriptMount := corev1.VolumeMount{MountPath: "/scripts/" + outputScript, Name: "output", SubPath: outputScript}
	container.VolumeMounts = append(container.VolumeMounts, outputScriptMount)
	spec.Containers = append(spec.Containers, container)

	if cmName, err := r.createTmpConfigMapFromDisk(bm, outputScript); err != nil {
		return obj, err
	} else {
		spec.Volumes = append(spec.Volumes, r.newCMVol("output", cmName))
	}

	return podutils.UpdatePodSpec(obj, *spec)
}

//////////////////////////////////////////////////////////

func (r *BenchmarkReconciler) addOutputVol(obj client.Object) (client.Object, error) {
	spec, err := podutils.PodSpec(obj)
	if err != nil {
		r.Log.Error(err, "podSpec")
		return nil, err
	}

	if r.volInSpec(spec.Volumes, "output-vol") {
		return obj, nil
	}

	outputVol := corev1.Volume{}
	outputVol.Name = "output-vol"
	outputVol.EmptyDir = &corev1.EmptyDirVolumeSource{}
	spec.Volumes = append(spec.Volumes, outputVol)

	// Make sure every container mounts this volume
	outputVolMount := corev1.VolumeMount{
		MountPath: "/output",
		Name:      "output-vol",
	}
	for i, _ := range spec.Containers {
		foundMount := false
		for _, v := range spec.Containers[i].VolumeMounts {
			if v.Name == "output-vol" {
				foundMount = true
			}
		}
		if !foundMount {
			spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, outputVolMount)
		}
	}

	return podutils.UpdatePodSpec(obj, *spec)
}

//////////////////////////////////////////////////////////

func (r *BenchmarkReconciler) addCountdoneScript(bm *cnsbench.Benchmark, obj client.Object) (client.Object, error) {
	spec, err := podutils.PodSpec(obj)
	if err != nil {
		r.Log.Error(err, "podSpec")
		return nil, err
	}

	if r.volInSpec(spec.Volumes, "helper") {
		return obj, nil
	}

	if cmName, err := r.createTmpConfigMapFromDisk(bm, "countdone.sh"); err != nil {
		return obj, err
	} else {
		spec.Volumes = append(spec.Volumes, r.newCMVol("helper", cmName))
	}

	return podutils.UpdatePodSpec(obj, *spec)
}

//////////////////////////////////////////////////////////

func (r *BenchmarkReconciler) addPodWatcherToken(obj client.Object) (client.Object, error) {
	spec, err := podutils.PodSpec(obj)
	if err != nil {
		r.Log.Error(err, "podSpec")
		return nil, err
	}

	if r.volInSpec(spec.Volumes, "pod-watcher-token") {
		return obj, err
	}

	vol := corev1.Volume{}
	vol.Name = "pod-watcher-token"
	vol.Secret = &corev1.SecretVolumeSource{
		SecretName: "pod-watcher-token",
	}

	spec.Volumes = append(spec.Volumes, vol)

	// All of the containers that need this token use buildContainer() to create the
	// container object, so we don't need to add any VolumeMounts here like we did
	// with the output volume

	return podutils.UpdatePodSpec(obj, *spec)
}

//////////////////////////////////////////////////////////

func (r *BenchmarkReconciler) replaceVars(cmString string, workloadSpec cnsbench.Workload, instanceNum, numInstances int, workloadName string, workloadConfigMap *corev1.ConfigMap) string {
	for variable, value := range workloadSpec.Vars {
		r.Log.Info("Searching for var", "var", variable, "replacement", value)
		cmString = strings.ReplaceAll(cmString, "{{"+variable+"}}", value)
	}

	// Use workload spec's annotations as default values for any remaining variables
	for k, v := range workloadConfigMap.ObjectMeta.Annotations {
		s := strings.SplitN(k, ".", 3)
		if len(s) == 3 && s[0] == "cnsbench" && s[1] == "default" {
			r.Log.Info("Searching for var", "var", s[2], "default replacement", v)
			cmString = strings.ReplaceAll(cmString, "{{"+s[2]+"}}", v)
		}
	}
	cmString = strings.ReplaceAll(cmString, "{{ACTION_NAME}}", workloadName)
	cmString = strings.ReplaceAll(cmString, "{{ACTION_NAME_CAPS}}", strings.ToUpper(workloadName))
	cmString = strings.ReplaceAll(cmString, "{{INSTANCE_NUM}}", strconv.Itoa(instanceNum))
	cmString = strings.ReplaceAll(cmString, "{{NUM_INSTANCES}}", strconv.Itoa(numInstances))

	if strings.Contains(cmString, "{{") {
		r.Log.Info("Object definition contains double curly brackets ({{), possible unset variable", "cmstring", cmString)
	}

	return cmString
}

func (r *BenchmarkReconciler) decodeConfigMap(cmString string) (client.Object, error) {
	// Decode the yaml object from the workload spec
	objBytes := []byte(cmString)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	robj, _, err := decode(objBytes, nil, nil)
	if err != nil {
		r.Log.Info("cm", "cm", cmString)
		r.Log.Error(err, "Error decoding yaml")
		return nil, err
	}
	obj := robj.(client.Object)

	return obj, nil
}

func (r *BenchmarkReconciler) getRole(annotations map[string]string) string {
	if _, exists := annotations["role"]; exists {
		return annotations["role"]
	}
	// if no role is set, consider the object a helper (e.g., PVC, ConfigMap)
	return "helper"
}

func (r *BenchmarkReconciler) setupForOutput(bm *cnsbench.Benchmark, obj client.Object) (client.Object, error) {
	var err error
	// Add emptydir for output, will be mounted by all containers in workload pod
	if obj, err = r.addOutputVol(obj); err != nil {
		return obj, err
	}
	// Add volume for the token that allows helper containers to query api server
	if obj, err = r.addPodWatcherToken(obj); err != nil {
		return obj, err
	}
	// Add volumes for the helper script config maps
	if obj, err = r.addCountdoneScript(bm, obj); err != nil {
		return obj, err
	}
	return obj, nil
}

func (r *BenchmarkReconciler) addContainers(bm *cnsbench.Benchmark, obj client.Object, annotations map[string]string, workloadSpec cnsbench.Workload) (client.Object, error) {
	var err error
	role := r.getRole(annotations)

	// If user is specifying output files, use those.  Otherwise, use the
	// defaults from the object's annotations
	if len(workloadSpec.OutputFiles) > 0 {
		// User can specify multiple files to get output from
		for i, output := range workloadSpec.OutputFiles {
			if role == output.Target || (role == "workload" && output.Target == "") {
				if obj, err = r.setupForOutput(bm, obj); err != nil {
					r.Log.Error(err, "Error setting up for output")
					return obj, nil
				}

				// XXX: Since the parser might be packaged with the current workload, need to make sure
				// the parser object is seen before any objects referencing it in an Output
				if obj, err = r.addParserContainer(bm, obj, output.Parser, output.Filename, i); err != nil {
					r.Log.Error(err, "Error adding parser container")
					return obj, err
				}

				if obj, err = r.addOutputContainer(bm, obj, output.Sink, output.Filename); err != nil {
					r.Log.Error(err, "Error adding output container")
					return obj, err
				}
			}
		}
	} else if _, exists := annotations["outputFile"]; exists {
		if obj, err = r.setupForOutput(bm, obj); err != nil {
			r.Log.Error(err, "Error setting up for output")
			return obj, nil
		}

		// TODO: Allow more than one default output file
		if obj, err = r.addParserContainer(bm, obj, annotations["parser"], annotations["outputFile"], 0); err != nil {
			r.Log.Error(err, "Error adding parser container")
			return obj, err
		}
		if obj, err = r.addOutputContainer(bm, obj, bm.Spec.AllWorkloadOutput, annotations["outputFile"]); err != nil {
			r.Log.Error(err, "Error adding output container")
			return obj, err
		}
	}

	return obj, nil
}

func (r *BenchmarkReconciler) getCount(count int) int {
	if count == 0 {
		return 1
	}
	return count
}

func (r *BenchmarkReconciler) addCNSBLabels(workloadSpec cnsbench.Workload, obj client.Object, annotations map[string]string) (client.Object, error) {
	// Add workloadname and multiinstance labels to object
	accessor := meta.NewAccessor()
	labels, err := accessor.Labels(obj)
	if err != nil {
		r.Log.Error(err, "Error getting labels")
		return nil, err
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	if workloadSpec.SyncGroup != "" {
		labels["syncgroup"] = workloadSpec.SyncGroup
	}
	labels["workloadname"] = workloadSpec.Name //workloadName
	labels["role"] = r.getRole(annotations)

	/*
		var multipleInstanceObjs []string
		if mis, found := cm.ObjectMeta.Annotations["multipleInstances"]; found {
			multipleInstanceObjs = strings.Split(mis, ",")
		}
		if utils.Contains(multipleInstanceObjs, k) {
			labels["multiinstance"] = "true"
		}
	*/

	r.Log.Info("labels", "labels", labels)

	accessor.SetLabels(obj, labels)
	obj, err = podutils.AddLabelsGeneric(obj, labels)
	if err != nil {
		r.Log.Error(err, "Error updating workload labels")
	}

	return obj, err
}

func (r *BenchmarkReconciler) prepareAndRun(bm *cnsbench.Benchmark, w int, k string, workloadName string, a cnsbench.Workload, cm *corev1.ConfigMap, objBytes []byte) error {
	var err error
	var obj client.Object
	var objAnnotations map[string]string
	accessor := meta.NewAccessor()

	count := r.getCount(a.Count)

	// Replace vars in workload spec with values from benchmark object
	cmString := r.replaceVars(string(objBytes), a, w, count, workloadName, cm)
	if obj, err = r.decodeConfigMap(cmString); err != nil {
		return err
	}

	if objAnnotations, err = accessor.Annotations(obj); err != nil {
		r.Log.Error(err, "Error getting object annotations")
		return err
	}

	// If this object is a volume but the user supplied a non default volume
	// name, skip creating this object
	nonDefaultVol, _ := a.Vars["volname"]
	nonDefaultVolBool, _ := strconv.ParseBool(nonDefaultVol)
	if r.getRole(objAnnotations) == "volume" && nonDefaultVolBool {
		r.Log.Info("This is volume but non default volume name supplied, skipping")
		return nil
	}

	// Add containers for parsing and outputting
	if obj, err = r.addContainers(bm, obj, objAnnotations, a); err != nil {
		return err
	}

	// Set env variables in workload container
	podutils.SetEnvVar("INSTANCE_NUM", strconv.Itoa(w), obj)
	podutils.SetEnvVar("NUM_INSTANCES", strconv.Itoa(count), obj)

	// Add workloadname and multiinstance labels to object
	if obj, err = r.addCNSBLabels(a, obj, objAnnotations); err != nil {
		return err
	}

	// Add sync container if sync start requested
	if _, exists := objAnnotations["sync"]; exists {
		if obj, err = r.addSyncContainer(bm, obj, a.Count, workloadName, a.SyncGroup); err != nil {
			return err
		}
	}

	// Ownership can't transcend namespaces
	makeOwner := true
	if ns, _ := accessor.Namespace(obj); ns != "default" {
		makeOwner = false
	}

	// Make the actual object
	name, _ := accessor.Name(obj)
	//kind, _ := accessor.Kind(obj)
	if err := r.createObj(bm, obj, makeOwner); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		} else {
			r.Log.Info("Already exists", "name", name)
			return nil
		}
	}

	return nil
}

func (r *BenchmarkReconciler) addSyncContainer(bm *cnsbench.Benchmark, obj client.Object, count int, workloadName string, syncGroup string) (client.Object, error) {
	spec, err := podutils.PodSpec(obj)
	if err != nil {
		r.Log.Error(err, "podSpec")
		return nil, err
	}

	numContainers := len(spec.InitContainers)

	c := corev1.Container{}
	c.Name = "sync-container"
	c.Image = "cnsbench/utility:latest"
	c.Command = []string{"/scripts/ready.sh", "workloadname%3D" + workloadName, strconv.Itoa(numContainers * count)}
	if syncGroup != "" {
		c.Command = append(c.Command, "syncgroup%3D"+syncGroup)
	}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/scripts/",
			Name:      "ready-script",
		},
		{
			MountPath: "/var/run/secrets/kubernetes.io/podwatcher",
			Name:      "pod-watcher-token",
		},
	}
	spec.InitContainers = append(spec.InitContainers, c)

	// Add volume for the token that allows helper containers to query api server
	if obj, err = r.addPodWatcherToken(obj); err != nil {
		return obj, err
	}

	if cmName, err := r.createTmpConfigMapFromDisk(bm, "ready.sh"); err != nil {
		return obj, err
	} else {
		spec.Volumes = append(spec.Volumes, r.newCMVol("ready-script", cmName))
	}

	return podutils.UpdatePodSpec(obj, *spec)
}
