/* scale.go
variables & functions related to object scaling
*/

package main

import (
	"encoding/json"
)

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

/* scaleEndCrit
What fields in an object's status must match the goal replica count for
a scaling to be considered 'done'
*/
var scaleEndCrit = map[string]scaleStatus{
	"deployments":  scaleStatus{1, 0, 0, 1, 1, 0},
	"jobs":         scaleStatus{0, 0, 0, 0, 0, 1},
	"replicasets":  scaleStatus{1, 0, 1, 1, 0, 0},
	"statefulsets": scaleStatus{0, 1, 0, 1, 0, 0},
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
	if _, found := scaleEndCrit[resource]; !found {
		return false
	}
	// Get .spec.replicas from a previous get request for the object
	prevRequest := getObjInfo(log, objstore)
	if prevRequest == nil {
		return false
	}
	oldReplicas := prevRequest[specLabel(resource)].(uint8)
	// Get .spec.replicas from the current request
	newReplicas := getGoalValue(resource, log.ResponseObject.Spec)
	// Make sure the new value of .spec.replicas has changed from the old value
	// Also make sure both actually have values
	if oldReplicas == 0 || newReplicas == 0 || oldReplicas == newReplicas {
		return false
	}
	// Make sure the scale isn't already being recorded
	if i := getScaleEndIndex(log, all); i >= 0 {
		record := all[i]
		if record[startLabel(resource)] == oldReplicas &&
			record[endLabel(resource)] == newReplicas {
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
	// Get the fields in the object's status that must match the goal replicas
	// & make sure the resource type is supported
	resource := log.ObjectRef.Resource
	criteria, found := scaleEndCrit[resource]
	if !found {
		return -1
	}
	// Unmarshal the request object's status
	reqStat := log.RequestObject.Status
	if reqStat == nil {
		return -1
	}
	var requestStatus scaleStatus
	if err := json.Unmarshal(reqStat, &requestStatus); err != nil {
		panic(err)
	}
	// Find a record that matches the information in the request
	i := getScaleEndIndex(log, all)
	if i < 0 {
		return -1
	}
	// Retrieve the goal replicas from the matching record
	goalReplicas := all[i][endLabel(resource)].(uint8)
	// Unmarshal the response object's status
	respStat := log.ResponseObject.Status
	if respStat == nil {
		return -1
	}
	var responseStatus scaleStatus
	if err := json.Unmarshal(respStat, &responseStatus); err != nil {
		panic(err)
	}
	if !allGoalsFulfilled(responseStatus, criteria, goalReplicas) {
		return -1
	}
	return i
}

func getScaleStart(log auditlog, objstore objinfostore) jsondict {
	// Set up all standard information in the record
	record := getGenericStart(log, strScale)
	// Additionally, set startReplicas & endReplicas
	prevReq := getObjInfo(log, objstore)
	resource := log.ObjectRef.Resource
	prevSpecVal := prevReq[specLabel(resource)]
	record[startLabel(resource)] = prevSpecVal
	newSpecVal := getGoalValue(resource, log.ResponseObject.Spec)
	record[endLabel(resource)] = newSpecVal
	return record
}

func getScaleEndIndex(log auditlog, all []jsondict) int {
	return getEndIndex(strScale, log, all)
}

func allGoalsFulfilled(stat, crit scaleStatus, goal uint8) bool {
	rf := func(s, c uint8) bool {
		if c == 0 {
			return true
		} else {
			return s == goal
		}
	}
	return rf(stat.AvailableReplicas, crit.AvailableReplicas) &&
		rf(stat.CurrentReplicas, crit.CurrentReplicas) &&
		rf(stat.FullyLabeledReplicas, crit.FullyLabeledReplicas) &&
		rf(stat.ReadyReplicas, crit.ReadyReplicas) &&
		rf(stat.UpdatedReplicas, crit.UpdatedReplicas) &&
		rf(stat.Active, crit.Active)
}

/* getGoalValue
Returns the appropriate field (Parallelism or Replicas) from the spec struct
of an object, depending on the resource type.
*/
func getGoalValue(resource string, s spec) uint8 {
	if resource == "jobs" {
		return s.Parallelism
	} else {
		return s.Replicas
	}
}

func specLabel(resource string) string {
	if resource == "jobs" {
		return "parallelism"
	} else {
		return "replicas"
	}
}

func startLabel(resource string) string {
	if resource == "jobs" {
		return "startParallelism"
	} else {
		return "startReplicas"
	}
}

func endLabel(resource string) string {
	if resource == "jobs" {
		return "endParallelism"
	} else {
		return "endReplicas"
	}
}
