/** helpers.go
 * Helper functions needed by parsing subroutines of all actions
 */

package objecttiming

import "time"

/** initializeRecords
 * Creates & initializes a new record with the given values
 */
func initializeRecord(op, nm, rs, ns string, tm time.Time) jsondict {
	record := make(jsondict)
	record["action"] = op
	record["name"] = nm
	record["resource"] = rs
	record["namespace"] = ns
	record["startTime"] = tm
	return record
}

/** getGenericStart
 * Returns a new dictionary containing record info that all actions share
 */
func getGenericStart(log jsondict, action string) jsondict {
	// Get name, resource, namespace of object
	name, resource, namespace := getIdentification(log)
	// Get start time of the object
	time := getStartTime(log)
	// Create & initialize initial record
	record := initializeRecord(action, name, resource, namespace, time)
	return record
}

/** objectMatch
 * Returns whether or not name, resource, namespace in obj match the given
 * name, resource, and namespace
 */
func objectMatch(obj jsondict, name, resource, namespace string) bool {
	if obj["name"] == name &&
	obj["resource"] == resource &&
	obj["namespace"] == namespace {
		return true
	}
	return false
}

/** getName
 * Returns the name of the object in the given log
 */
func getName(log jsondict) (name string) {
	if n := log["objectRef"].(jsondict)["name"]; n != nil {
		// Can usually get the name of an object from objectRef field
		name = n.(string)
	} else {
		// Object creation with name generation (usually for managed objects)
		responseObject := log["responseObject"].(jsondict)
		name = responseObject["metadata"].(jsondict)["name"].(string)
	}
	return
}

/** getResource
 * Returns the resource type of the object in the given log
 */
func getResource(log jsondict) string {
	return log["objectRef"].(jsondict)["resource"].(string)
}

/** getNamespace
 * Returns the namespace of the object in the given log
 */
func getNamespace(log jsondict) (namespace string) {
	if n := log["objectRef"].(jsondict)["namespace"]; n != nil {
		namespace = n.(string)
	} else {
		namespace = ""
	}
	return
}

/** getIdentification
 * Returns the name, resource type, and namespace of the given log,
 * which are the three fields by which an object can be uniquely ID-ed
 */
func getIdentification(log jsondict) (string, string, string) {
	return getName(log), getResource(log), getNamespace(log)
}

/** getEndIndex
 * Searches the given array (all) for a record that matches the given log's ID
 * and the given action type. Returns the index in the array of said record.
 */
func getEndIndex(action string, log jsondict, all []jsondict) int {
	// Get the name, resource, namespace of log
	name, resource, namespace := getIdentification(log)
	// Search array for a record that matches the name, resource, namespace
	for i := len(all)-1; i>=0; i-- {
		if all[i]["action"] == action &&
		objectMatch(all[i], name, resource, namespace) {
			// Only return the index if duration hasn't been set yet
			if all[i]["duration"] == nil {
				return i
			} else {
				return -1
			}
		}
	}
	return -1
}

/** setEndTime
 * Calculates and records the duration of the action stored in record.
 * Also records any labels that the object may have.
 */
func setEndTime(log jsondict, record jsondict) {
	// Calculate duration
	startTime := record["startTime"].(time.Time)
	endTime := getEndTime(log)
	duration := timeDiff(startTime, endTime)
	// Delete startTime from the record
	delete(record, "startTime")
	// Set duration in the record
	record["duration"] = duration
	// Also record object labels (if there are any) at this point
	metadata := log["responseObject"].(jsondict)["metadata"].(jsondict)
	if metadata["labels"] != nil {
		record["labels"] = metadata["labels"]
	}
}
