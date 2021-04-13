# Object Timing Utility

A standalone utility that parses audit log files from the Kubernetes API server
to extract timing information about object creation & scaling.

Performs a tail-like read from a log file, and prints a JSON object to stdout
for each completed create or scale action on one of the specified resources in the
specified namespaces.

This program is compatible with log rotation, i.e. where when a log file reaches
a certain size, it is copied and truncated, or renamed and replaced with a new
file.

For Pods, PVCs, and PVs, this utility tracks the time to create each object.

If both Pods and PVCs are selected, the system calculates additional timing
information for Pods with PVCs attached to them:
- delta12: Time between when the Pod creation starts and when the PVC is bound to
  the Pod.
- delta23: Time between when the PVC is bound to the Pod and when it's attached to
  the Pod.
- delta34: Time between when the PVC is attached to the Pod and when the container
  starts being created in the Pod.
- delta14: Time between when the Pod creation starts and when the container starts
  being created in the Pod.

For Deployments, Jobs, ReplicaSets, and StatefulSets, this utility tracks the
time it takes to scale each object.

All timing information in the output is in nanoseconds.

## Usage  
#### Compile & run source code:   
```
$ go run ./*.go [filename] --rs=[resources] --ns=[namespaces]

	filename: name of the audit log file to read from
		Default is "/var/log/apiserver/audit.log"

	resources: comma-separated list of resources, i.e.
		pod,pvc,deployment,job,replicaset,statefulset
		Default resource is "pod"

	namespaces: comma-separated list of namespaces
		Default namespace is "default"
```
#### Compile source code to executable & run:  
```
$ go build -o objecttiming
$ ./objecttiming [filename] --rs=[resources] --ns=[namespaces]
```

## Implementation overview  

```
objecttiming
|-- README.md
|-- main.go
|-- tail.go
|-- output.go
|-- constants.go
|-- types.go
|-- parselogs.go
|-- parsetime.go
|-- objinfo.go
|-- create.go
|-- scale.go
|-- helpers.go
`-- pvcpod.go
```

### main.go
Defines the main function, which parses command-line arguments, sets
flags for which resources to parse timing information for, and calls
[`ParseLogs`](#func-ParseLogs(file-string,-flags-uint8,-namespaces-[]string)-error)
to parse the log file.

### tail.go
Defines function `readLogFile` and its helpers. Used by
[`ParseLogs`](#func-ParseLogs(file-string,-flags-uint8,-namespaces-[]string)-error)
in `parselogs.go`.
#### func readLogFile(path string, callback func(string) error) error
Runs a goroutine that reads the log file at `path` with tail-like behavior.
For each line in the file, `callback` is called with the line as its argument.

### output.go
Defines function `OutputJson`, which formats a [`jsondict`](#type-jsondict) record
as a JSON string and prints it to stdout.
Used by `outputFinishedActions` inside
[`ParseLogs`](#func-ParseLogs(file-string,-flags-uint8,-namespaces-[]string)-error)
in `parselogs.go`.

### constants.go
Constants for flags and strings.

### types.go
Type definitions shared between several source files.
#### type jsondict
Type definition for a JSON dictionary in Go.
#### type auditlog
In-memory representation of a single audit log.
Used to unmarshal relevant information from a log in the file to memory.
Contains all the below mentioned structs.
#### type objref
Relevant information about an object; resource, namespace, name, subresource.
#### type reqobject
Relevant information from an API request.
#### type involvedobject
Relevant information about an involved object; kind, namespace, name.
#### type respobject
Relevant information from an API response.
#### type spec
Relevant information from an object spec.

### parselogs.go
Defines function `ParseLogs` and its helpers.
#### func ParseLogs(file string, flags uint8, namespaces []string) error
Reads a log file, parses timing information for actions (create, scale) on relevant 
resources (`flags`) in relevant namespaces (`namespaces`).  
Actions are stored as JSON dictionaries ([`jsondict`](#type-jsondict)). In-progress
actions are stored in the `ongoing` slice, and completed actions are stored in the
`results` slice.   
This function calls
[`readLogFile(file, callback)`](#func-readLogFile(path-string,-callback-func(string)-error)-error),
which takes care of the tail-like reading of the log file.  
The callback function consists of calls to the nested functions `parseLog` and
`outputFinishedActions`:
- `parseLog` is the subroutine that parses each log in the file to generate
actions with timing information. For each log relevant to the resources/actions that
need to be parsed, it edits the actions in `ongoing` and `results` accordingly.
- `outputFinishedActions` is the subroutine that outputs and cleans up actions as they're finished.
For each completed action in `results`, it calculates its duration(s), prints it to stdout using [`OutputJson`](#output.go), and removes it from `results`.

### objinfo.go
Defines `isGetCreateRequest`, `saveObjInfo`, `getObjInfo`, and their helpers.   
These functions are for saving object information from any GET or CREATE requests
encountered, and accessing them later on when parsing scale actions.  
`isGetCreateRequest` and `saveObjInfo` are called by `parseLog` inside
[`ParseLogs`](#func-ParseLogs(file-string,-flags-uint8,-namespaces-[]string)-error)
in `parselogs.go`.  
`getObjInfo` is used by `isScaleStart` and `getScaleStart` in `scale.go`.

### create.go
Variables & functions for parsing create actions.

### scale.go
Variables & functions for parsing scale actions.

### helpers.go
Helper functions shared by both create and scale actions.

### pvcpod.go
Variables & functions for parsing additional PVC Pod timing information.
