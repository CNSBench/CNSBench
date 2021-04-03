/* parselogs.go
Contains code for reading an audit file and extracting object
creation information.
Assumes the audit file consists of single-line json objects.
*/

package main

import (
	"encoding/json"
	"time"
)

func ParseLogs(file string, flags uint8, namespaces []string) error {
	// Initialize empty slices of actions, represented by dictionaries
	var ongoing []jsondict // temporary storage for actions still in progress
	var results []jsondict // final slice of finished actions
	
	// If no flags are set, return without doing anything
	if flags == 0 {
		return nil
	}

	// Initialize an objinfostore
	objstore := make(objinfostore)

	// Function for parsing a single log
	parseLog := func (line string) error {
		// Unmarshal the log string into a json dictionary
		var log auditlog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			return err
		}
		// Ignore the log if it's not in the ResponseComplete stage
		if log.Stage != "ResponseComplete" {
			return nil
		}
		// Create action parsing
		if flags & ParseCreate != 0 {
			if isCreateStart(log, ongoing) {
				record := getCreateStart(log)
				for _, o := range ongoing {
					if o["name"].(string) == log.ObjectRef.Name {
						return nil
					}
				}
				ongoing = append(ongoing, record)
				return nil
			} else if i := isCreateEnd(log, ongoing); i >= 0 {
				record := ongoing[i]
				setEndTime(log, record)
				if log.ObjectRef.Resource == "persistentvolumeclaims" {
					record["pvName"] = log.ResponseObject.Spec.VolumeName
				}
				ongoing = append(ongoing[:i], ongoing[i+1:]...)
				results = append(results, record)
				return nil
			}
		}
		if flags & ParseScale != 0 {
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
				return nil
			} else if i := isScaleEnd(log, ongoing); i >= 0 {
				record := ongoing[i]
				setEndTime(log, record)
				ongoing = append(ongoing[:i], ongoing[i+1:]...)
				results = append(results, record)
				return nil
			}
		}
		// Pod/PVC additional timestamps
		if flags & ParsePVCPod != 0 {
			// Time PVC is attachedd to Pod (attachTime)
			if isAttachVolumeEvent(log) {
				setAttachTime(log, ongoing)
				return nil
			}
			// Time Pod status changed to ContainerCreating (containerTime)
			if isContainerCreatingStatus(log) {
				setContainerTime(log, ongoing)
				return nil
			}
		}
		// Save relevant object info, if applicable
		if isGetCreateRequest(log) {
			saveObjInfo(log, objstore)
		}
		return nil
	}

	// Function for outputting finished results as they're done
	outputFinishedActions := func () {
		// Calculate durations
		for _, result := range results {

			startTime := result["startTime"].(time.Time)
			endTime := result["endTime"].(time.Time)

			if flags&ParsePVCPod != 0 && result["resource"] == "pod" {
				// Calculate intermediate deltas for Pod/PVC timings
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
			} else {
				result["duration"] = timeDiff(startTime, endTime)
				delete(result, "startTime")
				delete(result, "endTime")
			}
		}
		for i, result := range results {
			if _, ok := result["duration"]; ok {
				results = append(results[0:i], results[i+1:len(results)]...)
				for _, ns := range namespaces {
					if ns == result["namespace"] {
						formatResourceName(result)
						OutputJson(result)
					}
				}
			}
		}
	}

	callback := func (line string) error {
		if err := parseLog(line); err != nil {
			return err
		}
		outputFinishedActions()
		return nil
	}

	// Invoke readLogFile on file with parseLog as a callback
	if err := readLogFile(file, callback); err != nil {
		return err
	}

	return nil
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
