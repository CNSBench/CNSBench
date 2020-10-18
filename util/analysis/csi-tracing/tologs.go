package csiparser

import (
	"encoding/json"
	"strings"
	"time"
)

type CsiLog struct {
	Action			string			`json:"action"`
	Path			string			`json:"path"`
	Agent struct {
		Name		string			`json:"name"`
		Ip			string			`json:"ip"`
		Port		string			`json:"port"`
	}								`json:"agent"`
	Timing struct {
		Start		time.Time		`json:"start"`
		End			time.Time		`json:"end"`
		Duration	time.Duration	`json:"duration"`
	}								`json:"timing"`
	Request			interface{}		`json:"request"`
	Response		interface{}		`json:"response"`
}

func ToJson(log CsiLog) string {
	output, _ := json.Marshal(log)
	return string(output)
}

func PcapToLogs(fileName string) []CsiLog {
	rpcs := parseCSIPcapFile(fileName)
	return extractLogs(rpcs)
}

func parseCSIPcapFile(fileName string) []CsiRPC {
	rawframes := parsePackets(fileName)
	rawstreams := extractStreams(rawframes)
	rpcs := parseRPCs(rawstreams)
	sortRPCs(rpcs)
	return rpcs
}

func extractLogs(rpcs []CsiRPC) []CsiLog {
	var logs []CsiLog
	for _, r := range rpcs {
		if log, ok := processRPC(r); ok {
			logs = append(logs, log)
		}
	}
	return logs
}

func processRPC(r CsiRPC) (CsiLog, bool) {
	var log CsiLog
	path := strings.Split(r.csiPath, "/")
	rpc := path[len(path)-1]
	log.Action = rpc
	log.Path = r.csiPath
	log.Agent.Name = r.clientName
	log.Agent.Ip = r.clientIP.String()
	log.Agent.Port = r.clientPort.String()
	log.Timing.Start = r.requestTime
	log.Timing.End = r.responseTime
	log.Timing.Duration = r.responseTime.Sub(r.requestTime)
	log.Request = r.request
	log.Response = r.response
	return log, true
}
