package podutils

import (
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func HelmChartBuilder(repoURL, repoName, chartName, releaseName string, vals chartutil.Values) []client.Object {
	log.SetFlags(log.Lshortfile)

	settings := cli.New()
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), os.Getenv("HELM_DRIVER"), log.Printf); err != nil {
		log.Fatal(err)
	}
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	settings.RepositoryConfig = pwd + "/repositories.yaml"

	_, err = os.Stat(settings.RepositoryConfig)
	if err != nil && os.IsNotExist(err) {
		file, err := os.Create(settings.RepositoryConfig)
		if err != nil {
			log.Fatal(err)
		}
		file.Close()
	}

	repoFile, err := repo.LoadFile(settings.RepositoryConfig)
	if err != nil {
		log.Fatal(err)
	}

	repoEntry := repo.Entry{
		URL:  repoURL,
		Name: repoName,
	}
	chartRepo, err := repo.NewChartRepository(&repoEntry, getter.All(settings))
	if err != nil {
		log.Fatal(err)
	}

	_, err = chartRepo.DownloadIndexFile()
	if err != nil {
		log.Fatal(err)
	}

	if !repoFile.Has(repoEntry.Name) {
		repoFile.Update(&repoEntry)
		err := repoFile.WriteFile(settings.RepositoryConfig, os.ModePerm)
		if err != nil {
			log.Fatal(err)
		}
	}

	installClient := action.NewInstall(actionConfig)
	chartPath, err := installClient.LocateChart(repoName+"/"+chartName, settings)
	if err != nil {
		log.Fatal(err)
	}

	loadedChart, err := loader.Load(chartPath)
	if err != nil {
		log.Fatal(err)
	}

	if req := loadedChart.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(loadedChart, req); err != nil {
			if installClient.DependencyUpdate {
				mgr := &downloader.Manager{
					ChartPath:        chartPath,
					Keyring:          installClient.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          getter.All(settings),
					RepositoryConfig: settings.RegistryConfig,
					RepositoryCache:  settings.RepositoryCache,
				}
				if err := mgr.Update(); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Fatal(err)
			}
		}
	}

	installClient.ClientOnly = true
	installClient.ReleaseName = releaseName
	installClient.DryRun = true
	renderedRelease, err := installClient.Run(loadedChart, vals)
	if err != nil {
		log.Fatal(err)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	renderedTemplates := releaseutil.SplitManifests(renderedRelease.Manifest)

	clientObjs := make([]client.Object, 0)
	for _, yamlString := range renderedTemplates {
		rtObj, _, err := decode([]byte(yamlString), nil, nil)
		if err != nil {
			log.Fatal(err)
		}
		obj := rtObj.(client.Object)
		obj.SetCreationTimestamp(metav1.Time(actionConfig.Now()))
		clientObjs = append(clientObjs, obj)
	}

	return clientObjs
}

func decodeWorkloadObjsYAML(fileLocation string) []WorkloadObject {
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

func createWorkloadObject() WorkloadObject {
	return WorkloadObject{
		Role:      "helper",
		Duplicate: false,
		Sync:      false,
	}
}

func searchWorkloadObjects(workloadObjs []WorkloadObject, name string, objKind schema.ObjectKind) (WorkloadObject, bool) {
	for _, workloadObj := range workloadObjs {
		if workloadObj.Name == name && workloadObj.Kind == objKind.GroupVersionKind().Kind && workloadObj.ApiVersion == (objKind.GroupVersionKind().Group+objKind.GroupVersionKind().Version) {
			return workloadObj, true
		}
	}
	return createWorkloadObject(), false
}

func getWorkloadObjectsMap(workloadObjs []WorkloadObject, clientObjs []client.Object) map[string]interface{} {
	workloadObjsMap := make(map[string]interface{})
	for _, clientObj := range clientObjs {
		workloadObj, exists := searchWorkloadObjects(workloadObjs, clientObj.GetName(), clientObj.GetObjectKind())
		objKind := clientObj.GetObjectKind()
		workloadObj.Name = clientObj.GetName()
		workloadObj.Kind = objKind.GroupVersionKind().Kind
		workloadObj.ApiVersion = (objKind.GroupVersionKind().Group + objKind.GroupVersionKind().Version)
		workloadObjsMap[objKind.GroupVersionKind().Group+objKind.GroupVersionKind().Version+"/"+objKind.GroupVersionKind().Kind+"/"+clientObj.GetName()] = workloadObj
		if !exists {
			workloadObjs = append(workloadObjs, workloadObj)
		}
	}

	return workloadObjsMap
}

// Uncomment main function and change package to main to use this.
// func main() {
// 	repoURL := flag.String("repo-url", "", "Helm Repo URL")
// 	repoName := flag.String("repo-name", "", "Helm Repo Name")
// 	chartName := flag.String("chart", "", "Helm Chart Name")
// 	releaseName := flag.String("release", "", "Release Name")
// 	valuesFile := flag.String("values", "", "Chart Values File")
// 	workloadObjsFile := flag.String("workload-objs", "", "Workload Objects File")

// 	flag.Parse()

// 	if *repoURL == "" || *repoName == "" || *chartName == "" || *releaseName == "" {
// 		log.Fatal("Invalid Arguments")
// 	}

// 	var values chartutil.Values
// 	var err error
// 	if *valuesFile == "" {
// 		values = nil
// 	} else {
// 		values, err = chartutil.ReadValuesFile(*valuesFile)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 	}

// 	var workloadObjs []WorkloadObject
// 	if *workloadObjsFile == "" {
// 		workloadObjs = make([]WorkloadObject, 0)
// 	} else {
// 		workloadObjs = decodeWorkloadObjsYAML(*workloadObjsFile)
// 	}

// 	clientObjs := HelmChartBuilder(*repoURL, *repoName, *chartName, *releaseName, values)
// 	workloadObjsMap := getWorkloadObjectsMap(workloadObjs, clientObjs)
// 	for k, v := range workloadObjsMap {
// 		fmt.Println(k, v)
// 	}
// }
