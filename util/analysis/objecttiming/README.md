# Object Timing Utility

A standalone utility that parses audit log files from the Kubernetes API server
to extract timing information about object creation & scaling.

## Usage  
#### Compile & run source code:   
```
$ go run ./*.go [filename]
```
#### Compile source code to executable & run:  
```
$ go build -o objecttiming
$ ./objecttiming [filename]
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
