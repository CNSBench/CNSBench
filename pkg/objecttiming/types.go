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
	RequestObject  struct{ Status json.RawMessage }
	ResponseObject respobject
}

type objref = struct {
	Resource,
	Namespace,
	Name,
	Subresource string
}

type respobject = struct {
	Metadata struct {
		Name   string
		Labels map[string]string
	}
	Spec   struct{ Replicas uint8 }
	Status json.RawMessage
}
