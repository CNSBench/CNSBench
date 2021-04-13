package main

import (
	"flag"
	"fmt"
	"os"
)

const usage string = `usage:
go run *.go [filename]

	filename: name of the pcap file to read from
`

func main() {

	// Parse command-line args
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usage)
	}
	flag.Parse()
	if len(flag.Args()) == 0 {
		flag.Usage()
		return
	}
	filename := flag.Args()[0]

	logs := pcapToLogs(filename)
	for _, log := range logs {
		fmt.Println(toJson(log))
	}
}
