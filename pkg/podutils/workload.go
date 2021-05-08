package podutils

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"path"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var cl client.Client

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

type WorkloadConfigMapNames struct {
	ReadyScript      string
	CountdoneScript  string
	ParserConfigMaps map[string]string
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

func loadScript(scriptName string) (string, error) {
	if b, err := ioutil.ReadFile(path.Join("/scripts/", scriptName)); err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}

func createObj(obj client.Object) error {
	name, _ := meta.NewAccessor().Name(obj)

	for _, x := range scheme.Codecs.SupportedMediaTypes() {
		if x.MediaType == "application/yaml" {
			ptBytes, err := runtime.Encode(x.Serializer, obj)
			if err != nil {
				log.Println(err, "Error encoding spec")
			}
			fmt.Println(string(ptBytes))
		}
	}

	if err := cl.Create(context.TODO(), obj); err != nil {
		if errors.IsAlreadyExists(err) {
			log.Println("Object already exists, proceeding", "name", name)
		} else {
			return err
		}
	}

	return nil
}

func createTmpConfigMapFromDisk(scriptName string) (string, error) {
	script, err := loadScript(scriptName)
	if err != nil {
		log.Println(err, "Error creating tmp configmap", "script", scriptName)
		return "", err
	}

	newCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.NameGenerator.GenerateName(names.SimpleNameGenerator, "helper-"),
			Namespace: "default",
		},
		Data: map[string]string{
			scriptName: script,
		},
	}

	if err := createObj(client.Object(&newCm)); err != nil {
		log.Println(err, "Creating temp ConfigMap for", scriptName)
	}

	return newCm.ObjectMeta.Name, err
}

func cloneParser(parserName string) (string, error) {
	parserCm := &corev1.ConfigMap{}

	newCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.NameGenerator.GenerateName(names.SimpleNameGenerator, parserName+"-"),
			Namespace: "default",
		},
		Data: make(map[string]string),
	}
	for k, v := range parserCm.Data {
		newCm.Data[k] = v
	}

	var err error
	if err = createObj(client.Object(&newCm)); err != nil {
		log.Println(err, "Creating temp ConfigMap")
	}

	return newCm.ObjectMeta.Name, err
}

func WorkloadConfigMapNamesBuilder(workloadObjs []WorkloadObject) WorkloadConfigMapNames {
	workloadCMNames := WorkloadConfigMapNames{}
	workloadCMNames.ParserConfigMaps = make(map[string]string)

	for i := range workloadObjs {
		if workloadObjs[i].Sync && workloadCMNames.ReadyScript == "" {
			name, err := createTmpConfigMapFromDisk("ready.sh")
			if err != nil {
				log.Fatal(err)
			}

			workloadCMNames.ReadyScript = name
		}

		if len(workloadObjs[i].Outputs) > 0 {
			if workloadCMNames.CountdoneScript == "" {
				name, err := createTmpConfigMapFromDisk("countdone.sh")
				if err != nil {
					log.Fatal(err)
				}

				workloadCMNames.CountdoneScript = name
			}

			for _, output := range workloadObjs[i].Outputs {
				if _, ok := workloadCMNames.ParserConfigMaps[output.Parser]; !ok {
					name, err := cloneParser(output.Parser)
					if err != nil {
						log.Fatal(err)
					}

					workloadCMNames.ParserConfigMaps[output.Parser] = name
				}
			}
		}
	}

	return workloadCMNames
}
