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

        appsv1 "k8s.io/api/apps/v1"
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
	decode := scheme.Codecs.UniversalDeserializer().Decode
	obj, _, err := decode(objBytes, nil, nil)
	if err != nil {
		log.Error(err, "Error decoding yaml")
		return utils.NameKind{}, err
	}

	kind, err := meta.NewAccessor().Kind(obj)
	log.Info("KIND", "KIND", kind)
	// TODO: Instead of hardcoding config.yaml, use annotation in workload spec
	if kind == "ConfigMap" && k == "config.yaml" && a.Config != "" {
		// User provided their own config, don't create the default one from the workload
		return utils.NameKind{}, nil
	}
	if kind == "PersistentVolumeClaim" {
		obj.(*corev1.PersistentVolumeClaim).Spec.StorageClassName = &a.StorageClass
		if a.VolName == "" {
			obj.(*corev1.PersistentVolumeClaim).ObjectMeta.Name = "test-vol"
		} else {
			obj.(*corev1.PersistentVolumeClaim).ObjectMeta.Name = a.VolName
		}
	} else if kind == "Job" {
		for i, v := range obj.(*batchv1.Job).Spec.Template.Spec.Volumes {
			// TODO: Use annotation instead of hardcoding volume ID
			if v.Name == "data" {
				if a.VolName == "" {
					obj.(*batchv1.Job).Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = "test-vol"
				} else {
					obj.(*batchv1.Job).Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = a.VolName
				}
				// TODO: Instead of hardcoding pvc.yaml, use annotation in workload spec to indicate which yaml file corresponds
				// to the volume used here.  Then check if that is in the multi instance list
				if a.Count > 0 && isMultiInstanceObj {
					obj.(*batchv1.Job).Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName += "-"+strconv.Itoa(w)
				}
			}
		}
		for n, _ := range obj.(*batchv1.Job).Spec.Template.Spec.InitContainers {
			obj.(*batchv1.Job).Spec.Template.Spec.InitContainers[n].Env = append(obj.(*batchv1.Job).Spec.Template.Spec.InitContainers[n].Env, corev1.EnvVar{Name: "INSTANCE_NUM", Value: strconv.Itoa(w)})
		}
		parser, parserExists := cm.ObjectMeta.Annotations["parser"]
		outfile, outfileExists := cm.ObjectMeta.Annotations["outputFile"]
		if parserExists && outfileExists {
			obj, err = utils.AddParserContainerGeneric(obj, parser, outfile)
			if err != nil {
				log.Error(err, "Error adding parser container", cm.ObjectMeta.Annotations)
			}
		}
	} else if kind == "Pod" {
		for i, v := range obj.(*corev1.Pod).Spec.Volumes {
			// TODO: Use annotation instead of hardcoding volume ID
			if v.Name == "data" {
				if a.VolName == "" {
					obj.(*corev1.Pod).Spec.Volumes[i].PersistentVolumeClaim.ClaimName = "test-vol"
				} else {
					obj.(*corev1.Pod).Spec.Volumes[i].PersistentVolumeClaim.ClaimName = a.VolName
				}
				// TODO: Instead of hardcoding pvc.yaml, use annotation in workload spec to indicate which yaml file corresponds
				// to the volume used here.  Then check if that is in the multi instance list
				if a.Count > 0 && isMultiInstanceObj {
					obj.(*corev1.Pod).Spec.Volumes[i].PersistentVolumeClaim.ClaimName += "-"+strconv.Itoa(w)
				}
			}
		}
		parser, parserExists := cm.ObjectMeta.Annotations["parser"]
		outfile, outfileExists := cm.ObjectMeta.Annotations["outputFile"]
		if parserExists && outfileExists {
			obj, err = utils.AddParserContainerGeneric(obj, parser, outfile)
			if err != nil {
				log.Error(err, "Error adding parser container", cm.ObjectMeta.Annotations)
			}
		}
	} else if kind == "StatefulSet" {
		for i, v := range obj.(*appsv1.StatefulSet).Spec.VolumeClaimTemplates {
			if v.Name == "data" {
				obj.(*appsv1.StatefulSet).Spec.VolumeClaimTemplates[i].Spec.StorageClassName = &a.StorageClass
			}
		}
		obj, err = utils.AddParserContainerGeneric(obj, cm.ObjectMeta.Annotations["parser"], cm.ObjectMeta.Annotations["outputFile"])
		if err != nil {
			log.Error(err, "Error adding parser container", cm.ObjectMeta.Annotations)
		}
	}

	// Add actionname label to object
	labels, err := meta.NewAccessor().Labels(obj)
	if err != nil {
		log.Error(err, "Error getting labels")
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["actionname"] = actionName
	labels["multiinstance"] = strconv.FormatBool(isMultiInstanceObj)
	log.Info("labels", "labels", labels)
	meta.NewAccessor().SetLabels(obj, labels)
	obj, err = utils.AddLabelsGeneric(obj, labels)

	// Add use config if given
	if a.Config != "" {
		obj, err = utils.UseUserConfig(obj, a.Config)
	}

	// Add sync container if sync start requested
	// TODO: Make this an annotation rather than field in the benchmark spec
	if isMultiInstanceObj {
		obj, err = utils.AddSyncContainerGeneric(obj, a.Count, actionName)
	}

	name, err := meta.NewAccessor().Name(obj)
	kind, err = meta.NewAccessor().Kind(obj)
	if a.Count > 0 && isMultiInstanceObj {
		name += "-"+strconv.Itoa(w)
		meta.NewAccessor().SetName(obj, name)
	}
	if err := r.createObj(bm, obj, true); err != nil {
		if !errors.IsAlreadyExists(err) {
			return utils.NameKind{}, err
		} else {
			log.Info("Already exists", "name", name)
			return utils.NameKind{}, nil
		}
	}
	// TODO: Fix this hack
	if k == "workload.yaml" {
		return utils.NameKind{name, kind}, nil
	}

	return utils.NameKind{}, nil
}

func (r *ReconcileBenchmark) CreateSnapshot(bm *cnsbench.Benchmark, s cnsbench.Snapshot, actionName string) error {
	/*
	cm := &corev1.ConfigMap{}
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return err
	}
	cl, err := client.New(config, client.Options{})
	if err != nil {
		return err
	}
	err = cl.Get(context.TODO(), client.ObjectKey{Name: "base-snapshot", Namespace: "library"}, cm)
	if err != nil {
		log.Error(err, "Error getting ConfigMap", "spec", "base-snapshot")
		return err
	}


	decode := scheme.Codecs.UniversalDeserializer().Decode
	objBytes := []byte(cm.Data["base-snapshot.yaml"])
	obj, _, err := decode(objBytes, nil, nil)
	if err != nil {
		log.Error(err, "Error decoding yaml")
		return err
	}

	obj.(*snapshotv1beta1.VolumeSnapshot).Spec.VolumeSnapshotClassName = &s.SnapshotClass
	obj.(*snapshotv1beta1.VolumeSnapshot).Spec.Source.PersistentVolumeClaimName = &s.VolName

	meta.NewAccessor().SetName(obj, name)
	return r.createObj(bm, obj, false)
	*/

	snapshotscheme.AddToScheme(scheme.Scheme)

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
				PersistentVolumeClaimName: &s.VolName,
			},
		},
	}

	return r.createObj(bm, runtime.Object(&snap), false)
}

func (r *ReconcileBenchmark) DeleteObj(bm *cnsbench.Benchmark, d cnsbench.Delete) error {
	// TODO: Generalize to more than just PVCs and snapshots.  I think we need to get all the api groups,
	// then get all the kinds in those groups, then just iterate through those kinds searching
	// for objects matching the spec
	// See https://godoc.org/k8s.io/client-go/discovery

	log.Info("Delete object")

	labelSelector, err := metav1.LabelSelectorAsSelector(&d.Selector)
	if err != nil {
		return err
	}
	//pvcs := &corev1.PersistentVolumeClaimList{}
	pvcs := &snapshotv1beta1.VolumeSnapshotList{}
	if err := r.client.List(context.TODO(), pvcs, &client.ListOptions{Namespace: "default", LabelSelector: labelSelector}); err != nil {
		return err
	}
	sort.Slice(pvcs.Items, func (i, j int) bool {
                return pvcs.Items[i].ObjectMeta.CreationTimestamp.Unix() < pvcs.Items[j].ObjectMeta.CreationTimestamp.Unix()
        })
/*
	for _, p := range pvcs.Items {
		log.Info("asd", "s", p.ObjectMeta.CreationTimestamp.Unix(), "n", p.Name)
	}
*/
	if len(pvcs.Items) > 0 {
		log.Info("Deleting first item", "name", pvcs.Items[0].Name, "createtime", pvcs.Items[0].ObjectMeta.CreationTimestamp.Unix())
		return r.client.Delete(context.TODO(), &pvcs.Items[0])
	}
	log.Info("No objects found")

	return nil
}

func (r *ReconcileBenchmark) ScaleObj(bm *cnsbench.Benchmark, s cnsbench.Scale) error {
	var newSize int32
	var err error
	switch s.ObjKind {
	case "Job":
		job := &batchv1.Job{}
		if err := r.client.Get(context.TODO(), client.ObjectKey{Name: s.ObjName, Namespace: "default"}, job); err != nil {
			return err
		}
		newSize = *job.Spec.Completions + 1
		job.Spec.Completions = &newSize
		err = r.client.Update(context.TODO(), job)
	case "StatefulSet":
		sts := &appsv1.StatefulSet{}
		if err := r.client.Get(context.TODO(), client.ObjectKey{Name: s.ObjName, Namespace: "default"}, sts); err != nil {
			return err
		}
		newSize = *sts.Spec.Replicas + 1
		sts.Spec.Replicas = &newSize
		err = r.client.Update(context.TODO(), sts)
	case "Deployment":
		dep := &appsv1.Deployment{}
		if err := r.client.Get(context.TODO(), client.ObjectKey{Name: s.ObjName, Namespace: "default"}, dep); err != nil {
			return err
		}
		newSize = *dep.Spec.Replicas + 1
		dep.Spec.Replicas = &newSize
		err = r.client.Update(context.TODO(), dep)
	case "ReplicaSet":
		rs := &appsv1.ReplicaSet{}
		if err := r.client.Get(context.TODO(), client.ObjectKey{Name: s.ObjName, Namespace: "default"}, rs); err != nil {
			return err
		}
		newSize = *rs.Spec.Replicas + 1
		rs.Spec.Replicas = &newSize
		err = r.client.Update(context.TODO(), rs)
	}

	return err

}

func (r *ReconcileBenchmark) ReconcileInstances(bm *cnsbench.Benchmark, c client.Client, actions []cnsbench.Action) ([]utils.NameKind, error) {
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
