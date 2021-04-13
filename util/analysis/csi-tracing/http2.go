package main

import (
	"bytes"
	"io"
	"log"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func extractStreams(rfs map[string]rawFrames) []rawStreams {
	var streams []rawStreams
	for abID, rf := range rfs {
		// Extract info from current src-dst pair, then delete its rawFrames
		aIP, bIP := rf.srcIP, rf.dstIP
		aPort, bPort := rf.srcPort, rf.dstPort
		abFrames := rf.frames
		delete(rfs, abID)
		// Extract info from opposite src-dst pair, the delete that rawFrames
		baID := srcdstIdentifier(bIP, aIP, bPort, aPort)
		x, ok := rfs[baID]
		if !ok {
			continue
		}
		baFrames := x.frames
		delete(rfs, baID)
		// Construct rawStreams struct
		rs := rawStreams{aIP, bIP, aPort, bPort, nil, nil}
		rs.ab = framesToStreams(abFrames)
		rs.ba = framesToStreams(baFrames)
		// Append rawStreams struct to streams
		streams = append(streams, rs)
	}
	return streams
}

type streaminfo = struct {
	headersEnded bool
	headersIndex int
	stream       stream
}

func framesToStreams(allFrames []frames) map[uint32]stream {
	// Decoder with default max dynamic table size
	decoder := hpack.NewDecoder(4096, nil)
	// Magic
	magic := http2.ClientPreface
	// Initialize streaminfos struct
	streaminfos := make(map[uint32]streaminfo)
	for _, pktFrames := range allFrames {
		timestamp := pktFrames.captureTimestamp
		frameBytes := pktFrames.frames
		// If there's a magic prefix in the beginning of frameBytes, remove it
		if len(frameBytes) >= len(magic) &&
			string(frameBytes[:len(magic)]) == magic {
			frameBytes = frameBytes[len(magic):]
		}
		// Create new Framer to parse frames from frameBytes
		r := bytes.NewReader(frameBytes)
		framer := http2.NewFramer(nil, r)
		// Parse
		for true {
			// Read the next frame
			curFrame, err := framer.ReadFrame()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}
			// Get header of frame
			hdr := copyHdr(curFrame.Header())
			// Get stream ID of frame
			sid := hdr.StreamID
			// Create new streaminfo for sid if necessary
			if _, ok := streaminfos[sid]; !ok {
				streaminfos[sid] = streaminfo{true, -1, stream{}}
			}
			si := streaminfos[sid]
			// Only store Data, Headers, and PushPromise frames (includes Continuations)
			if hdr.Type == http2.FrameData {
				// Append frame
				body := copyBuf(curFrame.(*http2.DataFrame).Data())
				si.stream.frames = append(si.stream.frames, frame{hdr, body, nil})
				// Set/update captureTimestamp
				si.stream.captureTimestamp = timestamp
			} else if hdr.Type == http2.FrameHeaders || hdr.Type == http2.FramePushPromise {
				// Append frame
				var body []byte
				if hdr.Type == http2.FrameHeaders {
					body = copyBuf(curFrame.(*http2.HeadersFrame).HeaderBlockFragment())
				} else {
					body = copyBuf(curFrame.(*http2.PushPromiseFrame).HeaderBlockFragment())
				}
				si.stream.frames = append(si.stream.frames, frame{hdr, body, nil})
				// Update headersEnded tracking
				if hdr.Type == http2.FrameHeaders {
					si.headersEnded = curFrame.(*http2.HeadersFrame).HeadersEnded()
				} else {
					si.headersEnded = curFrame.(*http2.PushPromiseFrame).HeadersEnded()
				}
				// Update headersIndex
				si.headersIndex = len(si.stream.frames) - 1
				// If headersEnded, decode header block
				if si.headersEnded {
					x, err := decoder.DecodeFull(si.stream.frames[si.headersIndex].body)
					if err != nil {
						log.Fatal(err)
					}
					si.stream.frames[si.headersIndex].headerFields = x
					si.stream.frames[si.headersIndex].body = nil
				}
				// Set/update captureTimestamp
				si.stream.captureTimestamp = timestamp
			} else if hdr.Type == http2.FrameContinuation && !si.headersEnded {
				// Append header block fragment to headers frame
				fg := curFrame.(*http2.ContinuationFrame).HeaderBlockFragment()
				x := append(si.stream.frames[si.headersIndex].body, fg...)
				si.stream.frames[si.headersIndex].body = x
				// Update headersEnded tracking
				si.headersEnded = curFrame.(*http2.ContinuationFrame).HeadersEnded()
				// Set/update captureTimestamp
				si.stream.captureTimestamp = timestamp
				// If headersEnded, decode header block
				if si.headersEnded {
					x, err := decoder.DecodeFull(si.stream.frames[si.headersIndex].body)
					if err != nil {
						log.Fatal(err)
					}
					si.stream.frames[si.headersIndex].headerFields = x
					si.stream.frames[si.headersIndex].body = nil
				}
			}
			streaminfos[sid] = si
		}
	}
	// Create smap with all the streams from streaminfo
	smap := make(map[uint32]stream)
	for k, v := range streaminfos {
		if v.stream.frames != nil {
			smap[k] = v.stream
		}
	}
	return smap
}

func copyHdr(hdr http2.FrameHeader) (copyhdr http2.FrameHeader) {
	copyhdr.Type = hdr.Type
	copyhdr.Flags = hdr.Flags
	copyhdr.Length = hdr.Length
	copyhdr.StreamID = hdr.StreamID
	return
}

func copyBuf(buf []byte) []byte {
	copybuf := make([]byte, len(buf))
	n := copy(copybuf, buf)
	if n != len(buf) {
		log.Fatal("error when copying buffer")
	}
	return copybuf
}
