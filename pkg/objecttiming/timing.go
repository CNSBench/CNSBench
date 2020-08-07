/** calculatetimes.go
 * Contains code for 
 * - parsing ISO8601 timestamps and calculating elapsed times for each 
 *   object's creation.
 * - getting start and end times from packets
 */

package objecttiming

import "time"

func getStartTime(log jsondict) (startTime time.Time) {
	strTime := log["requestReceivedTimestamp"].(string)
	var err error
	if startTime, err = time.Parse(time.RFC3339Nano, strTime); err != nil {
		panic(err)
	}
	return
}

func getEndTime(log jsondict) (endTime time.Time) {
	strTime := log["stageTimestamp"].(string)
	var err error
	if endTime, err = time.Parse(time.RFC3339Nano, strTime); err != nil {
		panic(err)
	}
	return
}

func timeDiff(t1, t2 time.Time) int64 {
	return t2.Sub(t1).Microseconds()
}