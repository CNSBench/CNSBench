package csiparser

import (
	"net"

	"github.com/google/gopacket/layers"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func parseRPCs(rawstreams []rawStreams) []CsiRPC {
	var rpcs []CsiRPC
	// For each pair of hosts between which there exist streams:
	for _, streams := range rawstreams {
		for sid, abStrm := range streams.ab {
			// For each pair of opposite streams with the same stream ID:
			if baStrm, ok:= streams.ba[sid]; ok {
				// Figure out which is request & which is response
				var (
					request, response stream
					clientIP net.IP
					clientPort layers.TCPPort
				)
				if isGRPCRequest(abStrm) && isGRPCResponse(baStrm) {
					request, response = abStrm, baStrm
					clientIP, clientPort = streams.aIP, streams.aPort
				} else if isGRPCRequest(baStrm) && isGRPCResponse(abStrm) {
					request, response = baStrm, abStrm
					clientIP, clientPort = streams.bIP, streams.bPort
				} else {
					continue
				}
				// Create a CsiRPC struct from the stream
				curRPC, ok := parseGRPC(request, response, clientIP, clientPort)
				// Append to rpcs
				if ok {
					rpcs = append(rpcs, curRPC)
				}
			}
		}
	}
	return rpcs
}

func isGRPCRequest(strm stream) bool {
	// There must be at least 1 headers & 1 data frame
	if len(strm.frames) < 2 {
		return false
	}
	// Frame 0 must be headers
	if strm.frames[0].header.Type != http2.FrameHeaders {
		return false
	}
	// Frame 1 and on must be data
	for i := 1; i < len(strm.frames); i++ {
		if strm.frames[i].header.Type != http2.FrameData {
			return false
		}
	}
	// Make sure all necessary headers are in frame 0
	hdrs := strm.frames[0].headerFields
	method, scheme, contentType := false, false, false
	for _, h := range hdrs {
		if h.Name == ":method" && h.Value == "POST" {
			method = true
		} else if h.Name == ":scheme" && h.Value == "http" {
			scheme = true
		} else if h.Name == "content-type" && h.Value == "application/grpc" {
			contentType = true
		}
	}
	return method && scheme && contentType
}

// Will return true for a successful gRPC response
func isGRPCResponse(strm stream) bool {
	// There must be at least 1 header, 1 data, 1 header frame
	if len(strm.frames) < 3 {
		return false
	}
	// Frame 0 must be headers
	if strm.frames[0].header.Type != http2.FrameHeaders {
		return false
	}
	// Frame 1~(n-1) must be data
	for i := 1; i < len(strm.frames)-1; i++ {
		if strm.frames[i].header.Type != http2.FrameData {
			return false
		}
	}
	// Frame n must be header
	if strm.frames[len(strm.frames)-1].header.Type != http2.FrameHeaders {
		return false
	}
	// Frame 0 required headers
	status, contentType := false, false
	for _, h := range strm.frames[0].headerFields {
		if h.Name == ":status" && h.Value == "200" {
			status = true
		} else if h.Name == "content-type" && h.Value == "application/grpc" {
			contentType = true
		}
	}
	return status && contentType
}

func parseGRPC(request, response stream, clientIP net.IP,
	clientPort layers.TCPPort) (CsiRPC, bool) {
	// Find :path in request stream's header
	path := findField(request.frames[0].headerFields, ":path")
	// Find :authority in request stream's header
	clientName := findField(request.frames[0].headerFields, ":authority")
	// Parse into service & rpc
	service, rpc := parseRPCPath(path)
	// Find data blocks of both request & response
	reqData := request.frames[1].body
	repData := response.frames[1].body
	// Parse request & response
	req := parseCSIRequest(reqData, service, rpc)
	rep := parseCSIResponse(repData, service, rpc)
	// Construct CsiRPC struct
	rpcStruct := CsiRPC{clientIP, clientPort, clientName, path, req, rep,
		request.captureTimestamp, response.captureTimestamp}
	if req != nil && rep != nil {
		return rpcStruct, true
	} else {
		return rpcStruct, false
	}
}

func findField(headerFields []hpack.HeaderField, field string) string {
	for _, h := range headerFields {
		if h.Name == field {
			return h.Value
		}
	}
	return ""
}

func parseRPCPath(path string) (service, rpc string) {
	path = path[len("/csi.v1."):]
	split := -1
	for i, c := range path {
		if c == '/' {
			split = i
		}
	}
	service = path[:split]
	rpc = path[split+1:]
	return
}
