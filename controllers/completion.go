package controllers

import (
	"context"
	"fmt"
	cnsbench "github.com/cnsbench/cnsbench/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

func CheckCompletion(c client.Client, workloadName string) (bool, error) {
	var err error
	var selector labels.Selector
	ls := &metav1.LabelSelector{}
	ls = metav1.AddLabelToSelector(ls, "workloadname", workloadName)
	ls = metav1.AddLabelToSelector(ls, "role", "workload")
	if selector, err = metav1.LabelSelectorAsSelector(ls); err != nil {
		return false, err
	}

	if complete, err := PodsComplete(c, selector); err != nil || !complete {
		return false, err
	}
	if complete, err := JobsComplete(c, selector); err != nil || !complete {
		return false, err
	}
	if complete, err := StatefulSetsComplete(c, selector); err != nil || !complete {
		return false, err
	}
	if complete, err := PVCsComplete(c, selector); err != nil || !complete {
		return false, err
	}

	return true, nil
}

func PodsComplete(c client.Client, selector labels.Selector) (bool, error) {
	pods := &corev1.PodList{}
	if err := c.List(context.TODO(), pods, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
		return false, err
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase != "Succeeded" {
			return false, nil
		}
	}
	return true, nil
}

func StatefulSetsComplete(c client.Client, selector labels.Selector) (bool, error) {
	stss := &appsv1.StatefulSetList{}
	if err := c.List(context.TODO(), stss, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
		return false, err
	}

	for _, sts := range stss.Items {
		labelSelector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
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
	}

	return true, nil
}

func JobsComplete(c client.Client, selector labels.Selector) (bool, error) {
	jobs := &batchv1.JobList{}
	if err := c.List(context.TODO(), jobs, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
		return false, err
	}

	for _, job := range jobs.Items {
		if job.Status.Succeeded < *job.Spec.Completions {
			return false, nil
		}
	}

	return true, nil
	//return obj.Status.Succeeded >= *obj.Spec.Completions, nil
}

func PVCsComplete(c client.Client, selector labels.Selector) (bool, error) {
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := c.List(context.TODO(), pvcs, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
		return false, err
	}

	for _, pvc := range pvcs.Items {
		if pvc.Status.Phase != "Bound" {
			return false, nil
		}
	}
	return true, nil
}
