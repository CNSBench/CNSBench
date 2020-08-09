/* output.go
Contains code to output object creation data in a table format
*/

package objecttiming

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"
)

func printCreateRow(rec jsondict, writer io.Writer) {
	fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t\t\n", rec["action"], rec["name"],
		rec["resource"], rec["duration"])
}

func printScaleRow(rec jsondict, writer io.Writer) {
	fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%d\t%d\n", rec["action"], rec["name"],
		rec["resource"], rec["duration"], rec["startReplicas"], rec["endReplicas"])
}

func printRow(rec jsondict, writer io.Writer) {
	switch rec["action"].(string) {
	case strCreate:
		printCreateRow(rec, writer)
		return
	case strScale:
		printScaleRow(rec, writer)
	default:
		printCreateRow(rec, writer)
	}
}

func PrintTableResults(records []jsondict) {
	// Initialize tabwriter for cleanly aligned columns
	writer := tabwriter.NewWriter(os.Stdout, 8, 8, 1, '\t', tabwriter.AlignRight)
	fmt.Fprintln(writer, "action\tname\tresource\tduration(microsec)\tstartReplicas\tendReplicas")
	for i := 0; i < len(records); i++ {
		printRow(records[i], writer)
	}
	writer.Flush()
}

func OutputJson(records []jsondict) {
	for i := 0; i < len(records); i++ {
		data, err := json.Marshal(records[i])
		if err != nil {
			panic(err)
		}
		fmt.Println(string(data))
	}
}
