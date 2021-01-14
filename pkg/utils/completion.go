package utils

import (
	"context"
	"fmt"
	cnsbench "github.com/cnsbench/cnsbench/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type NameKind struct {
	Name string
	Kind string
}

func CheckInit(c client.Client, workloads []cnsbench.Workload) (bool, error) {
	for _, a := range workloads {
		labelSelector, err := metav1.ParseToLabelSelector("workloadname=" + a.Name)
		if err != nil {
			return false, err
		}
		selector, err := metav1.LabelSelectorAsSelector(labelSelector)
		if err != nil {
			return false, err
		}
		pods := &corev1.PodList{}
		if err := c.List(context.TODO(), pods, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
			return false, err
		}
		for _, pod := range pods.Items {
			fmt.Println(pod.Status.Phase)
			if pod.Status.Phase != "Running" && pod.Status.Phase != "Succeeded" {
				return false, nil
			}
		}
	}
	return true, nil
}

func CheckCompletion(c client.Client, objs []NameKind) (bool, error) {
	for _, o := range objs {
		fmt.Println(o.Name, o.Kind)
		if o.Kind == "Job" {
			if complete, err := JobComplete(c, o.Name); err != nil || !complete {
				fmt.Println("Not complete", o.Name)
				return complete, err
			}
		} else if o.Kind == "Pod" {
			if complete, err := PodComplete(c, o.Name); err != nil || !complete {
				fmt.Println("Not complete", o.Name)
				return complete, err
			}
		} else if o.Kind == "PersistentVolumeClaim" {
			if complete, err := PVCComplete(c, o.Name); err != nil || !complete {
				fmt.Println("Not complete", o.Name)
				return complete, err
			}
		} else if o.Kind == "StatefulSet" {
			if complete, err := StatefulSetComplete(c, o.Name); err != nil || !complete {
				fmt.Println("Not complete", o.Name)
				return complete, err
			}
		}
	}
	return true, nil
}

func PodComplete(c client.Client, name string) (bool, error) {
	pod := &corev1.Pod{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: "default"}, pod); err != nil {
		return false, err
	}
	return pod.Status.Phase == "Succeeded", nil
}

func StatefulSetComplete(c client.Client, name string) (bool, error) {
	obj := &appsv1.StatefulSet{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: "default"}, obj); err != nil {
		return false, err
	}

	labelSelector, err := metav1.LabelSelectorAsSelector(obj.Spec.Selector)
	if err != nil {
		return false, err
	}
	pods := &corev1.PodList{}
	if err := c.List(context.TODO(), pods, &client.ListOptions{Namespace: "default", LabelSelector: labelSelector}); err != nil {
		return false, err
	} else {
		if len(pods.Items) == 0 {
			return false, nil
		}
		for _, pod := range pods.Items {
			if len(pod.Status.ContainerStatuses) == 0 {
				return false, nil
			}
			fmt.Println(len(pod.Status.ContainerStatuses))
			for _, c := range pod.Status.ContainerStatuses {
				if c.RestartCount == 0 {
					return false, nil
				}
			}
		}
	}

	return true, nil
}

func JobComplete(c client.Client, name string) (bool, error) {
	obj := &batchv1.Job{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: "default"}, obj); err != nil {
		return false, err
	}

	return obj.Status.Succeeded >= *obj.Spec.Completions, nil
}

func PVCComplete(c client.Client, name string) (bool, error) {
	obj := &corev1.PersistentVolumeClaim{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: "default"}, obj); err != nil {
		return false, err
	}
	return obj.Status.Phase == "Bound", nil
}
