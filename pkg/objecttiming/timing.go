/* calculatetimes.go
Contains code for
- parsing ISO8601 timestamps and calculating elapsed times for each
  object's creation.
- getting start and end times from packets
*/

package objecttiming

import "time"

func parseStrTime(strTime string) (parsedTime time.Time) {
	var err error
	if parsedTime, err = time.Parse(time.RFC3339Nano, strTime); err != nil {
		panic(err)
	}
	return
}

func getStartTime(log auditlog) time.Time {
	strTime := log.RequestReceivedTimestamp
	return parseStrTime(strTime)
}

func getEndTime(log auditlog) time.Time {
	strTime := log.StageTimestamp
	return parseStrTime(strTime)
}

func timeDiff(t1, t2 time.Time) int64 {
	return t2.Sub(t1).Microseconds()
}
