package benchmark

import (
	"fmt"
	"os"
	"context"

	cnsbench "github.com/cnsbench/pkg/apis/cnsbench/v1alpha1"
	"github.com/cnsbench/pkg/utils"

        appsv1 "k8s.io/api/apps/v1"
        corev1 "k8s.io/api/core/v1"
        batchv1 "k8s.io/api/batch/v1"
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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func (r *ReconcileBenchmark) RunWorkload(bm *cnsbench.Benchmark, a cnsbench.CreateObj) ([]utils.NameKind, error) {
	cm := &corev1.ConfigMap{}
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

	decode := scheme.Codecs.UniversalDeserializer().Decode

	ret := []utils.NameKind{}
	for k := range cm.Data {
		objBytes := []byte(cm.Data[k])
		obj, _, err := decode(objBytes, nil, nil)
		if err != nil {
			log.Error(err, "Error decoding yaml")
			return ret, err
		}

		kind, err := meta.NewAccessor().Kind(obj)
		log.Info("KIND", "KIND", kind)
		if kind == "ConfigMap" && k == "config.yaml" && a.Config != ""{
			// User provided their own config, don't create the default one from the workload
			continue
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
				if v.Name == "data" {
					if a.VolName == "" {
						obj.(*batchv1.Job).Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = "test-vol"
					} else {
						obj.(*batchv1.Job).Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = a.VolName
					}
				}
			}
			obj, err = utils.AddParserContainerGeneric(obj, cm.ObjectMeta.Annotations["parser"], cm.ObjectMeta.Annotations["outputFile"])
			if err != nil {
				log.Error(err, "Error adding parser container", cm.ObjectMeta.Annotations)
			}
		} else if kind == "StatefulSet" {
			for i, v := range obj.(*appsv1.StatefulSet).Spec.VolumeClaimTemplates {
				log.Info("uh", "x", v)
				if v.Name == "data" {
					obj.(*appsv1.StatefulSet).Spec.VolumeClaimTemplates[i].Spec.StorageClassName = &a.StorageClass
				}
			}
			obj, err = utils.AddParserContainerGeneric(obj, cm.ObjectMeta.Annotations["parser"], cm.ObjectMeta.Annotations["outputFile"])
			if err != nil {
				log.Error(err, "Error adding parser container", cm.ObjectMeta.Annotations)
			}
		}

		if a.Config != "" {
			obj, err = utils.UseUserConfig(obj, a.Config)
		}

		name, err := meta.NewAccessor().Name(obj)
		kind, err = meta.NewAccessor().Kind(obj)
		if err := r.createObj(bm, obj, true); err != nil {
			if !errors.IsAlreadyExists(err) {
				return ret, err
			} else {
				log.Info("Already exists", "name", name)
				continue
			}
		}
		ret = append(ret, utils.NameKind{name, kind})
	}
	return ret, nil
}

func (r *ReconcileBenchmark) CreateSnapshot(bm *cnsbench.Benchmark, s cnsbench.Snapshot) error {
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

	snapshotscheme.AddToScheme(scheme.Scheme)

	decode := scheme.Codecs.UniversalDeserializer().Decode
	objBytes := []byte(cm.Data["base-snapshot.yaml"])
	obj, _, err := decode(objBytes, nil, nil)
	if err != nil {
		log.Error(err, "Error decoding yaml")
		return err
	}

	obj.(*snapshotv1beta1.VolumeSnapshot).Spec.VolumeSnapshotClassName = &s.SnapshotClass
	obj.(*snapshotv1beta1.VolumeSnapshot).Spec.Source.PersistentVolumeClaimName = &s.VolName

	name := names.NameGenerator.GenerateName(names.SimpleNameGenerator, bm.ObjectMeta.Name+"-snapshot-")
	meta.NewAccessor().SetName(obj, name)

	return r.createObj(bm, obj, false)
}

func (r *ReconcileBenchmark) DeleteObj(bm *cnsbench.Benchmark, d cnsbench.Delete) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "",
		Kind: d.ObjKind,
		Version: "v1",
	})
	if err := r.client.Get(context.TODO(), client.ObjectKey{Name: d.ObjName, Namespace: "default"}, obj); err != nil {
		return err
	}
	return r.client.Delete(context.TODO(), obj)
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
