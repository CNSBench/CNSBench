package csiparser

import (
	"log"

	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/runtime/protoiface"
)

func parseCSIRequest(frame []byte, pathService, pathRPC string) protoiface.MessageV1 {
	gRPCMsg := parseGRPCMessage(frame)
	reqStruct := csiRequestStruct(pathService, pathRPC)
	if reqStruct == nil {
		return nil
	}
	parseCSI(gRPCMsg.message, reqStruct)
	return reqStruct
}

func parseCSIResponse(frame []byte, pathService, pathRPC string) protoiface.MessageV1 {
	gRPCMsg := parseGRPCMessage(frame)
	repStruct := csiResponseStruct(pathService, pathRPC)
	if repStruct == nil {
		return nil
	}
	parseCSI(gRPCMsg.message, repStruct)
	return repStruct

}

// Parses csiFrame into *csiStruct
func parseCSI(csiFrame []byte, csiStruct protoiface.MessageV1) {
	if err := proto.Unmarshal(csiFrame, csiStruct); err != nil {
		log.Fatalln("Failed to parse request:", err)
	}
}

type gRPCMessage struct {
	compressedFlag 	uint8	// first byte: 0 for not compressed, 1 for compressed
	messageLength	uint32	// next 4 bytes: length of RPC message (CSI request/response)
	message			[]byte	// Message
}

func parseGRPCMessage(frame []byte) (gRPCMessage) {
	// Store frame as gRPC message struct
	msgLength := uint32(frame[1])
	msgLength = msgLength << 8 | uint32(frame[2])
	msgLength = msgLength << 8 | uint32(frame[3])
	msgLength = msgLength << 8 | uint32(frame[4])
	msg := gRPCMessage{uint8(frame[0]), msgLength, frame[5:]}
	// For now, give up if msg is compressed
	if msg.compressedFlag > 0 {
		log.Fatalln("Message is compressed. Not currently supported.")
	}
	// Sanity check
	if msg.messageLength != uint32(len(msg.message)) {
		log.Fatalln("Message length mismatch.")
	}
	return msg
}

func csiRequestStruct(service, rpc string) protoiface.MessageV1 {
	switch service {
	case "Identity":
		switch rpc {
		case "GetPluginInfo":
			return &GetPluginInfoRequest{}
		case "GetPluginCapabilities":
			return &GetPluginCapabilitiesRequest{}
		case "Probe":
			return &ProbeRequest{}
		default:
			return nil
		}
	case "Controller":
		switch rpc {
		case "CreateVolume":
			return &CreateVolumeRequest{}
		case "DeleteVolume":
			return &DeleteVolumeRequest{}
		case "ControllerPublishVolume":
			return &ControllerPublishVolumeRequest{}
		case "ControllerUnpublishVolume":
			return &ControllerUnpublishVolumeRequest{}
		case "ValidateVolumeCapabilities":
			return &ValidateVolumeCapabilitiesRequest{}
		case "ListVolumes":
			return &ListVolumesRequest{}
		case "GetCapacity":
			return &GetCapacityRequest{}
		case "ControllerGetCapabilities":
			return &ControllerGetCapabilitiesRequest{}
		case "CreateSnapshot":
			return &CreateSnapshotRequest{}
		case "DeleteSnapshot":
			return &DeleteSnapshotRequest{}
		case "ListSnapshots":
			return &ListSnapshotsRequest{}
		case "ControllerExpandVolume":
			return &ControllerExpandVolumeRequest{}
		case "ControllerGetVolume":
			return &ControllerGetVolumeRequest{}
		default:
			return nil
		}
	case "Node":
		switch rpc {
		case "NodeStageVolume":
			return &NodeStageVolumeRequest{}
		case "NodeUnstageVolume":
			return &NodeUnstageVolumeRequest{}
		case "NodePublishVolume":
			return &NodePublishVolumeRequest{}
		case "NodeUnpublishVolume":
			return &NodeUnpublishVolumeRequest{}
		case "NodeGetVolumeStats":
			return &NodeGetVolumeStatsRequest{}
		case "NodeExpandVolume":
			return &NodeExpandVolumeRequest{}
		case "NodeGetCapabilities":
			return &NodeGetCapabilitiesRequest{}
		case "NodeGetInfo":
			return &NodeGetInfoRequest{}
		default:
			return nil
		}
	default:
		return nil
	}
}

func csiResponseStruct(service, rpc string) protoiface.MessageV1 {
	switch service {
	case "Identity":
		switch rpc {
		case "GetPluginInfo":
			return &GetPluginInfoResponse{}
		case "GetPluginCapabilities":
			return &GetPluginCapabilitiesResponse{}
		case "Probe":
			return &ProbeResponse{}
		default:
			return nil
		}
	case "Controller":
		switch rpc {
		case "CreateVolume":
			return &CreateVolumeResponse{}
		case "DeleteVolume":
			return &DeleteVolumeResponse{}
		case "ControllerPublishVolume":
			return &ControllerPublishVolumeResponse{}
		case "ControllerUnpublishVolume":
			return &ControllerUnpublishVolumeResponse{}
		case "ValidateVolumeCapabilities":
			return &ValidateVolumeCapabilitiesResponse{}
		case "ListVolumes":
			return &ListVolumesResponse{}
		case "GetCapacity":
			return &GetCapacityResponse{}
		case "ControllerGetCapabilities":
			return &ControllerGetCapabilitiesResponse{}
		case "CreateSnapshot":
			return &CreateSnapshotResponse{}
		case "DeleteSnapshot":
			return &DeleteSnapshotResponse{}
		case "ListSnapshots":
			return &ListSnapshotsResponse{}
		case "ControllerExpandVolume":
			return &ControllerExpandVolumeResponse{}
		case "ControllerGetVolume":
			return &ControllerGetVolumeResponse{}
		default:
			return nil
		}
	case "Node":
		switch rpc {
		case "NodeStageVolume":
			return &NodeStageVolumeResponse{}
		case "NodeUnstageVolume":
			return &NodeUnstageVolumeResponse{}
		case "NodePublishVolume":
			return &NodePublishVolumeResponse{}
		case "NodeUnpublishVolume":
			return &NodeUnpublishVolumeResponse{}
		case "NodeGetVolumeStats":
			return &NodeGetVolumeStatsResponse{}
		case "NodeExpandVolume":
			return &NodeExpandVolumeResponse{}
		case "NodeGetCapabilities":
			return &NodeGetCapabilitiesResponse{}
		case "NodeGetInfo":
			return &NodeGetInfoResponse{}
		default:
			return nil
		}
	default:
		return nil
	}
}
