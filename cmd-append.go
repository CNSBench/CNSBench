package main

import (
	"context"
	"fmt"
	"strings"
	"io/ioutil"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"github.com/cnsbench/cnsbench/controllers"
)

func main() {
	/* Check args */
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run cmd-append.go <config file> <pod file>\n")
		os.Exit(0)
	}

	/* Setup pod based on config file */
	objBytes, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("Error reading file contents", err)
		os.Exit(1)
	}

	// Decode the yaml object from the workload spec
	decode := scheme.Codecs.UniversalDeserializer().Decode
	robj, _, err := decode(objBytes, nil, nil)
	if err != nil {
		fmt.Println("Error decoding yaml", err)
		os.Exit(1)
	}
	obj := robj.(client.Object)

	cl, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		fmt.Println("Failed to create client", err)
		os.Exit(1)
	}

	/* Copy pod file log to /output */
	dirpath := filepath.Dir(os.Args[2])
	cmd := fmt.Sprintf("cp %s /output%s;", os.Args[2], os.Args[2])
	if strings.Compare(dirpath, ".") != 0 {
		cmd = fmt.Sprintf("mkdir -p /output%s; ", dirpath) + cmd
	}
	cmds := []string{cmd}
	if obj, err = controllers.AppendWorkloadContainerCmd(obj, cmds); err != nil {
		os.Exit(1)
	}

	/* Create/run the pod */
	err = cl.Create(context.TODO(), obj)
	if err != nil {
		fmt.Println("Failed creating object", err)
		os.Exit(1)
	}

	fmt.Println("Pod created")
}
