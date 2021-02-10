package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	cnsbench "github.com/cnsbench/cnsbench/api/v1alpha1"
)

type OutputStruct struct {
	Name               string                 `json:"name"`
	Spec               cnsbench.BenchmarkSpec `json:"spec"`
	StartTime          int64                  `json:"startTime"`
	CompletionTime     int64                  `json:"completionTime"`
	InitCompletionTime int64                  `json:"initCompletionTime"`
}

func doOutput(outputs []cnsbench.Output, reader *bytes.Reader, outputName, benchmarkName string) {
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

func Metric(outputs []cnsbench.Output, outputName, benchmarkName string, metrics ...string) {
	if (len(metrics) % 2) != 0 {
		fmt.Println("!!! uneven number of metric key, value pairs given.  Ignoring last string")
		metrics = metrics[:len(metrics)-1]
	}
	metricsMap := make(map[string]string, 0)
	for i := 0; i < len(metrics); i += 2 {
		metricsMap[metrics[i]] = metrics[i+1]
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(metricsMap); err != nil {
		fmt.Println(err)
		return
	}
	reader := bytes.NewReader(buf.Bytes())

	doOutput(outputs, reader, outputName, benchmarkName)
}

func Output(outputName string, bm *cnsbench.Benchmark, startTime, completionTime, initCompletionTime int64) error {
	o := OutputStruct{bm.ObjectMeta.Name, bm.Spec, startTime, completionTime, initCompletionTime}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(o); err != nil {
		return err
	}
	reader := bytes.NewReader(buf.Bytes())

	doOutput(bm.Spec.Outputs, reader, outputName, bm.ObjectMeta.Name)

	return nil
}
