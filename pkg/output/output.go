package output

import (
	"fmt"
	"bytes"
	"encoding/json"
	cnsbench "github.com/cnsbench/pkg/apis/cnsbench/v1alpha1"
)

type OutputStruct struct {
	Name string
	Spec cnsbench.BenchmarkSpec
	Results map[string]interface{}
	StartTime int64
	CompletionTime int64
}

func Output(parsedOutput map[string]interface{}, outputName string, bm *cnsbench.Benchmark, startTime int64, completionTime int64) error {
	o := OutputStruct{bm.ObjectMeta.Name, bm.Spec, parsedOutput, startTime, completionTime}
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
