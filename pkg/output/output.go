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

func Output(outputName string, bm *cnsbench.Benchmark, startTime, completionTime, initCompletionTime int64) error {
	o := OutputStruct{bm.ObjectMeta.Name, bm.Spec, startTime, completionTime, initCompletionTime}
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(o); err != nil {
		return err
	}
	reader := bytes.NewReader(buf.Bytes())

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
	return nil
}
