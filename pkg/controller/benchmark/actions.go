package benchmark

import (
	"fmt"
	"os"
	"context"
	"strconv"
	"sort"
	"strings"

	cnsbench "github.com/cnsbench/pkg/apis/cnsbench/v1alpha1"
	"github.com/cnsbench/pkg/utils"

        //appsv1 "k8s.io/api/apps/v1"
        corev1 "k8s.io/api/core/v1"
        batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"k8s.io/client-go/tools/clientcmd"
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

func (r *ReconcileBenchmark) RunInstance(bm *cnsbench.Benchmark, cm *corev1.ConfigMap, multipleInstanceObjs []string, instanceNum int, action cnsbench.Action) ([]utils.NameKind, error) {
	ret := []utils.NameKind{}
	for k := range cm.Data {
		if !utils.Contains(multipleInstanceObjs, k) {
			continue
		}
		nk, err := r.prepareAndRun(bm, instanceNum, k, true, action.Name, action.CreateObjSpec, cm, []byte(cm.Data[k]))
		if err != nil {
			return ret, err
		}
		if nk != (utils.NameKind{}) {
			ret = append(ret, nk)
		}
	}

	return ret, nil
}

func (r *ReconcileBenchmark) RunWorkload(bm *cnsbench.Benchmark, a cnsbench.CreateObj, actionName string) ([]utils.NameKind, error) {
	cm := &corev1.ConfigMap{}

	// XXX If we use r.client.Get the configmap is never found - caching issue?
	// Workaround is to create a new client and use that to do the lookup
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return []utils.NameKind{}, err
	}
	cl, err := client.New(config, client.Options{})
	if err != nil {
		return []utils.NameKind{}, err
	}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: a.Workload, Namespace: "library"}, cm)
	if err != nil {
		log.Error(err, "Error getting ConfigMap", "spec", a.Workload)
		return []utils.NameKind{}, err
	}

	fmt.Println(cm.ObjectMeta.Annotations["multipleInstances"])
	var multipleInstanceObjs []string
	if mis, found := cm.ObjectMeta.Annotations["multipleInstances"]; found {
		multipleInstanceObjs = strings.Split(mis, ",")
	}
	fmt.Println("mis", multipleInstanceObjs, len(multipleInstanceObjs))

	ret := []utils.NameKind{}
	for k := range cm.Data {
		fmt.Println(k, utils.Contains(multipleInstanceObjs, k))
		var count int
		if a.Count == 0 || !utils.Contains(multipleInstanceObjs, k) {
			count = 1
		} else {
			count = a.Count
		}

		for w := 0; w < count; w++ {
			nk, err := r.prepareAndRun(bm, w, k, utils.Contains(multipleInstanceObjs, k), actionName, a, cm, []byte(cm.Data[k]))
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

func (r *ReconcileBenchmark) prepareAndRun(bm *cnsbench.Benchmark, w int, k string, isMultiInstanceObj bool, actionName string, a cnsbench.CreateObj, cm *corev1.ConfigMap, objBytes []byte) (utils.NameKind, error) {
	var count int
	if a.Count == 0 {
		count = 1
	} else {
		count = a.Count
	}
	// Replace vars in workload spec with values from benchmark object
	cmString := string(objBytes)
	for k, v := range a.Vars {
		log.Info("Searching for var", "var", k, "replacement", v)
		// If we're doing > 1 instance and this is an object that's getting repeated,
		cmString = strings.ReplaceAll(cmString, "{{"+k+"}}", v)
	}
	cmString = strings.ReplaceAll(cmString, "{{ACTION_NAME}}", actionName)
	cmString = strings.ReplaceAll(cmString, "{{ACTION_NAME_CAPS}}", strings.ToUpper(actionName))
	cmString = strings.ReplaceAll(cmString, "{{INSTANCE_NUM}}", strconv.Itoa(w))
	cmString = strings.ReplaceAll(cmString, "{{NUM_INSTANCES}}", strconv.Itoa(count))

	// Decode the yaml object from the workload spec
	objBytes = []byte(cmString)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(objBytes, nil, nil)
	if err != nil {
		log.Error(err, "Error decoding yaml")
		return utils.NameKind{}, err
	}

	// Get the object's kind
	kind, err := meta.NewAccessor().Kind(obj)
	log.Info("KIND", "KIND", kind)

	// Add parser container, if the object is the right kind and listed in parseWorklods annotation
	if kind == "Job" || kind == "Pod" || kind == "StatefulSet" {
		if parseWorkloads, exists := cm.ObjectMeta.Annotations["parseWorkloads"]; exists && strings.Contains(parseWorkloads, k) {
			parser, parserExists := cm.ObjectMeta.Annotations["parser"]
			outfile, outfileExists := cm.ObjectMeta.Annotations["outputFile"]
			if parserExists && outfileExists {
				obj, err = utils.AddParserContainerGeneric(obj, parser, outfile)
				if err != nil {
					log.Error(err, "Error adding parser container", cm.ObjectMeta.Annotations)
				}
			}
		}
	}

	// Add INSTANCE_NUM env to init containers, so multiple workload instances can coordinate initialization
	// TODO: Also do this for other kinds of object.  Ideally do it generically for any kind with a PodSpec
	if kind == "Job" {
		for n, _ := range obj.(*batchv1.Job).Spec.Template.Spec.InitContainers {
			obj.(*batchv1.Job).Spec.Template.Spec.InitContainers[n].Env = append(obj.(*batchv1.Job).Spec.Template.Spec.InitContainers[n].Env, corev1.EnvVar{Name: "INSTANCE_NUM", Value: strconv.Itoa(w)})
			obj.(*batchv1.Job).Spec.Template.Spec.InitContainers[n].Env = append(obj.(*batchv1.Job).Spec.Template.Spec.InitContainers[n].Env, corev1.EnvVar{Name: "NUM_INSTANCES", Value: strconv.Itoa(count)})
		}
		for n, _ := range obj.(*batchv1.Job).Spec.Template.Spec.Containers {
			obj.(*batchv1.Job).Spec.Template.Spec.Containers[n].Env = append(obj.(*batchv1.Job).Spec.Template.Spec.Containers[n].Env, corev1.EnvVar{Name: "INSTANCE_NUM", Value: strconv.Itoa(w)})
			obj.(*batchv1.Job).Spec.Template.Spec.Containers[n].Env = append(obj.(*batchv1.Job).Spec.Template.Spec.Containers[n].Env, corev1.EnvVar{Name: "NUM_INSTANCES", Value: strconv.Itoa(count)})
		}
	} else if kind == "Pod" {
		for n, _ := range obj.(*corev1.Pod).Spec.InitContainers {
			obj.(*corev1.Pod).Spec.InitContainers[n].Env = append(obj.(*corev1.Pod).Spec.InitContainers[n].Env, corev1.EnvVar{Name: "INSTANCE_NUM", Value: strconv.Itoa(w)})
			obj.(*corev1.Pod).Spec.InitContainers[n].Env = append(obj.(*corev1.Pod).Spec.InitContainers[n].Env, corev1.EnvVar{Name: "NUM_INSTANCES", Value: strconv.Itoa(count)})
		}
		for n, _ := range obj.(*corev1.Pod).Spec.Containers {
			obj.(*corev1.Pod).Spec.Containers[n].Env = append(obj.(*corev1.Pod).Spec.Containers[n].Env, corev1.EnvVar{Name: "INSTANCE_NUM", Value: strconv.Itoa(w)})
			obj.(*corev1.Pod).Spec.Containers[n].Env = append(obj.(*corev1.Pod).Spec.Containers[n].Env, corev1.EnvVar{Name: "NUM_INSTANCES", Value: strconv.Itoa(count)})
		}
	}

	// Add actionname and multiinstance labels to object
	labels, err := meta.NewAccessor().Labels(obj)
	if err != nil {
		log.Error(err, "Error getting labels")
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	if a.SyncGroup != "" {
		labels["syncgroup"] = a.SyncGroup
	}
	labels["actionname"] = actionName
	labels["multiinstance"] = strconv.FormatBool(isMultiInstanceObj)
	log.Info("labels", "labels", labels)
	meta.NewAccessor().SetLabels(obj, labels)
	obj, err = utils.AddLabelsGeneric(obj, labels)

	// Add sync container if sync start requested
	// TODO: Make this an annotation rather than field in the benchmark spec
	if isMultiInstanceObj {
		obj, err = utils.AddSyncContainerGeneric(obj, a.Count, actionName, a.SyncGroup)
	}

	// The I/O workload author should use {{INSTANCE_NUM}} as part of an
	// object's name if they want to create multiple instances of that object
	// when count > 0.  Otherwise each object will have the same name, so
	// the create fails (we catch the "AlreadyExists" error and ignore)
	name, err := meta.NewAccessor().Name(obj)
	kind, err = meta.NewAccessor().Kind(obj)

	// Ownership can't transcend namespaces
	makeOwner := true
	if ns, _ := meta.NewAccessor().Namespace(obj); ns != "default" {
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

	// TODO: Need a better way of knowing what workload objects to wait for completion
	if parseWorkloads, exists := cm.ObjectMeta.Annotations["parseWorkloads"]; exists && strings.Contains(parseWorkloads, k) {
		return utils.NameKind{name, kind}, nil
	}

	return utils.NameKind{}, nil
}

func (r *ReconcileBenchmark) CreateSnapshot(bm *cnsbench.Benchmark, s cnsbench.Snapshot, actionName string) error {
	ls := &metav1.LabelSelector{}
	ls = metav1.AddLabelToSelector(ls, "actionname", s.ActionName)
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
					"actionname": actionName,
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
					Image: "loadbalancer:5000/cnsb/kubectl",
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

func (r *ReconcileBenchmark) ReconcileInstances(bm *cnsbench.Benchmark, c client.Client, actions []cnsbench.Action) ([]utils.NameKind, error) {
	cm := &corev1.ConfigMap{}
	// XXX If we use r.client.Get the configmap is never found - caching issue
	// because the configmaps are in a different namespace?
	// Workaround is to create a new client and use that to do the lookup
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return []utils.NameKind{}, err
	}
	cl, err := client.New(config, client.Options{})
	if err != nil {
		return []utils.NameKind{}, err
	}

	ret := []utils.NameKind{}
	for _, a := range actions {
		if a.CreateObjSpec.Workload == "" {
			continue
		}
		fmt.Println(a)

		err = cl.Get(context.TODO(), client.ObjectKey{Name: a.CreateObjSpec.Workload, Namespace: "library"}, cm)
		if err != nil {
			return ret, err
		}

		var multipleInstanceObjs []string
		if mis, found := cm.ObjectMeta.Annotations["multipleInstances"]; found {
			multipleInstanceObjs = strings.Split(mis, ",")
		}

		// Get all pods that match actionname=true and multiinstance=true, if the number found is
		// less than a.Count, add another set of instances (i.e., instantiate all of the objects
		// in the workload spec listed in the workload's multipleInstances annotation)
		ls := &metav1.LabelSelector{}
		ls = metav1.AddLabelToSelector(ls, "actionname", a.Name)
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

		if incomplete < a.CreateObjSpec.Count {
			fmt.Println("Need new instances", a.CreateObjSpec.Count, incomplete)
			for n := 0; n < a.CreateObjSpec.Count - incomplete; n++ {
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
