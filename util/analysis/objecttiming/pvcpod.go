/* create.go
variables & functions related to Pod/PVC parsing
*/

package main

import "encoding/json"

/* pvcPodStatus
Used as the struct for unmarshaling .requestObject.status
*/
type pvcPodStatus = struct {
	ContainerStatuses []struct {
		State struct{ Waiting struct{ Reason string } }
	}
}

func isAttachVolumeEvent(log auditlog) bool {
	if log.ObjectRef.Resource == "events" &&
		log.RequestObject.InvolvedObject.Kind == "Pod" &&
		log.RequestObject.Reason == "SuccessfulAttachVolume" {
		return true
	} else {
		return false
	}
}

func setAttachTime(log auditlog, ongoing []jsondict) {
	// set attachTime for appropriate Pod
	name := log.RequestObject.InvolvedObject.Name
	namespace := log.RequestObject.InvolvedObject.Namespace
	index := findLog("create", name, "pods", namespace, ongoing)
	if index == -1 {
		return
	}
	ongoing[index]["attachTime"] = parseStrTime(log.StageTimestamp)
	// also store name of the PVC
	msg := log.RequestObject.Message
	pvName := ""
	start, end := -1, -1
	for i, c := range msg {
		if c == '"' {
			if start == -1 {
				start = i + 1
			} else {
				end = i
				break
			}
		}
	}
	if start != -1 && end != -1 {
		pvName = msg[start:end]
	}
	ongoing[index]["pvName"] = pvName
}

func isContainerCreatingStatus(log auditlog) bool {
	if log.Verb != "patch" ||
		log.ObjectRef.Resource != "pods" ||
		log.ObjectRef.Subresource != "status" {
		return false
	}
	var jsonStatus pvcPodStatus
	status := log.RequestObject.Status
	if err := json.Unmarshal(status, &jsonStatus); err != nil {
		panic(err)
	}
	if len(jsonStatus.ContainerStatuses) == 0 {
		return false
	}
	retval := true
	for _, status := range jsonStatus.ContainerStatuses {
		if status.State.Waiting.Reason != "ContainerCreating" {
			retval = false
			break
		}
	}
	return retval
}

func setContainerTime(log auditlog, ongoing []jsondict) {
	// set containerTime for appropriate Pod
	name := log.ObjectRef.Name
	namespace := log.ObjectRef.Namespace
	index := findLog("create", name, "pods", namespace, ongoing)
	if index == -1 {
		return
	}
	ongoing[index]["containerTime"] = parseStrTime(log.StageTimestamp)
}

func boundTimes(results []jsondict) {
	for _, podrec := range results {
		if pvName, ok := podrec["pvName"]; ok && podrec["resource"] == "pods" {
			for _, pvcrec := range results {
				if pvcrec["pvName"] == pvName &&
					pvcrec["resource"] == "persistentvolumeclaims" {
					podrec["boundTime"] = pvcrec["endTime"]
					delete(podrec, "pvName")
					delete(pvcrec, "pvName")
				}
			}
		}
	}
}
