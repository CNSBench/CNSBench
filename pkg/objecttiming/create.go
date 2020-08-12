/* create.go
variables & functions related to object creation parsing
*/

package objecttiming

import "encoding/json"

/* createStatus
Relevant fields from the status of an object being created.
Used as the struct for unmarshaling .responseObject.status
*/
type createStatus = struct {
	Phase string
}

/* createEndCrit
How to identify the last creation log for each supported resource type
by the contents of the .responseObject.status field
*/
var createEndCrit = map[string]createStatus{
	"pods":                   createStatus{"Running"},
	"persistentvolumes":      createStatus{"Bound"},
	"persistentvolumeclaims": createStatus{"Bound"},
}

func isCreateStart(log auditlog, all []jsondict) bool {
	// Start counting object creation from successful create request
	if log.Verb != "create" || log.ResponseStatus.Code != 201 {
		return false
	}
	// Make sure resource type is supported
	if _, found := createEndCrit[log.ObjectRef.Resource]; !found {
		return false
	}
	return true
}

func isCreateEnd(log auditlog, all []jsondict) int {
	// Pre-check traits that all end-of-creation logs should have
	if (log.Verb != "patch" && log.Verb != "update") ||
		log.ResponseStatus.Code != 200 {
		return -1
	}
	// Make sure resource type is supported
	resource := log.ObjectRef.Resource
	if _, found := createEndCrit[resource]; !found {
		return -1
	}
	// Check all end-of-creation criteria for the resource type
	if status := log.ResponseObject.Status; status != nil {
		var jsonStatus createStatus
		if err := json.Unmarshal(status, &jsonStatus); err != nil {
			panic(err)
		}
		if !isMatch(jsonStatus, createEndCrit[resource]) {
			return -1
		}
	} else {
		return -1
	}
	// Make sure there's a corresponding create in the records
	return getCreateEndIndex(log, all)
}

/* isMatch
helper for isCreateEnd
checks if s1 and s2 have the same values
*/
func isMatch(s1, s2 createStatus) bool {
	if s1.Phase != s2.Phase {
		return false
	}
	return true
}

func getCreateStart(log auditlog) jsondict {
	return getGenericStart(log, strCreate)
}

func getCreateEndIndex(log auditlog, all []jsondict) int {
	return getEndIndex(strCreate, log, all)
}
