package controllers

import (
	"context"
	"fmt"
	cnsbench "github.com/cnsbench/cnsbench/api/v1alpha1"
	"io/ioutil"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	utilptr "k8s.io/utils/pointer"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

func CleanupScalePods(c client.Client) error {
	ls := &metav1.LabelSelector{}
	ls = metav1.AddLabelToSelector(ls, "app", "scale-pod")

	selector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return err
	}
	pods := &corev1.PodList{}
	if err := c.List(context.TODO(), pods, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
		return err
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == "Succeeded" {
			if err := c.Delete(context.TODO(), &pod); err != nil {
				fmt.Println("Error deleting scaling pod", err)
			}
		}
	}

	return nil
}

func podSpec(obj client.Object) (*corev1.PodSpec, error) {
	kind, err := meta.NewAccessor().Kind(obj)
	if err != nil {
		return nil, err
	}
	if kind == "Job" {
		pt := *obj.(*batchv1.Job)
		return &pt.Spec.Template.Spec, nil
	} else if kind == "Pod" {
		pt := *obj.(*corev1.Pod)
		return &pt.Spec, nil
	} else if kind == "StatefulSet" {
		pt := *obj.(*appsv1.StatefulSet)
		return &pt.Spec.Template.Spec, nil
	}
	return nil, nil
}

func updatePodSpec(obj client.Object, spec corev1.PodSpec) (client.Object, error) {
	kind, err := meta.NewAccessor().Kind(obj)
	if err != nil {
		return nil, err
	}
	if kind == "Job" {
		pt := *obj.(*batchv1.Job)
		pt.Spec.Template.Spec = spec
		return client.Object(&pt), nil
	} else if kind == "Pod" {
		pt := *obj.(*corev1.Pod)
		pt.Spec = spec
		return client.Object(&pt), nil
	} else if kind == "StatefulSet" {
		pt := *obj.(*appsv1.StatefulSet)
		pt.Spec.Template.Spec = spec
		return client.Object(&pt), nil
	}
	return nil, nil
}

func volInSpec(vols []corev1.Volume, name string) bool {
	for _, v := range vols {
		if v.Name == name {
			return true
		}
	}
	return false
}

func newCMVol(name, cmName string) corev1.Volume {
	vol := corev1.Volume{}
	vol.Name = name
	cm := corev1.ConfigMapVolumeSource{}
	cm.DefaultMode = utilptr.Int32Ptr(0777)
	cm.Name = cmName
	vol.ConfigMap = &cm
	return vol
}

func addOutputVol(spec *corev1.PodSpec) {
	if !volInSpec(spec.Volumes, "output-vol") {
		outputVol := corev1.Volume{}
		outputVol.Name = "output-vol"
		outputVol.EmptyDir = &corev1.EmptyDirVolumeSource{}
		spec.Volumes = append(spec.Volumes, outputVol)
	}

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
 */
func (r *BenchmarkReconciler) createTmpConfigMap(bm *cnsbench.Benchmark, scriptName string) (string, error) {
	script, err := r.loadScript(scriptName)
	if err != nil {
		r.Log.Error(err, "Error creating tmp configmap for "+scriptName)
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

func (r *BenchmarkReconciler) AddParserContainer(bm *cnsbench.Benchmark, obj client.Object, parserCMName, logFilename, imageName string, num int) (client.Object, error) {
	spec, err := podSpec(obj)
	if spec == nil || err != nil {
		return nil, err
	}

	c := corev1.Container{}
	c.Name = "parser-container"
	c.Image = imageName
	// The countdone script counts how many non-init containers have finished in the
	// parser container's pod.  The first argument indicates how many need to finish
	// before moving on and running the parser.  This assumes that the workload pod
	// will consist of a single container that runs to completion.
	c.Command = []string{"sh", "-c", "/scripts/countdone.sh 1 && /scripts/parser/* " + logFilename + " > " + logFilename + ".parsed"}
	//c.Command = []string{"sh", "-c", "tail -f /dev/null"}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/scripts/countdone.sh",
			SubPath:   "countdone.sh",
			Name:      "helper",
		},
		{
			MountPath: "/scripts/parser",
			Name:      "parser",
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
	spec.Containers = append(spec.Containers, c)

	if !volInSpec(spec.Volumes, "parser") {
		// If we have more than one output file we could have multiple parsers
		// For now, just assume we only have one file so we can just name the
		// vol "parser"
		spec.Volumes = append(spec.Volumes, newCMVol("parser", parserCMName))
	}

	if !volInSpec(spec.Volumes, "helper") {
		if cmName, err := r.createTmpConfigMap(bm, "countdone.sh"); err != nil {
			return obj, err
		} else {
			spec.Volumes = append(spec.Volumes, newCMVol("helper", cmName))
		}
	}

	addOutputVol(spec)

	return updatePodSpec(obj, *spec)
}

func (r *BenchmarkReconciler) AddSyncContainer(bm *cnsbench.Benchmark, obj client.Object, count int, workloadName string, syncGroup string) (client.Object, error) {
	spec, err := podSpec(obj)
	if spec == nil || err != nil {
		return nil, err
	}

	numContainers := len(spec.InitContainers)

	c := corev1.Container{}
	c.Name = "sync-container"
	c.Image = "dwdraju/alpine-curl-jq"
	c.Command = []string{"/scripts/ready.sh", "workloadname%3D" + workloadName, strconv.Itoa(numContainers * count)}
	if syncGroup != "" {
		c.Command = append(c.Command, "syncgroup%3D"+syncGroup)
	}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/scripts/",
			Name:      "ready-script",
		},
	}
	spec.InitContainers = append(spec.InitContainers, c)

	if cmName, err := r.createTmpConfigMap(bm, "ready.sh"); err != nil {
		return obj, err
	} else {
		spec.Volumes = append(spec.Volumes, newCMVol("ready-script", cmName))
	}

	return updatePodSpec(obj, *spec)
}

func (r *BenchmarkReconciler) AddOutputContainer(bm *cnsbench.Benchmark, obj client.Object, outputArgs, outputContainer, outputFilename string) (client.Object, error) {
	spec, err := podSpec(obj)
	if spec == nil || err != nil {
		return nil, err
	}

	c := corev1.Container{}
	c.Name = "output-container"
	c.Image = "cnsbench/utility:latest"
	// See comment for parser container
	c.Command = []string{"sh", "-c", "/scripts/countdone.sh 2 && /scripts/output.sh " + outputFilename + ".parsed " + outputArgs}
	//c.Command = []string{"sh", "-c", "tail -f /dev/null"}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/scripts/countdone.sh",
			SubPath:   "countdone.sh",
			Name:      "helper",
		},
		{
			MountPath: "/scripts/output.sh",
			SubPath:   "output.sh",
			Name:      "output",
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
	spec.Containers = append(spec.Containers, c)

	if !volInSpec(spec.Volumes, "output") {
		spec.Volumes = append(spec.Volumes, newCMVol("output", outputContainer))
	}

	// This should have already been done by AddParserContainer, but just in case...
	if !volInSpec(spec.Volumes, "helper") {
		if cmName, err := r.createTmpConfigMap(bm, "countdone.sh"); err != nil {
			return obj, err
		} else {
			spec.Volumes = append(spec.Volumes, newCMVol("helper", cmName))
		}
	}

	addOutputVol(spec)

	return updatePodSpec(obj, *spec)
}

func (r *BenchmarkReconciler) AddLabelsGeneric(obj client.Object, labels map[string]string) (client.Object, error) {
	kind, err := meta.NewAccessor().Kind(obj)
	if err != nil {
		return nil, err
	}
	if kind == "Job" {
		pt := *obj.(*batchv1.Job)
		if pt.Spec.Template.ObjectMeta.Labels != nil {
			for k, v := range pt.Spec.Template.ObjectMeta.Labels {
				labels[k] = v
			}
		}
		addLabels(&pt.Spec.Template.ObjectMeta, labels)
		return client.Object(&pt), nil
	} else if kind == "StatefulSet" {
		pt := *obj.(*appsv1.StatefulSet)
		addLabels(&pt.Spec.Template.ObjectMeta, labels)
		return client.Object(&pt), nil
	}
	return obj, nil
}

func addLabels(spec *metav1.ObjectMeta, labels map[string]string) {
	spec.Labels = labels
}

func (r *BenchmarkReconciler) SetEnvVar(name, value string, obj client.Object) (client.Object, error) {
	kind, err := meta.NewAccessor().Kind(obj)
	if err != nil {
		return nil, err
	}

	if kind == "Job" {
		for n, _ := range obj.(*batchv1.Job).Spec.Template.Spec.InitContainers {
			obj.(*batchv1.Job).Spec.Template.Spec.InitContainers[n].Env = append(obj.(*batchv1.Job).Spec.Template.Spec.InitContainers[n].Env, corev1.EnvVar{Name: name, Value: value})
		}
		for n, _ := range obj.(*batchv1.Job).Spec.Template.Spec.Containers {
			obj.(*batchv1.Job).Spec.Template.Spec.Containers[n].Env = append(obj.(*batchv1.Job).Spec.Template.Spec.Containers[n].Env, corev1.EnvVar{Name: name, Value: value})
		}
	} else if kind == "Pod" {
		for n, _ := range obj.(*corev1.Pod).Spec.InitContainers {
			obj.(*corev1.Pod).Spec.InitContainers[n].Env = append(obj.(*corev1.Pod).Spec.InitContainers[n].Env, corev1.EnvVar{Name: name, Value: value})
		}
		for n, _ := range obj.(*corev1.Pod).Spec.Containers {
			obj.(*corev1.Pod).Spec.Containers[n].Env = append(obj.(*corev1.Pod).Spec.Containers[n].Env, corev1.EnvVar{Name: name, Value: value})
		}
	}

	return obj, nil
}
