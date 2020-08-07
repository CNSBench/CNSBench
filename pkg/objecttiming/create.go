/** create.go
 * variables & functions related to object creation parsing
 */

package objecttiming

/* createEndCrit
 * How to identify the last creation log for each supported resource type.
 */
var createEndCrit = jsondict{
	"pods": jsondict{
		"responseObject": jsondict{
			"status": jsondict{
				"phase": "Running",
			},
		},
	},
}

func isCreateStart(log jsondict, all []jsondict) bool {
	// Start counting object creation from successful create request
	if log["verb"] != "create" ||
	int(log["responseStatus"].(jsondict)["code"].(float64)) != 201 {
		return false
	}
	// Make sure resource type is supported
	resource := log["objectRef"].(jsondict)["resource"]
	if resource == nil || createEndCrit[resource.(string)] == nil {
		return false
	}
	return true
}

func isCreateEnd(log jsondict, all []jsondict) bool {
	// Pre-check traits that all end-of-creation logs should have
	if (log["verb"] != "patch" && log["verb"] != "update") ||
	int(log["responseStatus"].(jsondict)["code"].(float64)) != 200 {
		return false
	}
	// Make sure resource type is supported
	resource := log["objectRef"].(jsondict)["resource"]
	if resource == nil || createEndCrit[resource.(string)] == nil {
		return false
	}
	// Check all end-of-creation criteria for the resource type
	return isMatch(log, createEndCrit[resource.(string)].(jsondict))
}

/** isMatch
 * helper for isCreateEnd
 * recursively checks that all values in match are also found in dict
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

func getCreateStart(log jsondict) jsondict {
	return getGenericStart(log, strCreate)
}

func getCreateEndIndex(log jsondict, all []jsondict) int {
	return getEndIndex(strCreate, log, all)
}
