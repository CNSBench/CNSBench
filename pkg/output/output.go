package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	cnsbench "github.com/cnsbench/cnsbench/api/v1alpha1"
)

type OutputStruct struct {
	Name               string
	Spec               cnsbench.BenchmarkSpec
	StartTime          int64
	CompletionTime     int64
	InitCompletionTime int64
}

type MetricStruct struct {
	Name   string
	Type   string
	Metric string
}

func Metric(outputs []cnsbench.Output, outputName, benchmarkName, metricType, metric string) {
	o := MetricStruct{benchmarkName, metricType, metric}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(o); err != nil {
		fmt.Println(err)
		return
	}
	reader := bytes.NewReader(buf.Bytes())
	if outputName == "" {
		if err := HttpPost(reader, "http://cnsbench-output-collector.cnsbench-system.svc.cluster.local:8888/metadata/"+benchmarkName); err != nil {
			fmt.Println(err)
			return
		}
	} else {
		for _, out := range outputs {
			fmt.Println(out)
			fmt.Println(outputName)
			if out.Name == outputName {
				if out.HttpPostSpec.URL != "" {
					if err := HttpPost(reader, out.HttpPostSpec.URL); err != nil {
						fmt.Println(err)
						return
					}
				}
			}
		}
	}
}

func Output(outputName string, bm *cnsbench.Benchmark, startTime, completionTime, initCompletionTime int64) error {
	o := OutputStruct{bm.ObjectMeta.Name, bm.Spec, startTime, completionTime, initCompletionTime}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(o); err != nil {
		return err
	}
	reader := bytes.NewReader(buf.Bytes())

	if outputName == "" {
		if err := HttpPost(reader, "http://cnsbench-output-collector.cnsbench-system.svc.cluster.local:8888/metadata/"+bm.ObjectMeta.Name); err != nil {
			return err
		}
	} else {
		for _, out := range bm.Spec.Outputs {
			fmt.Println(out)
			fmt.Println(outputName)
			if out.Name == outputName {
				if out.HttpPostSpec.URL != "" {
					if err := HttpPost(reader, out.HttpPostSpec.URL); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
