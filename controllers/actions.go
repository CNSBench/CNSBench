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

	objMeta, err := meta.Accessor(obj)
	if err != nil {
		r.Log.Error(err, "Error getting ObjectMeta from obj", name)
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

func (r *BenchmarkReconciler) RunInstance(bm *cnsbench.Benchmark, cm *corev1.ConfigMap, instanceNum int, workload cnsbench.Workload) error {
	/*
		for k := range cm.Data {
			// check if duplicate=true label set on object, if so run it
			continue

				if !utils.Contains(multipleInstanceObjs, k) {
					r.Log.Info("Continuing")
					continue
				}
			//if err := r.prepareAndRun(bm, instanceNum, k, workload.Name, workload, cm, []byte(cm.Data[k])); err != nil {
			//	return err
			//}
		}*/

	return nil
}

func (r *BenchmarkReconciler) RunWorkload(bm *cnsbench.Benchmark, a cnsbench.Workload, workloadName string) error {
	cm := &corev1.ConfigMap{}

	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: a.Workload, Namespace: LIBRARY_NAMESPACE}, cm)
	if err != nil {
		r.Log.Error(err, "Error getting ConfigMap", "spec", a.Workload)
		return err
	}

	// hack to make sure parsers are created first, they need to exist before any workload that
	// uses them is created
	for k := range cm.Data {
		if !strings.Contains(k, "parse") {
			continue
		}
		if err := r.prepareAndRun(bm, 0, k, workloadName, a, cm, []byte(cm.Data[k])); err != nil {
			return err
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
			if err := r.prepareAndRun(bm, w, k, workloadName, a, cm, []byte(cm.Data[k])); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *BenchmarkReconciler) CreateSnapshot(bm *cnsbench.Benchmark, s cnsbench.Snapshot, actionName string) error {
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
	snaps := &snapshotv1beta1.VolumeSnapshotList{}
	if err := r.Client.List(context.TODO(), snaps, &client.ListOptions{Namespace: "default", LabelSelector: labelSelector}); err != nil {
		return err
	}
	sort.Slice(snaps.Items, func(i, j int) bool {
		return snaps.Items[i].ObjectMeta.CreationTimestamp.Unix() < snaps.Items[j].ObjectMeta.CreationTimestamp.Unix()
	})
	if len(snaps.Items) > 0 {
		r.Log.Info("Deleting first item", "name", snaps.Items[0].Name, "createtime", snaps.Items[0].ObjectMeta.CreationTimestamp.Unix())
		return r.Client.Delete(context.TODO(), &snaps.Items[0])
	}
	r.Log.Info("No objects found")

	return nil
}

func (r *BenchmarkReconciler) ScaleObj(bm *cnsbench.Benchmark, s cnsbench.Scale) error {
	var err error

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
			RestartPolicy: "Never",
			Containers: []corev1.Container{
				{
					Name:    "scale-container",
					Image:   "localcontainerrepo:5000/cnsb/kubectl",
					Command: []string{"/scripts/scaleup.sh", s.ObjName},
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
	scaleScriptCM.Name = s.ScriptConfigMap
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

func (r *BenchmarkReconciler) ReconcileInstances(bm *cnsbench.Benchmark, c client.Client, workloads []cnsbench.Workload) error {
	cm := &corev1.ConfigMap{}

	for _, a := range workloads {
		fmt.Println(a)

		if err := r.Client.Get(context.TODO(), client.ObjectKey{Name: a.Workload, Namespace: LIBRARY_NAMESPACE}, cm); err != nil {
			return err
		}

		// Iterate through each object in the workload spec, if it has "duplicate=true" annotation
		// then check how many instances are running
		ls := &metav1.LabelSelector{}
		ls = metav1.AddLabelToSelector(ls, "workloadname", a.Name)
		ls = metav1.AddLabelToSelector(ls, "duplicate", "true")

		selector, err := metav1.LabelSelectorAsSelector(ls)
		if err != nil {
			return err
		}
		pods := &corev1.PodList{}
		if err := c.List(context.TODO(), pods, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
			return err
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
			for n := 0; n < a.Count-incomplete; n++ {
				if err := r.RunInstance(bm, cm, len(pods.Items)+1+n, a); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
