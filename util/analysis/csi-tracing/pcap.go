package main

import (
	"log"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

func parsePackets(fileName string) map[string]rawFrames {
	rf := make(map[string]rawFrames)
	handle, err := pcap.OpenOffline(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		si, di, sp, dp, fr := parsePacket(packet)
		if si == nil {
			continue
		}
		id := srcdstIdentifier(si, di, sp, dp)
		if x, ok := rf[id]; ok {
			x.frames = append(x.frames, fr)
			rf[id] = x
		} else {
			rf[id] = rawFrames{si, di, sp, dp, []frames{fr}}
		}
	}
	return rf
}

func parsePacket(packet gopacket.Packet) (net.IP, net.IP, layers.TCPPort,
	layers.TCPPort, frames) {
	ip := packet.Layer(layers.LayerTypeIPv4)
	tcp := packet.Layer(layers.LayerTypeTCP)
	app := packet.ApplicationLayer()
	if ip == nil || tcp == nil || app == nil {
		return nil, nil, 0, 0, frames{nil, time.Time{}}
	}
	srcIP := ip.(*layers.IPv4).SrcIP
	dstIP := ip.(*layers.IPv4).DstIP
	srcPort := tcp.(*layers.TCP).SrcPort
	dstPort := tcp.(*layers.TCP).DstPort
	fr := frames{app.Payload(), packet.Metadata().CaptureInfo.Timestamp}
	return srcIP, dstIP, srcPort, dstPort, fr
}

func srcdstIdentifier(si, di net.IP, sp, dp layers.TCPPort) string {
	return si.String() + di.String() + sp.String() + dp.String()
}
