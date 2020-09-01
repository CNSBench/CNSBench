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
		// Save relevant object info, if applicable
		if isGetCreateRequest(log) {
			saveObjInfo(log, objstore)
		}
	}
	formatResourceNames(results)
	return results, nil
}

/* setEndTime
Calculates and records the duration of the action stored in record.
Also records any labels that the object may have.
*/
func setEndTime(log auditlog, record jsondict) {
	// Calculate duration
	startTime := record["startTime"].(time.Time)
	endTime := getEndTime(log)
	duration := timeDiff(startTime, endTime)
	// Delete startTime from the record
	delete(record, "startTime")
	// Set duration in the record
	record["duration"] = duration
	// Also record object labels (if there are any) at this point
	metadata := log.ResponseObject.Metadata
	if metadata.Labels != nil {
		record["labels"] = metadata.Labels
	}
}
