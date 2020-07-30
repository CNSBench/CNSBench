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
		if kind == "PersistentVolumeClaim" {
			obj.(*corev1.PersistentVolumeClaim).Spec.StorageClassName = &a.StorageClass
			obj.(*corev1.PersistentVolumeClaim).ObjectMeta.Name = a.VolName
		} else if kind == "Job" {
			for i, v := range obj.(*batchv1.Job).Spec.Template.Spec.Volumes {
				if v.Name == "data" {
					obj.(*batchv1.Job).Spec.Template.Spec.Volumes[i].PersistentVolumeClaim.ClaimName = a.VolName
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
