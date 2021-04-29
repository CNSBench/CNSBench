package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/cnsbench/cnsbench/pkg/podutils"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func HelmChartBuilder(repoURL, repoName, chartName, releaseName string, vals chartutil.Values) []client.Object {
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

func main() {
	repoURL := flag.String("repo-url", "", "Helm Repo URL")
	repoName := flag.String("repo-name", "", "Helm Repo Name")
	chartName := flag.String("chart", "", "Helm Chart Name")
	releaseName := flag.String("release", "", "Release Name")
	valuesFile := flag.String("values", "", "Chart Values File")
	workloadObjsFile := flag.String("workload-objs", "", "Workload Objects File")

	flag.Parse()

	if *repoURL == "" || *repoName == "" || *chartName == "" || *releaseName == "" {
		log.Fatal("Invalid Arguments")
	}

	var values chartutil.Values
	var err error
	if *valuesFile == "" {
		values = nil
	} else {
		values, err = chartutil.ReadValuesFile(*valuesFile)
		if err != nil {
			log.Fatal(err)
		}
	}

	var workloadObjs []podutils.WorkloadObject
	if *workloadObjsFile == "" {
		workloadObjs = make([]podutils.WorkloadObject, 0)
	} else {
		workloadObjs = podutils.DecodeWorkloadObjsYAML(*workloadObjsFile)
	}

	clientObjs := HelmChartBuilder(*repoURL, *repoName, *chartName, *releaseName, values)
	workloadObjsMap := podutils.GetWorkloadObjectsMap(workloadObjs, clientObjs)
	for k, v := range workloadObjsMap {
		fmt.Println(k, v)
	}
}
