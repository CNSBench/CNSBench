/** objinfo.go
functions for saving & accessing object info from get & create requests.
useful info from objects of relevant resource types is saved in case it's
needed later on.
*/

package objecttiming

type objinfo = jsondict

type objinfostore = map[string]map[string]map[string]objinfo

func isGetCreateRequest(log auditlog) bool {
	if log.Verb == "get" || log.Verb == "create" {
		return true
	} else {
		return false
	}
}

func saveObjInfo(log auditlog, store objinfostore) {
	name, resource, namespace := getIdentification(log)
	if log.ResponseStatus.Code == 201 || log.ResponseStatus.Code == 200 {
		if scaleEndCrit[resource] != nil {
			toAdd := objinfo{
				"parallelism": log.ResponseObject.Spec.Parallelism,
				"replicas":    log.ResponseObject.Spec.Replicas,
				"startTime":   parseStrTime(log.RequestReceivedTimestamp),
			}
			addInfo(toAdd, namespace, resource, name, store)
		}
	}
}

func addInfo(toAdd objinfo, ns, res, name string, store objinfostore) {
	if store[ns] == nil {
		store[ns] = make(map[string](map[string]objinfo))
	}
	if store[ns][res] == nil {
		store[ns][res] = make(map[string]objinfo)
	}
	store[ns][res][name] = toAdd
}

func getObjInfo(log auditlog, store objinfostore) objinfo {
	name, resource, namespace := getIdentification(log)
	return store[namespace][resource][name]
}
