package podutils

import (
	"io/ioutil"
	"log"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Output struct {
	File   string `yaml:"file"`
	Parser string `yaml:"parser"`
}

type WorkloadObject struct {
	Name       string   `yaml:"name"`
	Kind       string   `yaml:"kind"`
	ApiVersion string   `yaml:"apiVersion"`
	Role       string   `yaml:"role"`
	Duplicate  bool     `yaml:"duplicate"`
	Sync       bool     `yaml:"sync"`
	Outputs    []Output `yaml:"outputs"`
}

type WorkloadObjectsArray struct {
	WorkloadObjects []WorkloadObject `yaml:"workloadObjects"`
}

func DecodeWorkloadObjsYAML(fileLocation string) []WorkloadObject {
	yamlData, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		log.Fatal(err)
	}

	workloadObjsArray := WorkloadObjectsArray{}
	err = yaml.Unmarshal(yamlData, &workloadObjsArray)
	if err != nil {
		log.Fatal(err)
	}

	for i := range workloadObjsArray.WorkloadObjects {
		if workloadObjsArray.WorkloadObjects[i].Role != "volume" && workloadObjsArray.WorkloadObjects[i].Role != "workload" {
			workloadObjsArray.WorkloadObjects[i].Role = "helper"
		}
		for j := range workloadObjsArray.WorkloadObjects[i].Outputs {
			if workloadObjsArray.WorkloadObjects[i].Outputs[j].Parser == "" {
				workloadObjsArray.WorkloadObjects[i].Outputs[j].Parser = "null-parser"
			}
		}
	}

	return workloadObjsArray.WorkloadObjects
}

func CreateWorkloadObject() WorkloadObject {
	return WorkloadObject{
		Role:      "helper",
		Duplicate: false,
		Sync:      false,
	}
}

func SearchWorkloadObjects(workloadObjs []WorkloadObject, name string, objKind schema.ObjectKind) int {
	for index, workloadObj := range workloadObjs {
		if workloadObj.Name == name && workloadObj.Kind == objKind.GroupVersionKind().Kind && workloadObj.ApiVersion == (objKind.GroupVersionKind().Group+objKind.GroupVersionKind().Version) {
			return index
		}
	}
	return -1
}

func GetWorkloadObjectsMap(workloadObjs []WorkloadObject, clientObjs []client.Object) map[string]WorkloadObject {
	var workloadObj WorkloadObject
	workloadObjsMap := make(map[string]WorkloadObject)
	for _, clientObj := range clientObjs {
		objKind := clientObj.GetObjectKind()
		index := SearchWorkloadObjects(workloadObjs, clientObj.GetName(), clientObj.GetObjectKind())
		if index < 0 {
			workloadObj = CreateWorkloadObject()
			workloadObj.Name = clientObj.GetName()
			workloadObj.Kind = objKind.GroupVersionKind().Kind
			workloadObj.ApiVersion = (objKind.GroupVersionKind().Group + objKind.GroupVersionKind().Version)
			workloadObjs = append(workloadObjs, workloadObj)
		} else {
			workloadObj = workloadObjs[index]
		}
		workloadObjsMap[objKind.GroupVersionKind().Group+objKind.GroupVersionKind().Version+"/"+objKind.GroupVersionKind().Kind+"/"+clientObj.GetName()] = workloadObj
	}

	return workloadObjsMap
}

func replaceVars(cmString string, vars map[string]string, instanceNum, numInstances int, workloadName string, workloadConfigMap *corev1.ConfigMap) string {
	for variable, value := range vars {
		log.Println("Searching for var", "var", variable, "replacement", value)
		cmString = strings.ReplaceAll(cmString, "{{"+variable+"}}", value)
	}

	// Use workload spec's annotations as default values for any remaining variables
	for k, v := range workloadConfigMap.ObjectMeta.Annotations {
		s := strings.SplitN(k, ".", 3)
		if len(s) == 3 && s[0] == "cnsbench" && s[1] == "default" {
			log.Println("Searching for var", "var", s[2], "default replacement", v)
			cmString = strings.ReplaceAll(cmString, "{{"+s[2]+"}}", v)
		}
	}
	cmString = strings.ReplaceAll(cmString, "{{ACTION_NAME}}", workloadName)
	cmString = strings.ReplaceAll(cmString, "{{ACTION_NAME_CAPS}}", strings.ToUpper(workloadName))
	cmString = strings.ReplaceAll(cmString, "{{INSTANCE_NUM}}", strconv.Itoa(instanceNum))
	cmString = strings.ReplaceAll(cmString, "{{NUM_INSTANCES}}", strconv.Itoa(numInstances))

	if strings.Contains(cmString, "{{") {
		log.Println("Object definition contains double curly brackets ({{), possible unset variable", "cmstring", cmString)
	}

	return cmString
}

func DecodeWorkloadSpecsYAML(fileLocation string, vars map[string]string) []WorkloadObject {
	yamlData, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		log.Fatal(err)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	Obj, _, err := decode([]byte(yamlData), nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	configMap := Obj.(*corev1.ConfigMap)

	workloadObjs := make([]WorkloadObject, 0)
	for k := range configMap.Data {
		yamlString := replaceVars(configMap.Data[k], vars, 0, 0, "fio", configMap)
		rtObj, _, err := decode([]byte(yamlString), nil, nil)
		if err != nil {
			log.Fatal(err)
		}

		annotations, err := meta.NewAccessor().Annotations(rtObj)
		if err != nil {
			log.Fatal(err)
		}

		workloadObj := CreateWorkloadObject()
		workloadObj.ApiVersion = rtObj.GetObjectKind().GroupVersionKind().Group + rtObj.GetObjectKind().GroupVersionKind().Version
		workloadObj.Kind = rtObj.GetObjectKind().GroupVersionKind().Kind
		workloadObj.Name = annotations["name"]
		if _, ok := annotations["role"]; ok {
			if annotations["role"] == "volume" || annotations["role"] == "workload" {
				workloadObj.Role = annotations["role"]
			}
		}
		if _, ok := annotations["sync"]; ok {
			sync, err := strconv.ParseBool(annotations["sync"])
			if err == nil {
				workloadObj.Sync = sync
			}
		}
		if _, ok := annotations["duplicate"]; ok {
			duplicate, err := strconv.ParseBool(annotations["duplicate"])
			if err == nil {
				workloadObj.Duplicate = duplicate
			}
		}
		if _, ok := annotations["outputFile"]; ok {
			workloadObj.Outputs = make([]Output, 1)
			workloadObj.Outputs[0].File = annotations["outputFile"]
			if _, ok := annotations["parser"]; ok {
				workloadObj.Outputs[0].Parser = annotations["parser"]
			} else {
				workloadObj.Outputs[0].Parser = "null-parser"
			}
		}
		workloadObjs = append(workloadObjs, workloadObj)
	}

	return workloadObjs
}
