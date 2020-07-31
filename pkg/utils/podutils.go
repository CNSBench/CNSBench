package utils

import (
	"fmt"
	"os"
	"context"
	"bytes"
        "k8s.io/apimachinery/pkg/runtime"
        "k8s.io/apimachinery/pkg/api/meta"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	utilptr "k8s.io/utils/pointer"
)

type NameKind struct {
	Name string
	Kind string
}

func CheckCompletion(c client.Client, objs []NameKind) (bool, error) {
	for _, o := range objs {
		if o.Kind == "Job" {
			if complete, err := JobComplete(c, o.Name); err != nil || !complete {
				return complete, err
			}
		} else if o.Kind == "PersistentVolumeClaim" {
			if complete, err := PVCComplete(c, o.Name); err != nil || !complete {
				return complete, err
			}
		} else if o.Kind == "StatefulSet" {
			if complete, err := StatefulSetComplete(c, o.Name); err != nil || !complete {
				return complete, err
			}
		}
	}
	return true, nil
}

func StatefulSetComplete(c client.Client, name string) (bool, error) {
	//updated := false

	obj := &appsv1.StatefulSet{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: "default"}, obj); err != nil {
		return false, err
	}
	/*
	annotations := obj.ObjectMeta.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}*/

	labelSelector, err := metav1.LabelSelectorAsSelector(obj.Spec.Selector)
	if err != nil {
		return false, err
	}
	pods := &corev1.PodList{}
	if err := c.List(context.TODO(), pods, &client.ListOptions{Namespace: "default", LabelSelector: labelSelector}); err != nil {
		return false, err
	} else {
		for _, pod := range pods.Items {
			for _, c := range pod.Status.ContainerStatuses {
				if c.RestartCount == 0 {
					return false, nil
				}
			}
			/*
			numContainers := len(pod.Status.ContainerStatuses)
			if numContainers > 0 && pod.Status.ContainerStatuses[numContainers-1].State.Terminated != nil {
				// container is done, add to annotations
				if _, exists := annotations[pod.Name]; !exists {
					annotations[pod.Name] = "complete"
					updated = true
				}
			}*/
		}
	}

	/*
	fmt.Println(annotations, len(annotations), updated, int(*obj.Spec.Replicas))
	if updated {
		obj.ObjectMeta.Annotations = annotations
		if err := c.Update(context.TODO(), obj); err != nil {
			return false, err
		}
	}

	return len(annotations) >= int(*obj.Spec.Replicas), nil
	*/
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

	//return false, nil
}

func ReadContainerLog(pod string, container string) (string, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return "", err
	}

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	req := cs.CoreV1().Pods("default").GetLogs(pod, &corev1.PodLogOptions{Container: container},)
	readCloser, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}
	buf.ReadFrom(readCloser)
	fmt.Println("Output length:", buf.Len())
	if buf.Len() == 0 {
		req = cs.CoreV1().Pods("default").GetLogs(pod, &corev1.PodLogOptions{Container: container, Previous: true},)
		readCloser, err = req.Stream(context.TODO())
		if err != nil {
			return "", err
		}
		buf.ReadFrom(readCloser)
		fmt.Println("Output length:", buf.Len())
	}

	return buf.String(), nil
}

func AddParserContainerGeneric(obj runtime.Object, parserCMName string, logFilename string) (runtime.Object, error) {
	kind, err := meta.NewAccessor().Kind(obj)
	if err != nil {
		return nil, err
	}
	if kind == "Job" {
		pt := *obj.(*batchv1.Job)
		addParserContainer(&pt.Spec.Template.Spec, parserCMName, logFilename)
		return runtime.Object(&pt), nil
	} else if kind == "StatefulSet" {
		pt := *obj.(*appsv1.StatefulSet)
		addParserContainer(&pt.Spec.Template.Spec, parserCMName, logFilename)
		return runtime.Object(&pt), nil
	}
	return nil, nil
}

func addParserContainer(spec *corev1.PodSpec, parserCMName string, logFilename string) {
	spec.ShareProcessNamespace = utilptr.BoolPtr(true)

	c := corev1.Container{}
	c.Name = "parser-container"
	c.Image = "python:3.8.0"
	c.Command = []string{"/collector/parse-logs.sh", logFilename}
	//c.Command = []string{"tail", "-f", "/dev/null"}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/parser/",
			Name: "parser",
		},
		{
			MountPath: "/collector/",
			Name: "collector",
		},
	}
	spec.Containers = append(spec.Containers, c)

	parserVol := corev1.Volume{}
	parserVol.Name = "parser"
	parserCmvs := corev1.ConfigMapVolumeSource {}
	parserCmvs.DefaultMode = utilptr.Int32Ptr(0777)
	parserCmvs.Name = parserCMName
	parserVol.ConfigMap = &parserCmvs
	spec.Volumes = append(spec.Volumes, parserVol)

	collectorVol := corev1.Volume{}
	collectorVol.Name = "collector"
	collectorCmvs := corev1.ConfigMapVolumeSource {}
	collectorCmvs.DefaultMode = utilptr.Int32Ptr(0777)
	collectorCmvs.Name = "parse-logs"
	collectorVol.ConfigMap = &collectorCmvs
	spec.Volumes = append(spec.Volumes, collectorVol)
}
