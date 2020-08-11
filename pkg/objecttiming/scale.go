/* scale.go
variables & functions related to object scaling
*/

package objecttiming

import (
	"encoding/json"
	"reflect"
)

/* scaleEndCrit
What fields in an object's status must match the goal replica count for
a scaling to be considered 'done'
*/
var scaleEndCrit = map[string]([]string){
	"deployments": []string{
		"AvailableReplicas",
		"ReadyReplicas",
		"UpdatedReplicas",
	},
	"jobs": []string{
		"Active",
	},
	"replicasets": []string{
		"AvailableReplicas",
		"FullyLabeledReplicas",
		"ReadyReplicas",
	},
	"statefulsets": []string{
		"CurrentReplicas",
		"ReadyReplicas",
	},
}

/* scaleStatus
Struct with all possible relevant status fields for scaling.
Used as the struct for unmarshaling the .responseObject.status field.
*/
type scaleStatus = struct {
	AvailableReplicas,
	CurrentReplicas,
	FullyLabeledReplicas,
	ReadyReplicas,
	UpdatedReplicas uint8
	Active uint8
}

var replicaLabels = map[string]string{
	"spec":  "replicas",
	"start": "startReplicas",
	"end":   "endReplicas",
}

/* scaleLabels
Field labels by category (spec, start, end) for each resource type.
spec: replicas/parallelism field in the object spec
start: name for the start replicas/parallelism field in the results json
end: name for the end replicas/parallelism field in the results json
*/
var scaleLabels = map[string]map[string]string{
	"deployments": replicaLabels,
	"jobs": map[string]string{
		"spec":  "parallelism",
		"start": "startParallelism",
		"end":   "endParallelism",
	},
	"replicasets":  replicaLabels,
	"statefulsets": replicaLabels,
}

/* getSpecCrit
Returns the appropriate field (Parallelism or Replicas) from the spec struct
of an object, depending on the resource type.
*/
func getSpecCrit(resource string, s spec) uint8 {
	if resource == "jobs" {
		return s.Parallelism
	} else {
		return s.Replicas
	}
}

func isScaleStart(log auditlog, objstore objinfostore, all []jsondict) bool {
	// Start counting object scaling from successful change to spec.replicas
	// 2 possible ways: (1) patch object/scale (2) update entire object
	if log.Verb != "update" && log.Verb != "patch" {
		return false
	}
	if log.ResponseStatus.Code != 200 {
		return false
	}
	// Make sure resource type is supported
	resource := log.ObjectRef.Resource
	if scaleEndCrit[resource] == nil {
		return false
	}
	// Get .spec.replicas from a previous get request for the object
	prevRequest := getObjInfo(log, objstore)
	if prevRequest == nil {
		return false
	}
	oldReplicas := prevRequest[scaleLabels[resource]["spec"]].(uint8)
	// Get .spec.replicas from the current request
	newReplicas := getSpecCrit(resource, log.ResponseObject.Spec)
	// Make sure the new value of .spec.replicas has changed from the old value
	// Also make sure both actually have values
	if oldReplicas == 0 || newReplicas == 0 || oldReplicas == newReplicas {
		return false
	}
	// Make sure the scale isn't already being recorded
	if i := getScaleEndIndex(log, all); i >= 0 {
		record := all[i]
		if record[scaleLabels[resource]["start"]] == oldReplicas &&
			record[scaleLabels[resource]["end"]] == newReplicas {
			return false
		}
	}
	return true
}

func isScaleEnd(log auditlog, all []jsondict) int {
	// Pre-check traits that all end-of-scale logs should have in common
	if log.Verb != "update" || log.ObjectRef.Subresource != "status" ||
		log.ResponseStatus.Code != 200 {
		return -1
	}
	// Make sure the resource type is supported
	resource := log.ObjectRef.Resource
	if scaleEndCrit[resource] == nil {
		return -1
	}
	// Get the fields in the object's status that must match the goal replicas
	criteria := scaleEndCrit[resource]
	// Unmarshal the request object's status
	reqStat := log.RequestObject.Status
	if reqStat == nil {
		return -1
	}
	var requestStatus scaleStatus
	if err := json.Unmarshal(reqStat, &requestStatus); err != nil {
		panic(err)
	}
	// Make sure the request was made to update one of the required fields
	replicaChangeMade := false
	for _, crit := range criteria {
		val := reflect.ValueOf(&requestStatus).Elem().FieldByName(crit).Uint()
		if val != 0 {
			replicaChangeMade = true
		}
	}
	if !replicaChangeMade {
		return -1
	}
	// Find a record that matches the information in the request
	i := getScaleEndIndex(log, all)
	if i < 0 {
		return -1
	}
	// Retrieve the goal replicas from the matching record
	goalReplicas := all[i][scaleLabels[resource]["end"]].(uint8)
	// Unmarshal the response object's status
	respStat := log.ResponseObject.Status
	if respStat == nil {
		return -1
	}
	var responseStatus scaleStatus
	if err := json.Unmarshal(respStat, &responseStatus); err != nil {
		panic(err)
	}
	// Make sure all required fields in object's status match the goal replicas
	for _, crit := range criteria {
		val := reflect.ValueOf(&responseStatus).Elem().FieldByName(crit).Uint()
		if uint8(val) != goalReplicas {
			return -1
		}
	}
	return i
}

func getScaleStart(log auditlog, objstore objinfostore) jsondict {
	// Set up all standard information in the record
	record := getGenericStart(log, strScale)
	// Additionally, set startReplicas & endReplicas
	prevReq := getObjInfo(log, objstore)
	resource := log.ObjectRef.Resource
	prevSpecVal := prevReq[scaleLabels[resource]["spec"]]
	record[scaleLabels[resource]["start"]] = prevSpecVal
	newSpecVal := getSpecCrit(resource, log.ResponseObject.Spec)
	record[scaleLabels[resource]["end"]] = newSpecVal
	return record
}

func getScaleEndIndex(log auditlog, all []jsondict) int {
	return getEndIndex(strScale, log, all)
}
