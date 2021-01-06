package benchmark

import (
	"fmt"
	"context"
	"strconv"
	"sort"
	"strings"

	cnsbench "github.com/cnsbench/pkg/apis/cnsbench/v1alpha1"
	"github.com/cnsbench/pkg/utils"

        //appsv1 "k8s.io/api/apps/v1"
        corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"k8s.io/apimachinery/pkg/runtime"
	snapshotscheme "github.com/kubernetes-csi/external-snapshotter/v2/pkg/client/clientset/versioned/scheme"
	"k8s.io/apiserver/pkg/storage/names" 
	snapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	utilptr "k8s.io/utils/pointer"
)

func (r *ReconcileBenchmark) createObj(bm *cnsbench.Benchmark, obj runtime.Object, makeOwner bool) error {
	name, err := meta.NewAccessor().Name(obj)

	objMeta, err := meta.Accessor(obj)
	if err != nil {
		log.Error(err, "Error getting ObjectMeta from obj", name)
		return err
	}

	if makeOwner {
		if err := controllerutil.SetControllerReference(bm, objMeta, r.scheme); err != nil {
			log.Error(err, "Error making object child of Benchmark", "name", name)
			return err
		}
		if err := controllerutil.SetOwnerReference(bm, objMeta, r.scheme); err != nil {
			log.Error(err, "Error making object child of Benchmark")
			return err
		}
	}

	for _, x := range scheme.Codecs.SupportedMediaTypes() {
		if x.MediaType == "application/yaml" {
			ptBytes, err := runtime.Encode(x.Serializer, obj)
			if err != nil {
				log.Error(err, "Error encoding spec")
			}
			fmt.Println(string(ptBytes))
		}
	}

	if err := r.client.Create(context.TODO(), obj); err != nil {
		if errors.IsAlreadyExists(err) {
			log.Info("Object already exists, proceeding", "name", name)
		} else {
			return err
		}
	}

	err = r.controller.Watch(&source.Kind{Type: obj}, &handler.EnqueueRequestForOwner{
		IsController: false,
		OwnerType: &cnsbench.Benchmark{},
	})
	if err != nil {
		log.Error(err, "Watching")
		return err
	}

	return nil
}

func (r *ReconcileBenchmark) CreateVolume(bm *cnsbench.Benchmark, vol cnsbench.Volume) {
	var count int
	if vol.Count == 0 {
		count = 1
	} else {
		count = vol.Count
	}

	// XXX This might be called because a rate fired, in which case there might
	// already be a volume - need to check what the last volume number is and
	// count from <last vol> to count+<last vol>
	for c := 0; c < count; c++ {
		name := vol.Name
		if count > 1 {
			name += "-" + strconv.Itoa(c)
		}
		pvc := corev1.PersistentVolumeClaim {
			ObjectMeta: metav1.ObjectMeta {
				Name: name,
				Namespace: "default",
				Labels: map[string]string {
					"volumename": vol.Name,
				},
			},
			Spec: vol.Spec,
		}

		if err := r.createObj(bm, runtime.Object(&pvc), true); err != nil {
			log.Error(err, "Creating volume")
		}
	}
}

func (r *ReconcileBenchmark) RunInstance(bm *cnsbench.Benchmark, cm *corev1.ConfigMap, multipleInstanceObjs []string, instanceNum int, workload cnsbench.Workload) ([]utils.NameKind, error) {
	ret := []utils.NameKind{}
	for k := range cm.Data {
		log.Info("uh")
		if !utils.Contains(multipleInstanceObjs, k) {
			log.Info("Continuing")
			continue
		}
		nk, err := r.prepareAndRun(bm, instanceNum, k, workload.Name, workload, cm, []byte(cm.Data[k]))
		if err != nil {
			return ret, err
		}
		if nk != (utils.NameKind{}) {
			ret = append(ret, nk)
		}
	}

	return ret, nil
}

func (r *ReconcileBenchmark) RunWorkload(bm *cnsbench.Benchmark, a cnsbench.Workload, workloadName string) ([]utils.NameKind, error) {
	cm := &corev1.ConfigMap{}

	err := r.client.Get(context.TODO(), client.ObjectKey{Name: a.Workload, Namespace: "library"}, cm)
	if err != nil {
		log.Error(err, "Error getting ConfigMap", "spec", a.Workload)
		return []utils.NameKind{}, err
	}

	ret := []utils.NameKind{}
	// hack to make sure parsers are created first, they need to exist before any workload that
	// uses them is created
	for k := range cm.Data {
		if !strings.Contains(k, "parse") {
			continue
		}
		nk, err := r.prepareAndRun(bm, 0, k, workloadName, a, cm, []byte(cm.Data[k]))
		if err != nil {
			return ret, err
		}
		if nk != (utils.NameKind{}) {
			ret = append(ret, nk)
		}
	}
	for k := range cm.Data {
		if strings.Contains(k, "parse") {
			continue
		}

		var count int
		if a.Count == 0 {
			count = 1
		} else {
			count = a.Count
		}

		for w := 0; w < count; w++ {
			nk, err := r.prepareAndRun(bm, w, k, workloadName, a, cm, []byte(cm.Data[k]))
			if err != nil {
				return ret, err
			}
			if nk != (utils.NameKind{}) {
				ret = append(ret, nk)
			}
		}
	}
	return ret, nil
}

func (r *ReconcileBenchmark) getParserContainerImage(parserName string) (string, error) {
	parserCm := &corev1.ConfigMap{}

	err := r.client.Get(context.TODO(), client.ObjectKey{Name: parserName, Namespace: "library"}, parserCm)
	if err != nil {
		log.Error(err, "Error getting ConfigMap", "spec", parserName)
		return "", err
	}

	if imageName, exists := parserCm.ObjectMeta.Annotations["container"]; !exists {
		log.Info("Container annotation does not exist for parser", "parser", parserName)
		return "busybox", nil
	} else {
		return imageName, nil
	}
}

func (r *ReconcileBenchmark) createTmpParser(bm *cnsbench.Benchmark, parserName string) (string, error) {
	parserCm := &corev1.ConfigMap{}

	err := r.client.Get(context.TODO(), client.ObjectKey{Name: parserName, Namespace: "library"}, parserCm)
	if err != nil {
		log.Error(err, "Error getting ConfigMap", "spec", parserName)
		return "", err
	}

	newCm := corev1.ConfigMap {
		ObjectMeta: metav1.ObjectMeta {
			Name: names.NameGenerator.GenerateName(names.SimpleNameGenerator, parserName+"-"),
			Namespace: "default",
		},
		Data: make(map[string]string, 0),
	}
	for k, v := range parserCm.Data {
		newCm.Data[k] = v
	}

	if err := r.createObj(bm, runtime.Object(&newCm), true); err != nil {
		log.Error(err, "Creating temp ConfigMap")
	}

	return newCm.ObjectMeta.Name, err
}

func (r *ReconcileBenchmark) addParserContainer(bm *cnsbench.Benchmark, obj runtime.Object, parser string, outfile string, num int) (runtime.Object, error) {
	if parser == "" {
		parser = "null-parser"
	}
	/* We need to do this because parser scripts are in the library namespace,
	 * the workload is in the default namespace, and you can't attach configmaps
	 * across namespaces.  So make a copy of the configmap in the default namespace */
	if tmpCmName, err := r.createTmpParser(bm, parser); err != nil {
		log.Error(err, "Error adding parser container", parser)
		return obj, err
	} else {
		imageName, err := r.getParserContainerImage(parser)
		if err != nil {
			log.Error(err, "Error getting parser container image", parser)
			return obj, err
		}
		log.Info("Creating parser", "cmname", parser, "image", imageName)
		obj, err = utils.AddParserContainer(obj, tmpCmName, outfile, imageName, num)
		if err != nil {
			log.Error(err, "Error adding parser container", outfile)
			return obj, err
		}
		log.Info("Created temp parser", "name", tmpCmName)
	}
	return obj, nil
}

func (r *ReconcileBenchmark) addOutputContainer(bm *cnsbench.Benchmark, obj runtime.Object, outputName string, outputFile string) (runtime.Object, error) {
	outputArgs := ""
	outputContainer := "null-output"
	for _, output := range bm.Spec.Outputs {
		if output.Name == outputName {
			log.Info("Matched an output", "output", output)
			if output.HttpPostSpec.URL != "" {
				outputContainer = "http-output"
				outputArgs = output.HttpPostSpec.URL
			}
		}
	}

	obj, err := utils.AddOutputContainer(obj, outputArgs, outputContainer, outputFile)
	if err != nil {
		log.Error(err, "Error adding output container", outputFile)
		return obj, err
	}
	log.Info("Added output container", "name", outputFile)

	return obj, nil
}

/* TODO: Break this function up */
func (r *ReconcileBenchmark) prepareAndRun(bm *cnsbench.Benchmark, w int, k string, workloadName string, a cnsbench.Workload, cm *corev1.ConfigMap, objBytes []byte) (utils.NameKind, error) {
	var count int
	if a.Count == 0 {
		count = 1
	} else {
		count = a.Count
	}

	// Replace vars in workload spec with values from benchmark object
	cmString := string(objBytes)
	for variable, value := range a.Vars {
		log.Info("Searching for var", "var", variable, "replacement", value)
		cmString = strings.ReplaceAll(cmString, "{{"+variable+"}}", value)
	}
	cmString = strings.ReplaceAll(cmString, "{{ACTION_NAME}}", workloadName)
	cmString = strings.ReplaceAll(cmString, "{{ACTION_NAME_CAPS}}", strings.ToUpper(workloadName))
	cmString = strings.ReplaceAll(cmString, "{{INSTANCE_NUM}}", strconv.Itoa(w))
	cmString = strings.ReplaceAll(cmString, "{{NUM_INSTANCES}}", strconv.Itoa(count))

	// Use workload spec's annotations as default values for any remaining variables
	for k, v := range cm.ObjectMeta.Annotations {
		s := strings.SplitN(k, ".", 3)
		if len(s) == 3 && s[0] == "cnsbench" && s[1] == "default" {
			log.Info("Searching for var", "var", s[2], "default replacement", v)
			cmString = strings.ReplaceAll(cmString, "{{"+s[2]+"}}", v)
		}
	}

	if strings.Contains(cmString, "{{") {
		log.Info("Object definition contains double curly brackets ({{), possible unset variable", "cmstring", cmString)
	}

	// Decode the yaml object from the workload spec
	objBytes = []byte(cmString)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(objBytes, nil, nil)
	if err != nil {
		log.Info("cm", "cm", cmString)
		log.Error(err, "Error decoding yaml")
		return utils.NameKind{}, err
	}

	accessor := meta.NewAccessor()

	// Get the object's kind
	kind, err := accessor.Kind(obj)
	log.Info("KIND", "KIND", kind)

	objAnnotations, err := accessor.Annotations(obj)
	if err != nil {
		log.Error(err, "Error getting object annotations")
		return utils.NameKind{}, err
	}

	var role string
	if _, exists := objAnnotations["role"]; exists {
		role = objAnnotations["role"]
	} else {
		// if no role is set, consider the object a helper (e.g., PVC, ConfigMap)
		role = "helper"
	}

	// If user is specifying output files, use those.  Otherwise, use the object's
	// annotations
	if len(a.OutputFiles) > 0 {
		for i, output := range a.OutputFiles {
			if role == output.Target || (role == "workload" && output.Target == "") {
				// This object needs a parser container added.  The parsers are defined in the
				// "library" namespace, so every time a parser is used make a temporary ConfigMap
				// in the default (TODO: should be same namespace as Benchmark obj, not necessarily
				// default) namespace.  Make it controlled by the Benchmark object so it will be
				// cleaned up when the benchmark exits.
				// XXX: Since the parser might be packaged with the current workload, need to make sure
				// the parser object is seen before any objects referencing it in an Output
				obj, err = r.addParserContainer(bm, obj, output.Parser, output.Filename, i)
				if err != nil {
					log.Error(err, "Error adding parser container")
				}
				if output.Sink != "" {
					obj, err = r.addOutputContainer(bm, obj, output.Sink, output.Filename)
				} else {
					obj, err = r.addOutputContainer(bm, obj, bm.Spec.AllWorkloadOutput, output.Filename)
				}
				if err != nil {
					log.Error(err, "Error adding output container")
				}
			}
		}
	} else if _, exists := objAnnotations["outputFile"]; exists {
		// TODO: Allow more than one default output file
		obj, err = r.addParserContainer(bm, obj, objAnnotations["parser"], objAnnotations["outputFile"], 0)
		if err != nil {
			log.Error(err, "Error adding parser container")
		}
		obj, err = r.addOutputContainer(bm, obj, bm.Spec.AllWorkloadOutput, objAnnotations["outputFile"])
		if err != nil {
			log.Error(err, "Error adding output container")
		}
	}

	// Add INSTANCE_NUM env to init containers, so multiple workload instances can coordinate initialization
	utils.SetEnvVar("INSTANCE_NUM", strconv.Itoa(w), obj)
	utils.SetEnvVar("NUM_INSTANCES", strconv.Itoa(count), obj)

	// Add workloadname and multiinstance labels to object
	labels, err := accessor.Labels(obj)
	if err != nil {
		log.Error(err, "Error getting labels")
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	if a.SyncGroup != "" {
		labels["syncgroup"] = a.SyncGroup
	}
	labels["workloadname"] = workloadName

	var multipleInstanceObjs []string
	if mis, found := cm.ObjectMeta.Annotations["multipleInstances"]; found {
		multipleInstanceObjs = strings.Split(mis, ",")
	}
	if utils.Contains(multipleInstanceObjs, k) {
		labels["multiinstance"] = "true"
	}

	log.Info("labels", "labels", labels)
	accessor.SetLabels(obj, labels)
	obj, err = utils.AddLabelsGeneric(obj, labels)

	// Add sync container if sync start requested
	if _, exists := objAnnotations["sync"]; exists {
		obj, err = utils.AddSyncContainer(obj, a.Count, workloadName, a.SyncGroup)
	}

	// The I/O workload author should use {{INSTANCE_NUM}} as part of an
	// object's name if they want to create multiple instances of that object
	// when count > 0.  Otherwise each object will have the same name, so
	// the create fails (we catch the "AlreadyExists" error and ignore)
	name, err := accessor.Name(obj)
	kind, err = accessor.Kind(obj)

	// Ownership can't transcend namespaces
	makeOwner := true
	if ns, _ := accessor.Namespace(obj); ns != "default" {
		makeOwner = false
	}

	// Make the actual object
	if err := r.createObj(bm, obj, makeOwner); err != nil {
		if !errors.IsAlreadyExists(err) {
			return utils.NameKind{}, err
		} else {
			log.Info("Already exists", "name", name)
			return utils.NameKind{}, nil
		}
	}

	if role == "workload" {
		return utils.NameKind{name, kind}, nil
	}

	return utils.NameKind{}, nil
}

func (r *ReconcileBenchmark) CreateSnapshot(bm *cnsbench.Benchmark, s cnsbench.Snapshot, actionName string) error {
	ls := &metav1.LabelSelector{}

	if s.ActionName != "" {
		ls = metav1.AddLabelToSelector(ls, "workloadname", s.ActionName)
	} else if s.VolumeName != "" {
		ls = metav1.AddLabelToSelector(ls, "volumename", s.VolumeName)
	}
	selector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return err
	}
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := r.client.List(context.TODO(), pvcs, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
		return err
	}

	snapshotscheme.AddToScheme(scheme.Scheme)

	// Takes a snapshot of every volume matching the given selector
	for _, pvc := range pvcs.Items {
		volName := pvc.Name
		name := names.NameGenerator.GenerateName(names.SimpleNameGenerator, bm.ObjectMeta.Name+"-snapshot-")
		snap := snapshotv1beta1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta {
				Name: name,
				Namespace: "default",
				Labels: map[string]string {
					"workloadname": actionName,
				},
			},
			Spec: snapshotv1beta1.VolumeSnapshotSpec {
				VolumeSnapshotClassName: &s.SnapshotClass,
				Source: snapshotv1beta1.VolumeSnapshotSource {
					PersistentVolumeClaimName: &volName,
				},
			},
		}

		if err := r.createObj(bm, runtime.Object(&snap), false); err != nil {
			log.Error(err, "Creating snapshot")
		}
	}

	return nil
}

func (r *ReconcileBenchmark) DeleteObj(bm *cnsbench.Benchmark, d cnsbench.Delete) error {
	// TODO: Generalize to more than just snapshots.  I think we need to get all the api groups,
	// then get all the kinds in those groups, then just iterate through those kinds searching
	// for objects matching the spec
	// See https://godoc.org/k8s.io/client-go/discovery

	log.Info("Delete object")

	labelSelector, err := metav1.LabelSelectorAsSelector(&d.Selector)
	if err != nil {
		return err
	}
	snaps := &snapshotv1beta1.VolumeSnapshotList{}
	if err := r.client.List(context.TODO(), snaps, &client.ListOptions{Namespace: "default", LabelSelector: labelSelector}); err != nil {
		return err
	}
	sort.Slice(snaps.Items, func (i, j int) bool {
                return snaps.Items[i].ObjectMeta.CreationTimestamp.Unix() < snaps.Items[j].ObjectMeta.CreationTimestamp.Unix()
        })
	if len(snaps.Items) > 0 {
		log.Info("Deleting first item", "name", snaps.Items[0].Name, "createtime", snaps.Items[0].ObjectMeta.CreationTimestamp.Unix())
		return r.client.Delete(context.TODO(), &snaps.Items[0])
	}
	log.Info("No objects found")

	return nil
}

func (r *ReconcileBenchmark) ScaleObj(bm *cnsbench.Benchmark, s cnsbench.Scale) error {
	var err error

	name := names.NameGenerator.GenerateName(names.SimpleNameGenerator, "scale-pod-")
	scalePod := &corev1.Pod {
		ObjectMeta: metav1.ObjectMeta {
			Name: name,
			Namespace: "default",
			Labels: map[string]string {
				"app": "scale-pod",
			},
		},
		Spec: corev1.PodSpec {
			RestartPolicy: "Never",
			Containers: []corev1.Container {
				{
					Name: "scale-container",
					Image: "localcontainerrepo:5000/cnsb/kubectl",
					Command: []string{"/scripts/scaleup.sh", s.ObjName},
					VolumeMounts: []corev1.VolumeMount {
						{
							MountPath: "/scripts/",
							Name: "scale-script",
						},
					},
				},
			},
		},
	}
	scaleScriptCM := corev1.ConfigMapVolumeSource {}
	scaleScriptCM.DefaultMode = utilptr.Int32Ptr(0777)
	scaleScriptCM.Name = s.ScriptConfigMap
	scaleScriptVol := corev1.Volume{}
	scaleScriptVol.Name = "scale-script"
	scaleScriptVol.ConfigMap = &scaleScriptCM
	scalePod.Spec.Volumes = append(scalePod.Spec.Volumes, scaleScriptVol)

	if err := controllerutil.SetControllerReference(bm, scalePod, r.scheme); err != nil {
		log.Error(err, "Error making object child of Benchmark", "name", name)
		return err
	}
	if err := controllerutil.SetOwnerReference(bm, scalePod, r.scheme); err != nil {
		log.Error(err, "Error making object child of Benchmark")
		return err
	}

	if err := r.client.Create(context.TODO(), scalePod); err != nil {
		return err
	}

	return err
}

func (r *ReconcileBenchmark) ReconcileInstances(bm *cnsbench.Benchmark, c client.Client, workloads []cnsbench.Workload) ([]utils.NameKind, error) {
	cm := &corev1.ConfigMap{}

	ret := []utils.NameKind{}
	for _, a := range workloads {
		// XXX This should never happen now
		if a.Workload == "" {
			continue
		}
		fmt.Println(a)

		err := r.client.Get(context.TODO(), client.ObjectKey{Name: a.Workload, Namespace: "library"}, cm)
		if err != nil {
			return ret, err
		}

		var multipleInstanceObjs []string
		if mis, found := cm.ObjectMeta.Annotations["multipleInstances"]; found {
			multipleInstanceObjs = strings.Split(mis, ",")
		}

		// Get all pods that match workloadname=true and multiinstance=true, if the number found is
		// less than a.Count, add another set of instances (i.e., instantiate all of the objects
		// in the workload spec listed in the workload's multipleInstances annotation)
		ls := &metav1.LabelSelector{}
		ls = metav1.AddLabelToSelector(ls, "workloadname", a.Name)
		ls = metav1.AddLabelToSelector(ls, "multiinstance", "true")

		selector, err := metav1.LabelSelectorAsSelector(ls)
		if err != nil {
			return ret, err
		}
		pods := &corev1.PodList{}
		if err := c.List(context.TODO(), pods, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
			return ret, err
		}

		incomplete := 0
		for _, pod := range pods.Items {
			fmt.Println(pod.Name, pod.Status.Phase)
			if pod.Status.Phase != "Succeeded" {
				incomplete += 1
			}
		}

		if incomplete < a.Count {
			fmt.Println("Need new instances", a.Count, incomplete)
			for n := 0; n < a.Count - incomplete; n++ {
				if nks, err := r.RunInstance(bm, cm, multipleInstanceObjs, len(pods.Items)+1+n, a); err != nil {
					return ret, err
				} else {
					for _, nk := range nks {
						ret = append(ret, nk)
					}
				}
			}
		}
	}

	return ret, nil
}
