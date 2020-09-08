package utils

import (
	"fmt"
	"os"
	"strconv"
	"context"
	"bytes"
	"strings"
	"bufio"
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
        cnsbench "github.com/cnsbench/pkg/apis/cnsbench/v1alpha1"
)

type NameKind struct {
	Name string
	Kind string
}

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
			if err := c.Delete(context.TODO(), &pod); err != nil{
				fmt.Println("Error deleting scaling pod", err)
			}
		}
	}

	return nil
}

func CheckInit(c client.Client, actions []cnsbench.Action) (bool, error) {
	for _, a := range actions {
		labelSelector, err := metav1.ParseToLabelSelector("actionname="+a.Name)
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

func GetLastLine(s string) (string, error) {
	var lastLine string
	scanner := bufio.NewScanner(strings.NewReader(s))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024*10)
	for scanner.Scan() {
		lastLine = scanner.Text()
	}
	return lastLine, scanner.Err()
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

func UseUserConfig(obj runtime.Object, config string) (runtime.Object, error) {
	kind, err := meta.NewAccessor().Kind(obj)
	if err != nil {
		return nil, err
	}
	switch kind {
	case "Job":
		pt := *obj.(*batchv1.Job)
		useUserConfig(&pt.Spec.Template.Spec, config)
		return runtime.Object(&pt), nil
	case "StatefulSet":
		pt := *obj.(*appsv1.StatefulSet)
		useUserConfig(&pt.Spec.Template.Spec, config)
		return runtime.Object(&pt), nil
	case "Deployment":
		pt := *obj.(*appsv1.Deployment)
		useUserConfig(&pt.Spec.Template.Spec, config)
		return runtime.Object(&pt), nil
	case "ReplicaSet":
		pt := *obj.(*appsv1.ReplicaSet)
		useUserConfig(&pt.Spec.Template.Spec, config)
		return runtime.Object(&pt), nil
	case "Pod":
		pt := *obj.(*corev1.Pod)
		useUserConfig(&pt.Spec, config)
		return runtime.Object(&pt), nil
	}
	return obj, nil
}

func useUserConfig(spec *corev1.PodSpec, config string) {
	for _, v := range spec.Volumes {
		if v.Name == "config" {
			v.ConfigMap.Name = config
			break
		}
	}
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
	} else if kind == "Pod" {
		pt := *obj.(*corev1.Pod)
		addParserContainer(&pt.Spec, parserCMName, logFilename)
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
	//c.Image = "python:3.8.0"
	c.Image = "loadbalancer:5000/cnsb/python"
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

func AddSyncContainerGeneric(obj runtime.Object, count int, actionName string) (runtime.Object, error) {
	kind, err := meta.NewAccessor().Kind(obj)
	if err != nil {
		return nil, err
	}
	if kind == "Job" {
		pt := *obj.(*batchv1.Job)
		addSyncContainer(&pt.Spec.Template.Spec, count, actionName)
		return runtime.Object(&pt), nil
	} else if kind == "StatefulSet" {
		pt := *obj.(*appsv1.StatefulSet)
		addSyncContainer(&pt.Spec.Template.Spec, count, actionName)
		return runtime.Object(&pt), nil
	}
	return obj, nil
}

func addSyncContainer(spec *corev1.PodSpec, count int, actionName string) {
	numContainers := len(spec.InitContainers)

	c := corev1.Container{}
	c.Name = "sync-container"
	c.Image = "dwdraju/alpine-curl-jq"
	c.Command = []string{"/scripts/ready.sh", "actionname%3D"+actionName, strconv.Itoa(numContainers*count)}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/scripts/",
			Name: "ready-script",
		},
	}
	spec.InitContainers = append(spec.InitContainers, c)

	syncVol := corev1.Volume{}
	syncVol.Name = "ready-script"
	syncCmvs := corev1.ConfigMapVolumeSource {}
	syncCmvs.DefaultMode = utilptr.Int32Ptr(0777)
	syncCmvs.Name = "ready-script"
	syncVol.ConfigMap = &syncCmvs
	spec.Volumes = append(spec.Volumes, syncVol)
}

func AddLabelsGeneric(obj runtime.Object, labels map[string]string) (runtime.Object, error) {
	kind, err := meta.NewAccessor().Kind(obj)
	if err != nil {
		return nil, err
	}
	if kind == "Job" {
		pt := *obj.(*batchv1.Job)
		addLabels(&pt.Spec.Template.ObjectMeta, labels)
		return runtime.Object(&pt), nil
	} else if kind == "StatefulSet" {
		pt := *obj.(*appsv1.StatefulSet)
		addLabels(&pt.Spec.Template.ObjectMeta, labels)
		return runtime.Object(&pt), nil
	}
	return obj, nil
}

func addLabels(spec *metav1.ObjectMeta, labels map[string]string) {
	spec.Labels = labels
}
