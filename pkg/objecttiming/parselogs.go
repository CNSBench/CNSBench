/** parselogs.go
 * Contains code for reading an audit file and extracting object
 * creation information.
 * Assumes the audit file consists of single-line json objects.
 */

package objecttiming

import(
	"bufio"
	"encoding/json"
	"io"
)

func ParseLogs(reader io.Reader, flags uint8) ([]jsondict) {
	// Initialize empty array of operations, represented by dictionaries
	var results []jsondict
	// Create a scanner to wrap the reader. Split by lines (default)
	scanner := bufio.NewScanner(reader)
	// For each line/log:
	for scanner.Scan() {
		// Get the line that represents the next log
		line := scanner.Text()
		// Unmarshal the log string into a json dictionary
		var log jsondict
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			panic(err)
		}
		// Create operation parsing
		if flags & ParseCreate != 0 {
			if isCreateStart(log, results) {
				record := getCreateStart(log)
				results = append(results, record)
				continue
			} else if isCreateEnd(log, results) {
				i := getCreateEndIndex(log, results)
				setEndTime(log, &(results[i]))
				continue
			}
		}
		// Scale operation parsing
		if flags & ParseScale != 0 {
			if isScaleStart(log, results) {
				record := getScaleStart(log)
				results = append(results, record)
				continue
			} else if i := isScaleEnd(log, results); i >= 0 {
				setEndTime(log, &(results[i]))
				continue
			}
		}
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
	return results
}
