package output

import (
	"fmt"
	"bytes"
	"net/http"
	"encoding/json"
)

func HttpPost(o OutputStruct, url string) error {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(o)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			fmt.Println(buf.String())
		return err
	}
	defer resp.Body.Close()
	fmt.Println("Response status", resp.Status)

	return nil
}
