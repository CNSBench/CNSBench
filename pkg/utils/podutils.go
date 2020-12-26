package utils

import (
	"strconv"
        "k8s.io/apimachinery/pkg/runtime"
        "k8s.io/apimachinery/pkg/api/meta"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilptr "k8s.io/utils/pointer"
)

func podSpec(obj runtime.Object) (*corev1.PodSpec, error) {
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
	return nil, nil
}

func updatePodSpec(obj runtime.Object, spec corev1.PodSpec) (runtime.Object, error) {
	kind, err := meta.NewAccessor().Kind(obj)
	if err != nil {
		return nil, err
	}
	if kind == "Job" {
		pt := *obj.(*batchv1.Job)
		pt.Spec.Template.Spec = spec
		return runtime.Object(&pt), nil
	} else if kind == "Pod" {
		pt := *obj.(*corev1.Pod)
		pt.Spec = spec
		return runtime.Object(&pt), nil
	} else if kind == "StatefulSet" {
		pt := *obj.(*appsv1.StatefulSet)
		pt.Spec.Template.Spec = spec
		return runtime.Object(&pt), nil
	}
	return nil, nil
}

func volInSpec(vols []corev1.Volume, name string) bool {
	for _, v := range vols {
		if v.Name == name {
			return true
		}
	}
	return false
}


func newCMVol(name, cmName string) corev1.Volume {
	vol := corev1.Volume{}
	vol.Name = name
	cm := corev1.ConfigMapVolumeSource {}
	cm.DefaultMode = utilptr.Int32Ptr(0777)
	cm.Name = cmName
	vol.ConfigMap = &cm
	return vol
}

func addOutputVol(spec *corev1.PodSpec) {
	if !volInSpec(spec.Volumes, "output-vol") {
		outputVol := corev1.Volume{}
		outputVol.Name = "output-vol"
		outputVol.EmptyDir = &corev1.EmptyDirVolumeSource {}
		spec.Volumes = append(spec.Volumes, outputVol)
	}

	outputVolMount := corev1.VolumeMount {
		MountPath: "/output",
		Name: "output-vol",
	}
	for i, _ := range spec.Containers {
		foundMount := false
		for _, v := range spec.Containers[i].VolumeMounts {
			if v.Name == "output-vol" {
				foundMount = true
			}
		}
		if !foundMount {
			spec.Containers[i].VolumeMounts = append(spec.Containers[i].VolumeMounts, outputVolMount)
		}
	}
}

func AddParserContainer(obj runtime.Object, parserCMName, logFilename, imageName string, num int) (runtime.Object, error) {
	spec, err := podSpec(obj)
	if spec == nil || err != nil {
		return nil, err
	}

	c := corev1.Container{}
	c.Name = "parser-container"
	c.Image = imageName
	c.Command = []string{"sh", "-c", "/scripts/countdone.sh 1 && /scripts/parser.sh " + logFilename + " > " + logFilename + ".parsed"}
	//c.Command = []string{"sh", "-c", "tail -f /dev/null"}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/scripts/countdone.sh",
			SubPath: "countdone.sh",
			Name: "helper",
		},
		{
			//MountPath: "/scripts/parser-"+strconv.Itoa(num)+".sh",
			MountPath: "/scripts/parser.sh",
			SubPath: "parser.sh",
			//Name: "parser"+strconv.Itoa(num),
			Name: "parser",
		},
		{
			MountPath: "/output",
			Name: "output-vol",
		},
	}
	c.Env = []corev1.EnvVar {
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource {
				FieldRef: &corev1.ObjectFieldSelector {
					FieldPath: "metadata.name",
				},
			},
		},
	}
	spec.Containers = append(spec.Containers, c)

	if !volInSpec(spec.Volumes, "parser") {
		// If we have more than one output file we could have multiple parsers
		// For now, just assume we only have one file so we can just name the
		// vol "parser"
		spec.Volumes = append(spec.Volumes, newCMVol("parser", parserCMName))
	}

	if !volInSpec(spec.Volumes, "helper") {
		spec.Volumes = append(spec.Volumes, newCMVol("helper", "countdone"))
	}

	addOutputVol(spec)

	return updatePodSpec(obj, *spec)
}

func AddSyncContainer(obj runtime.Object, count int, workloadName string, syncGroup string) (runtime.Object, error) {
	spec, err := podSpec(obj)
	if spec == nil || err != nil {
		return nil, err
	}

	numContainers := len(spec.InitContainers)

	c := corev1.Container{}
	c.Name = "sync-container"
	c.Image = "dwdraju/alpine-curl-jq"
	c.Command = []string{"/scripts/ready.sh", "workloadname%3D"+workloadName, strconv.Itoa(numContainers*count)}
	if syncGroup != "" {
		c.Command = append(c.Command, "syncgroup%3D"+syncGroup)
	}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/scripts/",
			Name: "ready-script",
		},
	}
	spec.InitContainers = append(spec.InitContainers, c)

	spec.Volumes = append(spec.Volumes, newCMVol("ready-script", "ready-script"))

	return updatePodSpec(obj, *spec)
}

func AddOutputContainer(obj runtime.Object, outputArgs, outputContainer, outputFilename string) (runtime.Object, error) {
	spec, err := podSpec(obj)
	if spec == nil || err != nil {
		return nil, err
	}

	c := corev1.Container{}
	c.Name = "output-container"
	c.Image = "kubes1:5000/cnsb/helper"
	c.Command = []string{"sh", "-c", "/scripts/countdone.sh 2 && /scripts/output.sh " + outputFilename + ".parsed " + outputArgs}
	//c.Command = []string{"sh", "-c", "tail -f /dev/null"}
	c.VolumeMounts = []corev1.VolumeMount{
		{
			MountPath: "/scripts/countdone.sh",
			SubPath: "countdone.sh",
			Name: "helper",
		},
		{
			MountPath: "/scripts/output.sh",
			SubPath: "output.sh",
			Name: "output",
		},
		{
			MountPath: "/output",
			Name: "output-vol",
		},
	}
	c.Env = []corev1.EnvVar {
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource {
				FieldRef: &corev1.ObjectFieldSelector {
					FieldPath: "metadata.name",
				},
			},
		},
	}
	spec.Containers = append(spec.Containers, c)

	if !volInSpec(spec.Volumes, "output") {
		spec.Volumes = append(spec.Volumes, newCMVol("output", outputContainer))
	}

	if !volInSpec(spec.Volumes, "helper") {
		spec.Volumes = append(spec.Volumes, newCMVol("helper", "countdone"))
	}

	addOutputVol(spec)

	return updatePodSpec(obj, *spec)
}

func AddLabelsGeneric(obj runtime.Object, labels map[string]string) (runtime.Object, error) {
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

func SetEnvVar(name, value string, obj runtime.Object) (runtime.Object, error) {
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
