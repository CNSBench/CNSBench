package podutils

import (
	"io/ioutil"
	"log"
	"strconv"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

type Annotations struct {
	Role       string `yaml:"role"`
	Duplicate  string `yaml:"duplicate"`
	Sync       string `yaml:"sync"`
	OutputFile string `yaml:"outputFile"`
	Parser     string `yaml:"parser"`
}

type Metadata struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
	Labels      map[string]string `yaml:"labels"`
	Annotations Annotations       `yaml:"annotations"`
}

type WorkloadSpecObject struct {
	ApiVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
}

type WorkloadSpec struct {
	ApiVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   Metadata          `yaml:"metadata"`
	Data       map[string]string `yaml:"data"`
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

func DecodeWorkloadSpecsYAML(fileLocation string) []WorkloadObject {
	yamlData, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		log.Fatal(err)
	}

	workload := WorkloadSpec{}
	err = yaml.Unmarshal(yamlData, &workload)
	if err != nil {
		log.Fatal(err)
	}

	workloadObjs := make([]WorkloadObject, 0)
	for _, yamlString := range workload.Data {
		workloadSpecObj := WorkloadSpecObject{}
		err = yaml.Unmarshal([]byte(yamlString), &workloadSpecObj)
		if err != nil {
			log.Fatal(err)
		}
		workloadObj := CreateWorkloadObject()
		workloadObj.ApiVersion = workloadSpecObj.ApiVersion
		workloadObj.Kind = workloadSpecObj.Kind
		workloadObj.Name = workloadSpecObj.Metadata.Name
		if workloadSpecObj.Metadata.Annotations.Role != "volume" && workloadSpecObj.Metadata.Annotations.Role != "workload" {
			workloadObj.Role = "helper"
		} else {
			workloadObj.Role = workloadSpecObj.Metadata.Annotations.Role
		}
		workloadObj.Sync, _ = strconv.ParseBool(workloadSpecObj.Metadata.Annotations.Sync)
		workloadObj.Duplicate, _ = strconv.ParseBool(workloadSpecObj.Metadata.Annotations.Duplicate)
		if workloadSpecObj.Metadata.Annotations.OutputFile != "" {
			workloadObj.Outputs = make([]Output, 1)
			workloadObj.Outputs[0].File = workloadSpecObj.Metadata.Annotations.OutputFile
			if workloadSpecObj.Metadata.Annotations.Parser != "" {
				workloadObj.Outputs[0].Parser = workloadSpecObj.Metadata.Annotations.Parser
			} else {
				workloadObj.Outputs[0].Parser = "null-parser"
			}
		}
		workloadObjs = append(workloadObjs, workloadObj)
	}

	return workloadObjs
}
