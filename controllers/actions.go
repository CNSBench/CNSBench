package controllers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	cnsbench "github.com/cnsbench/cnsbench/api/v1alpha1"

	snapshotv1beta1 "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	snapshotscheme "github.com/kubernetes-csi/external-snapshotter/v2/pkg/client/clientset/versioned/scheme"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes/scheme"
	utilptr "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func (r *BenchmarkReconciler) createObj(bm *cnsbench.Benchmark, obj client.Object, makeOwner bool) error {
	name, err := meta.NewAccessor().Name(obj)
	kind, err := meta.NewAccessor().Kind(obj)

	objMeta, err := meta.Accessor(obj)
	if err != nil {
		r.Log.Error(err, "Error getting ObjectMeta from obj", "name", name)
		return err
	}

	if makeOwner {
		if err := controllerutil.SetControllerReference(bm, objMeta, r.Scheme); err != nil {
			r.Log.Error(err, "Error making object child of Benchmark", "name", name)
			return err
		}
		if err := controllerutil.SetOwnerReference(bm, objMeta, r.Scheme); err != nil {
			r.Log.Error(err, "Error making object child of Benchmark")
			return err
		}
	}

	for _, x := range scheme.Codecs.SupportedMediaTypes() {
		if x.MediaType == "application/yaml" {
			ptBytes, err := runtime.Encode(x.Serializer, obj)
			if err != nil {
				r.Log.Error(err, "Error encoding spec")
			}
			fmt.Println(string(ptBytes))
		}
	}

	if err := r.Client.Create(context.TODO(), obj); err != nil {
		if errors.IsAlreadyExists(err) {
			r.Log.Info("Object already exists, proceeding", "name", name)
		} else {
			return err
		}
	}

	r.metric(bm, "createObj", "name", name, "kind", kind)

	err = r.controller.Watch(&source.Kind{Type: obj}, &handler.EnqueueRequestForOwner{
		IsController: false,
		OwnerType:    &cnsbench.Benchmark{},
	})
	if err != nil {
		r.Log.Error(err, "Watching")
		return err
	}

	return nil
}

func (r *BenchmarkReconciler) CreateVolume(bm *cnsbench.Benchmark, vol cnsbench.Volume) {
	// XXX This might be called because a rate fired, in which case there might
	// already be a volume - need to check what the last volume number is and
	// count from <last vol> to count+<last vol>
	for c := 0; c < vol.Count; c++ {
		name := vol.Name
		if vol.Count > 1 {
			name += "-" + strconv.Itoa(c)
		}
		pvc := corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels: map[string]string{
					"volumename": vol.Name,
				},
			},
			Spec: vol.Spec,
		}

		if err := r.createObj(bm, client.Object(&pvc), true); err != nil {
			r.Log.Error(err, "Creating volume")
		}
	}
}

func (r *BenchmarkReconciler) RunWorkload(bm *cnsbench.Benchmark, a cnsbench.Workload, workloadName string) error {
	cm := &corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: a.Workload, Namespace: LIBRARY_NAMESPACE}, cm)
	if err != nil {
		r.Log.Error(err, "Error getting ConfigMap", a.Workload)
		return err
	}

	// hack to make sure parsers are created first, they need to exist before any workload that
	// uses them is created
	for k := range cm.Data {
		if !strings.Contains(k, "parse") {
			continue
		}
		if err := r.prepareAndRun(bm, 0, workloadName, a, cm, []byte(cm.Data[k])); err != nil {
			return err
		}
	}
	for k := range cm.Data {
		if strings.Contains(k, "parse") {
			continue
		}

		for w := 0; w < a.Count; w++ {
			if err := r.prepareAndRun(bm, w, workloadName, a, cm, []byte(cm.Data[k])); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *BenchmarkReconciler) CreateSnapshot(bm *cnsbench.Benchmark, s cnsbench.Snapshot, actionName string) error {
	ls := &metav1.LabelSelector{}

	if s.WorkloadName != "" {
		ls = metav1.AddLabelToSelector(ls, "workloadname", s.WorkloadName)
	} else if s.VolumeName != "" {
		ls = metav1.AddLabelToSelector(ls, "volumename", s.VolumeName)
	}
	selector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return err
	}
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(context.TODO(), pvcs, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
		return err
	}

	snapshotscheme.AddToScheme(scheme.Scheme)

	// Takes a snapshot of every volume matching the given selector
	for _, pvc := range pvcs.Items {
		volName := pvc.Name
		name := names.NameGenerator.GenerateName(names.SimpleNameGenerator, bm.ObjectMeta.Name+"-snapshot-")
		snap := snapshotv1beta1.VolumeSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "default",
				Labels: map[string]string{
					"workloadname": actionName,
				},
			},
			Spec: snapshotv1beta1.VolumeSnapshotSpec{
				VolumeSnapshotClassName: &s.SnapshotClass,
				Source: snapshotv1beta1.VolumeSnapshotSource{
					PersistentVolumeClaimName: &volName,
				},
			},
		}

		if err := r.createObj(bm, client.Object(&snap), false); err != nil {
			r.Log.Error(err, "Creating snapshot")
		}
	}

	return nil
}

func (r *BenchmarkReconciler) DeleteObj(bm *cnsbench.Benchmark, d cnsbench.Delete) error {
	// TODO: Generalize to more than just snapshots.  I think we need to get all the api groups,
	// then get all the kinds in those groups, then just iterate through those kinds searching
	// for objects matching the spec
	// See https://godoc.org/k8s.io/client-go/discovery

	r.Log.Info("Delete object")

	labelSelector, err := metav1.LabelSelectorAsSelector(&d.Selector)
	if err != nil {
		return err
	}
	objList := &unstructured.UnstructuredList{}
	objList.SetAPIVersion(d.APIVersion)
	objList.SetKind(d.Kind)
	if err := r.Client.List(context.TODO(), objList, &client.ListOptions{Namespace: "default", LabelSelector: labelSelector}); err != nil {
		return err
	}
	sort.Slice(objList.Items, func(i, j int) bool {
		return objList.Items[i].GetCreationTimestamp().Unix() < objList.Items[j].GetCreationTimestamp().Unix()
	})
	if len(objList.Items) > 0 {
		r.Log.Info("Deleting first item", "name", objList.Items[0].GetName(), "createtime", objList.Items[0].GetCreationTimestamp().Unix())
		return r.Client.Delete(context.TODO(), &objList.Items[0])
	}
	r.Log.Info("No objects found")

	return nil
}

func (r *BenchmarkReconciler) ScaleObj(bm *cnsbench.Benchmark, s cnsbench.Scale, numReplicas int) error {
	// For now, the way this works is: a configmap in the library namespace
	// contains scripts for scaling up/down an object.  In a Scale control
	// op spec, the user specifies the name of the object they want to
	// scale and the name of this configmap in the library.  We clone the
	// configmap in to the default namespace, create a pod that attaches
	// that configmap, and run the scale script in that pod.
	//
	// Once we are able to include more complicated objects (i.e.,
	// non-default resource types) in a workload, then we will add a field
	// to the Scale spec that references a Workload in the Benchmark (like
	// the WorkloadName field of a Snapshot control op spec).  That will
	// let us lookup the target object (so the user doesn't need to specify
	// the actual name of the object to be scaled), and the workload spec
	// should include the scale scripts so we can look those up as well.
	//
	// Summary: Currently, a user must supply both the name of the object
	// to be scaled and the name of a configmap in the library namespace
	// that has scripts for scaling that object.  In the future, they will
	// be able to instead just supply the name of a workload that CNSBench
	// has already instantiated, and CNSBench will be able to lookup both
	// the scale scripts and the target object from that alone.

	// TODO: Check to see if a copy of the scale script configmap already
	// exists, use that if so.
	scriptsCMName, err := r.cloneParser(bm, s.ScaleScripts)
	if err != nil {
		return err
	}

	name := names.NameGenerator.GenerateName(names.SimpleNameGenerator, "scale-pod-")
	scalePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels: map[string]string{
				"app": "scale-pod",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:      "Never",
			ServiceAccountName: s.ServiceAccountName,
			Containers: []corev1.Container{
				{
					Name:    "scale-container",
					Image:   "cnsbench/kubectl-container",
					Command: []string{"/scripts/scale.sh", s.ObjName, strconv.Itoa(numReplicas)},
					VolumeMounts: []corev1.VolumeMount{
						{
							MountPath: "/scripts/",
							Name:      "scale-script",
						},
					},
				},
			},
		},
	}
	scaleScriptCM := corev1.ConfigMapVolumeSource{}
	scaleScriptCM.DefaultMode = utilptr.Int32Ptr(0777)
	scaleScriptCM.Name = scriptsCMName
	scaleScriptVol := corev1.Volume{}
	scaleScriptVol.Name = "scale-script"
	scaleScriptVol.ConfigMap = &scaleScriptCM
	scalePod.Spec.Volumes = append(scalePod.Spec.Volumes, scaleScriptVol)

	if err := controllerutil.SetControllerReference(bm, scalePod, r.Scheme); err != nil {
		r.Log.Error(err, "Error making object child of Benchmark", "name", name)
		return err
	}
	if err := controllerutil.SetOwnerReference(bm, scalePod, r.Scheme); err != nil {
		r.Log.Error(err, "Error making object child of Benchmark")
		return err
	}

	if err := r.Client.Create(context.TODO(), scalePod); err != nil {
		return err
	}

	return err
}

func (r *BenchmarkReconciler) ReconcileInstances(bm *cnsbench.Benchmark, workloads []cnsbench.Workload) error {
	var err error
	cm := &corev1.ConfigMap{}
	accessor := meta.NewAccessor()

	for _, a := range workloads {
		fmt.Println(a)

		// Check how many workloads are complete and how many exist (running or otherwise) but aren't complete
		workloadsNeeded := 0
		var workloadsComplete, workloadsNotComplete int
		if workloadsComplete, workloadsNotComplete, err = CountCompletions(r.Client, a.Name); err != nil {
			return err
		} else {
			workloadsNeeded = a.Count - workloadsNotComplete
		}
		if workloadsNeeded <= 0 {
			continue
		}
		r.Log.Info("ReconcileInstances", "Workloads needed", workloadsNeeded, "complete", workloadsComplete, "not complete", workloadsNotComplete)

		// Fewer non-complete workloads exist than "count", so need to create more workload instances
		if err := r.Client.Get(context.TODO(), client.ObjectKey{Name: a.Workload, Namespace: LIBRARY_NAMESPACE}, cm); err != nil {
			return err
		}

		for k := range cm.Data {
			// We need to decode the configmap to get the workload object's annotations
			// But we won't actually use the decoded yaml to instantiate anything, so
			// just use "0" for the workload number
			cmString := r.replaceVars(cm.Data[k], a, 0, a.Count, a.Name, cm)
			if obj, err := r.decodeConfigMap(cmString); err != nil {
				return err
			} else {
				// Only create a new instance of this workload if it's "duplicate" annotation is "true"
				if objAnnotations, err := accessor.Annotations(obj); err != nil {
					return err
				} else if duplicate, exists := objAnnotations["duplicate"]; !exists || duplicate != "true" {
					continue
				}
			}

			for w := workloadsComplete + workloadsNotComplete; w < workloadsComplete+a.Count; w++ {
				if err := r.prepareAndRun(bm, w, a.Name, a, cm, []byte(cm.Data[k])); err != nil {
					return err
				}
			}
		}

	}

	return nil
}
