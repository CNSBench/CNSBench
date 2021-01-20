package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Output struct {
	Timestamp  int64  `json:"timestamp"`
	RemoteAddr string `json:"remoteAddr"`
	Endpoint   string `json:"endpoint"`
	Params	   map[string][]string `json:"params"`
	Data       string `json:"data"`
}

func handler(w http.ResponseWriter, req *http.Request) {
	data := ""
	buf := new(strings.Builder)

	if _, err := io.Copy(buf, req.Body); err != nil {
		fmt.Println(err)
	} else {
		data = buf.String()
	}

	output := &Output{time.Now().Unix(), req.RemoteAddr, req.URL.Path, req.URL.Query(), data}

	if j, err := json.Marshal(output); err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(string(j))
	}
}

func main() {
	http.HandleFunc("/", handler)

	http.ListenAndServe(":8888", nil)
}
