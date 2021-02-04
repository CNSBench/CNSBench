### Default output collector

If your cluster already has a data collection tool setup (e.g., Elasticsearch,
Splunk, etc.), collecting CNSBench output should be as simple as specifying the
correct URL to send the output to in your Benchmark resource's definition.

In case your cluster does not have a collection tool already setup, CNSBench
comes with a simple utility that can be used to collect output.  If a workload
has an output with no associated `cnsbench.Output` specification, the output is
sent to this default collector.  If the `metadataOutput` field is not set in the
Benchmark resource specification, the benchmark's metadata output is sent to the
default collector.

The default output collector runs a basic HTTP server in the pod
`cnsbench-output-collector` in the `cnsbench-system` namespace.  It is exposed
by a service which is accessible from within the Kubernetes cluster at the
address
`http://cnsbench-output-collector.cnsbench-system.svc.cluster.local:8888/metadata/`.
When the HTTP server receives data, it prints a JSON object with:
* Unix timestamp that the data was received
* Address of the sender
* Endpoint the data was sent to (the portion of the URL after the port number
  and before any query string)
* A `key: value` dictionary of the query parameters
* The data

This output can be viewed with the command `kubectl logs
cnsbench-output-collector -ncnsbench-system`.  To save output to a file for
processing at a later time, do
```
kubectl logs cnsbench-output-collector -ncnsbench-system > outfile
```
or to save output as it is received,
```
kubectl logs -f cnsbench-output-collector -ncnsbench-system > outfile
```

The output can be filtered using standard text processing utilities.  For
example, if a benchmark consists of an
[fio](https://github.com/CNSBench/workload-library/tree/master/workloads/fio)
workload which is sent to the "fio" endpoint" and a
[filebench](https://github.com/CNSBench/workload-library/tree/master/workloads/filebench)
workload which is sent to the "filebench" endpoint, the output can be saved to
separate files:
```
kubectl logs -f cnsbench-output-collector -ncnsbench-system | grep -v metadata | grep fio > fio-outfile
kubectl logs -f cnsbench-output-collector -ncnsbench-system | grep -v metadata | grep filebench > filebench-outfile
```
Note that the metadata output needs to be filtered out first, since the
benchmark specification included in the metadata output will contain both the
strings "fio" and "filebench."
