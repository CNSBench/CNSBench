package output

import (
	"fmt"
	"bytes"
	"io/ioutil"
	"net/http"
)

func HttpPost(reader *bytes.Reader, url string) error {
	req, err := http.NewRequest("POST", url, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if resp != nil && resp.ContentLength > 0 {
			buf := new(bytes.Buffer)
			buf.ReadFrom(resp.Body)
			fmt.Println(buf.String())
		}
		return err
	}
	defer resp.Body.Close()
	fmt.Println("Response status", resp.Status)
	b, err := ioutil.ReadAll(resp.Body)
	fmt.Println("Response body", string(b))

	return nil
}
