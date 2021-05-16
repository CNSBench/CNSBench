package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/cnsbench/cnsbench/pkg/podutils"
	"helm.sh/helm/v3/pkg/chartutil"
)

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

	clientObjs := podutils.HelmChartBuilder(*repoURL, *repoName, *chartName, *releaseName, values)
	workloadObjsMap := podutils.GetWorkloadObjectsMap(workloadObjs, clientObjs)
	for k, v := range workloadObjsMap {
		fmt.Println(k, v)
	}
}
