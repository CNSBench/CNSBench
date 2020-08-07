/** scale.go
 * variables & functions related to object scaling
 */

package objecttiming

/* scaleEndCrit
 * What fields in an object's status must match the goal replica count for
 * a scaling to be considered 'done'
 */
var scaleEndCrit = map[string]([]string){
    "deployments": []string{
        "availableReplicas",
        "readyReplicas",
        "updatedReplicas",
    },
    "replicasets": []string{
        "availableReplicas",
        "fullyLabeledReplicas",
        "readyReplicas",
    },
    "statefulsets": []string{
        "currentReplicas",
        "currentRevision",
        "readyReplicas",
        "updatedRevision",
    },
}

func isScaleStart(log jsondict, all []jsondict) bool {
	// Start counting object scaling from successful change to spec.replicas
	// 2 possible ways: (1) patch object/scale (2) update entire object
	if log["verb"] != "update" &&
	(log["verb"] != "patch" || 
	log["objectRef"].(jsondict)["subresource"] != "scale") {
		return false
	}
	if int(log["responseStatus"].(jsondict)["code"].(float64)) != 200 {
		return false
	}
	// Make sure resource type is supported
	resource := log["objectRef"].(jsondict)["resource"]
	if (resource == nil || scaleEndCrit[resource.(string)] == nil) {
		return false
	}
	// Get the values of .spec.replicas and .status.replicas of the object
	responseObject := log["responseObject"].(jsondict)
	if _, x := responseObject["spec"].(jsondict); x == false {
		return false
	}
	if _, x := responseObject["status"].(jsondict); x == false {
		return false
	}
	newReplicas := responseObject["spec"].(jsondict)["replicas"]
	oldReplicas := responseObject["status"].(jsondict)["replicas"]
	// Check if .spec.replicas has changed from .status.replicas
	// Make sure .status.replicas didn't start at 0, which happens at creation
	if newReplicas == nil || oldReplicas == nil || 
	int(oldReplicas.(float64)) == 0 ||
	newReplicas.(float64) == oldReplicas.(float64) {
		return false
	}
	// Make sure the unfinished scale action isn't already being tracked
	if getScaleEndIndex(log, all) >= 0 {
		return false
	}
	return true
}

func isScaleEnd(log jsondict, all []jsondict) int {
	// Pre-check traits that all end-of-scale logs should have in common
	if log["verb"] != "update" ||
	log["objectRef"].(jsondict)["subresource"] != "status" ||
	int(log["responseStatus"].(jsondict)["code"].(float64)) != 200 {
		return -1
	}
	// Make sure the resource type is supported
	resource := log["objectRef"].(jsondict)["resource"]
	if (resource == nil || scaleEndCrit[resource.(string)] == nil) {
		return -1
	}
	// Get the fields in the object's status that must match the goal replicas
	criteria := scaleEndCrit[resource.(string)]
	// Make sure the request was made to update one of the required fields
	requestStatus := log["requestObject"].(jsondict)["status"].(jsondict)
	replicaChangeMade := false
	for _, crit := range criteria {
		if requestStatus[crit] != nil {
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
	// Make sure all required fields in object's status match the goal replicas
	responseStatus := log["responseObject"].(jsondict)["status"].(jsondict)
	for _, crit := range criteria {
		if int(responseStatus[crit].(float64)) != int(goalReplicas) {
			return -1
		}
	}
	return i
}

func getScaleStart(log jsondict) jsondict {
	// Set up all standard information in the record
	record := getGenericStart(log, strScale)
	// Additionally, set startReplicas & endReplicas
	responseObject := log["responseObject"].(jsondict)
	newReplicas := responseObject["spec"].(jsondict)["replicas"].(float64)
	oldReplicas := responseObject["status"].(jsondict)["replicas"].(float64)
	record["startReplicas"] = uint8(oldReplicas)
	record["endReplicas"] = uint8(newReplicas)
	return record
}

func getScaleEndIndex(log jsondict, all []jsondict) int {
	return getEndIndex(strScale, log, all)
}
