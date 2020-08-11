/* create.go
variables & functions related to object creation parsing
*/

package objecttiming

import "encoding/json"

/* createEndCrit
How to identify the last creation log for each supported resource type
by the contents of the .responseObject.status field
*/
var createEndCrit = jsondict{
	"pods": jsondict{
		"phase": "Running",
	},
}

func isCreateStart(log auditlog, all []jsondict) bool {
	// Start counting object creation from successful create request
	if log.Verb != "create" || log.ResponseStatus.Code != 201 {
		return false
	}
	// Make sure resource type is supported
	if createEndCrit[log.ObjectRef.Resource] == nil {
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
	if createEndCrit[resource] == nil {
		return -1
	}
	// Check all end-of-creation criteria for the resource type
	if status := log.ResponseObject.Status; status != nil {
		var jsonStatus jsondict
		if err := json.Unmarshal(status, &jsonStatus); err != nil {
			panic(err)
		}
		if !isMatch(jsonStatus, createEndCrit[resource].(jsondict)) {
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
recursively checks that all values in match are also found in dict
*/
func isMatch(dict, match jsondict) bool {
	for key, val := range match {
		subdict := dict[key]
		if subdict == nil {
			return false
		}
		if submatch, ok := val.(jsondict); ok {
			if !isMatch(subdict.(jsondict), submatch) {
				return false
			}
		} else if str, ok := val.(string); ok {
			if subdict.(string) != str {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

func getCreateStart(log auditlog) jsondict {
	return getGenericStart(log, strCreate)
}

func getCreateEndIndex(log auditlog, all []jsondict) int {
	return getEndIndex(strCreate, log, all)
}
