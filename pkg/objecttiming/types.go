package objecttiming

import "encoding/json"

/* Structures to store parsing & calculation data */

type jsondict = map[string]interface{}

/* Structures to store audit log input */

type auditlog = struct {
	Stage,
	Verb,
	StageTimestamp,
	RequestReceivedTimestamp string
	ObjectRef      objref
	ResponseStatus struct{ Code int16 }
	RequestObject  reqobject
	ResponseObject respobject
}

type objref = struct {
	Resource,
	Namespace,
	Name,
	Subresource string
}

type reqobject = struct {
	Status          json.RawMessage
	InvolvedObject  involvedobject
	Reason, Message string
}

type involvedobject = struct {
	Kind,
	Namespace,
	Name string
}

type respobject = struct {
	Metadata struct {
		Name   string
		Labels map[string]string
	}
	Spec   spec
	Status json.RawMessage
}

type spec = struct {
	Replicas    uint8
	Parallelism uint8
	VolumeName  string
}
