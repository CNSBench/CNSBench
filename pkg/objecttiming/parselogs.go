/* parselogs.go
Contains code for reading an audit file and extracting object
creation information.
Assumes the audit file consists of single-line json objects.
*/

package objecttiming

import (
	"encoding/json"
	"time"
)

func ParseLogs(logs []string, flags uint8) ([]jsondict, error) {
	// Initialize empty slices of actions, represented by dictionaries
	var ongoing []jsondict // temporary storage for actions still in progress
	var results []jsondict // final slice of finished actions
	// If no flags are set, return without doing anything
	if flags == 0 {
		return results, nil
	}
	// Initialize an objinfostore
	objstore := make(objinfostore)
	// Create a scanner to wrap the reader. Split by lines (default)
	// For each line/log:
	for _, line := range logs {
		// Unmarshal the log string into a json dictionary
		var log auditlog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			return results, err
		}
		// Ignore the log if it's not in the ResponseComplete stage
		if log.Stage != "ResponseComplete" {
			continue
		}
		// Create action parsing
		if flags&ParseCreate != 0 {
			if isCreateStart(log, ongoing) {
				record := getCreateStart(log)
				ongoing = append(ongoing, record)
				continue
			} else if i := isCreateEnd(log, ongoing); i >= 0 {
				record := ongoing[i]
				setEndTime(log, record)
				if log.ObjectRef.Resource == "persistentvolumeclaims" {
					record["pvName"] = log.ResponseObject.Spec.VolumeName
				}
				ongoing = append(ongoing[:i], ongoing[i+1:]...)
				results = append(results, record)
				continue
			}
		}
		// Scale action parsing
		if flags&ParseScale != 0 {
			if isScaleStart(log, objstore, ongoing) {
				// If there's an unfinished scale for the same object,
				// force end tracking it
				if i := getScaleEndIndex(log, ongoing); i >= 0 {
					record := ongoing[i]
					setEndTime(log, record)
					record["unfinished"] = true
					ongoing = append(ongoing[:i], ongoing[i+1:]...)
					results = append(results, record)
				}
				// Record the new scale for the object
				record := getScaleStart(log, objstore)
				ongoing = append(ongoing, record)
				continue
			} else if i := isScaleEnd(log, ongoing); i >= 0 {
				record := ongoing[i]
				setEndTime(log, record)
				ongoing = append(ongoing[:i], ongoing[i+1:]...)
				results = append(results, record)
				continue
			}
		}
		// Pod/PVC additional timestamps
		if flags&ParsePVCPod != 0 {
			// Time PVC is attached to Pod (attachTime)
			if isAttachVolumeEvent(log) {
				setAttachTime(log, ongoing)
				continue
			}
			// Time Pod status changed to ContainerCreating (containerTime)
			if isContainerCreatingStatus(log) {
				setContainerTime(log, ongoing)
				continue
			}
		}
		// Save relevant object info, if applicable
		if isGetCreateRequest(log) {
			saveObjInfo(log, objstore)
		}
	}
	// For each PVC-bound Pod, boundTime = the matching PVC create log's endTime
	if flags&ParsePVCPod != 0 {
		boundTimes(results)
	}
	// Calculate duration & other deltas
	calculateDeltas(results, flags)
	// Convert resource names to capitalized names
	formatResourceNames(results)
	return results, nil
}

/* setEndTime
Records the end time of the action stored in record.
Also records any labels that the object may have.
*/
func setEndTime(log auditlog, record jsondict) {
	endTime := getEndTime(log)
	record["endTime"] = endTime
	// Also record object labels (if there are any) at this point
	metadata := log.ResponseObject.Metadata
	if metadata.Labels != nil {
		record["labels"] = metadata.Labels
	}
}

/* calculateDeltas
Calculates duration of each log (endTime - startTime).
For Pods with PVCs, if ParsePVCPod flag is set, calculates relevant deltas.
*/
func calculateDeltas(results []jsondict, flags uint8) {
	for _, result := range results {
		startTime := result["startTime"].(time.Time)
		endTime := result["endTime"].(time.Time)
		// Calculate intermediate deltas for Pod/PVC timings
		if flags&ParsePVCPod != 0 {
			if at, ok := result["attachTime"]; ok {
				attachTime := at.(time.Time)
				boundTime := result["boundTime"].(time.Time)
				containerTime := result["containerTime"].(time.Time)
				// 1:startTime 2:boundTime 3:attachTime 4:containerTime 5:endTime
				result["delta12"] = timeDiff(startTime, boundTime)
				result["delta23"] = timeDiff(boundTime, attachTime)
				result["delta34"] = timeDiff(attachTime, containerTime)
				result["delta45"] = timeDiff(containerTime, endTime)
				result["delta14"] = timeDiff(startTime, containerTime)
				// delta15 = duration
			}
		}
		// Calculate duration
		duration := timeDiff(startTime, endTime)
		result["duration"] = duration
		// Cleanup
		delete(result, "startTime")
		delete(result, "endTime")
		delete(result, "attachTime")
		delete(result, "boundTime")
		delete(result, "containerTime")
		delete(result, "pvName")
	}
}
