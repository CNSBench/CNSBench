/** scale.go
 * variables & functions related to object scaling
 */

package objecttiming

import (
	"encoding/json"
	"reflect"
)

/* scaleEndCrit
 * What fields in an object's status must match the goal replica count for
 * a scaling to be considered 'done'
 */
var scaleEndCrit = map[string]([]string){
	"deployments": []string{
		"AvailableReplicas",
		"ReadyReplicas",
		"UpdatedReplicas",
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

type scaleStatus = struct {
	AvailableReplicas,
	CurrentReplicas,
	FullyLabeledReplicas,
	ReadyReplicas,
	UpdatedReplicas uint8
}

func isScaleStart(log auditlog, objstore objinfostore, all []jsondict) bool {
	// Start counting object scaling from successful change to spec.replicas
	// 2 possible ways: (1) patch object/scale (2) update entire object
	if log.Verb != "update" &&
		(log.Verb != "patch" ||
			log.ObjectRef.Subresource != "scale") {
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
	oldReplicas := prevRequest["replicas"]
	// Get .spec.replicas from the current request
	newReplicas := log.ResponseObject.Spec.Replicas
	// Make sure the new value of .spec.replicas has changed from the old value
	// Also make sure both actually have values
	if oldReplicas == 0 || newReplicas == 0 || oldReplicas == newReplicas {
		return false
	}
	// Make sure the scale isn't already being recorded
	if i := getScaleEndIndex(log, all); i >= 0 {
		record := all[i]
		if record["startReplicas"] == oldReplicas &&
			record["endReplicas"] == newReplicas {
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
	goalReplicas := all[i]["endReplicas"].(uint8)
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
	record["startReplicas"] = prevReq["replicas"]
	record["endReplicas"] = log.ResponseObject.Spec.Replicas
	return record
}

func getScaleEndIndex(log auditlog, all []jsondict) int {
	return getEndIndex(strScale, log, all)
}
