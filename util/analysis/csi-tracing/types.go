package main

import (
	"net"
	"time"

	"github.com/google/gopacket/layers"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

/* type CsiRPC
stores information about a csi gRPC call
stores type of csi RPC & appropriate request & response structs
*/
type CsiRPC = struct {
	clientIP     net.IP
	clientPort   layers.TCPPort
	clientName   string // :authority field of request
	csiPath      string
	request      interface{}
	response     interface{}
	requestTime  time.Time // capture timestamp of request
	responseTime time.Time // capture timestamp of response
}

/* type rawFrames
stores all raw (HTTP2) frames sent from a src host to a dst host
*/
type rawFrames = struct {
	srcIP, dstIP     net.IP
	srcPort, dstPort layers.TCPPort
	frames           []frames
}

/* type frames
stores all raw frames in a single captured packet
& the capturedTimestamp of the packet
*/
type frames = struct {
	frames           []byte
	captureTimestamp time.Time
}

/* type rawStreams
stores streams exchanged between two hosts, a and b
ab: frames from a to b for each stream
ba: frames from b to a for each stream
streams are identified by stream ID (uint32)
*/
type rawStreams = struct {
	aIP, bIP     net.IP
	aPort, bPort layers.TCPPort
	ab, ba       map[uint32]stream
}

/* type stream
stores frames belonging to the same stream (in one direction)
and the capture timestamp of the last frame in the stream
*/
type stream = struct {
	frames           []frame
	captureTimestamp time.Time
}

/* type frame
stores header & raw body of a frame
stores slice of HeaderFields for a decoded header block
*/
type frame = struct {
	header       http2.FrameHeader
	body         []byte
	headerFields []hpack.HeaderField
}
