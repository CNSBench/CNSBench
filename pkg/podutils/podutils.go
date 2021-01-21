package podutils

import (
	"context"
	"errors"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func PodSpec(obj client.Object) (*corev1.PodSpec, error) {
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
	fmt.Printf("obj: %v", obj)
	return nil, errors.New("nil kind")
}

func UpdatePodSpec(obj client.Object, spec corev1.PodSpec) (client.Object, error) {
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

func AddLabelsGeneric(obj client.Object, labels map[string]string) (client.Object, error) {
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

func SetEnvVar(name, value string, obj client.Object) (client.Object, error) {
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
