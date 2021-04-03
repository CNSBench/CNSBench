/* output.go
Contains code to output object creation data in a table format
*/

package main

import (
	"encoding/json"
	"fmt"
)

func OutputJson(record jsondict) {
	data, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
}
