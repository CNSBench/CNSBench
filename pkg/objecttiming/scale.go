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
	UpdatedReplicas	uint8
}

func isScaleStart(log auditlog, all []jsondict) bool {
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
	// Get the values of .spec.replicas and .status.replicas of the object
	responseObject := log.ResponseObject
	if responseObject.Spec == nil || responseObject.Status == nil {
		return false
	}
	var respSpec, respStatus struct { Replicas int }
	if err := json.Unmarshal(responseObject.Spec, &respSpec); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(responseObject.Status, &respStatus); err != nil {
		panic(err)
	}
	oldReplicas := respStatus.Replicas
	newReplicas := respSpec.Replicas
	// Check if .spec.replicas has changed from .status.replicas
	// Make sure .status.replicas didn't start at 0, which happens at creation
	// Also make sure .status.replicas and .spec.replicas have values
	if oldReplicas == 0 || newReplicas == 0 || newReplicas == oldReplicas {
		return false
	}
	// Make sure the unfinished scale action isn't already being tracked
	if getScaleEndIndex(log, all) >= 0 {
		return false
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

func getScaleStart(log auditlog) jsondict {
	// Set up all standard information in the record
	record := getGenericStart(log, strScale)
	// Additionally, set startReplicas & endReplicas
	respObject := log.ResponseObject
	var responseSpec, responseStatus struct { Replicas uint8 }
	if err := json.Unmarshal(respObject.Spec, &responseSpec); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(respObject.Status, &responseStatus); err != nil {
		panic(err)
	}
	record["startReplicas"] = responseStatus.Replicas
	record["endReplicas"] = responseSpec.Replicas
	return record
}

func getScaleEndIndex(log auditlog, all []jsondict) int {
	return getEndIndex(strScale, log, all)
}
