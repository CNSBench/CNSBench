package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"os"
)

const usage string = `usage:
go run *.go [filename] --rs=[resources] --ns=[namespaces]

	filename: name of the audit log file to read from
		Default is "/var/log/apiserver/audit.log"

	resources: comma-separated list of resources, i.e.
		pod,pvc,deployment,job,replicaset,statefulset
		Default resource is "pod"

	namespaces: comma-separated list of namespaces
		Default namespace is "default"
`

func main() {

	// Parse CLI
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
	}
	var resourcePtr, nsPtr string
	flag.StringVar(&resourcePtr, "rs", "pod", "")
	flag.StringVar(&nsPtr, "ns", "default", "")
	flag.Parse()

	tail := flag.Args()
	var file string
	if len(tail) == 0 {
		file = "/var/log/apiserver/audit.log"
	} else {
		file = tail[0]
	}
	resources := strings.Split(resourcePtr, ",")
	namespaces := strings.Split(nsPtr, ",")

	// Set flags based on resources selected
	var (
		flags uint8 = 0
		pod bool = false
		pvc bool = false
	)
	for _, r := range resources {
		if r == "pod" {
			flags |= ParseCreate
			pod = true
		} else if r == "pvc" {
			flags |= ParseCreate
			pvc = true
		} else if r == "pv" {
			flags |= ParseCreate
		} else if r == "deployment" || r == "job" || r == "replicaset" ||
				r == "statefulset" {
			flags |= ParseScale
		}
	}
	if pod && pvc {
		flags |= ParsePVCPod
	}

	// Parse logs
	err := ParseLogs(file, flags, namespaces)
	if err != nil {
		log.Fatal(err)
	}
}
