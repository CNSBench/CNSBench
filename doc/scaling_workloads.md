# User information



```
apiVersion: cnsbench.example.com/v1alpha1
kind: Benchmark
metadata:
  name: ycsb-cassandra
spec:
  workloads:
    - name: cassandra-test
      workload: ycsb-cassandra
      count: 1
      vars:
        recordcount: "1000000"
        operationcount: "500000"
  controlOperations:
    - name: scaleCassandra
      rateName: once
      scaleSpec:
        objName: rook-cassandra
        scaleScripts: scale-cassandra
        serviceAccountName: internal-kubectl
  rates:
    - name: once
      constantIncreaseDecreaseRateSpec:
        incInterval: 900
        decInterval: 900
        max: 2
        min: 1
```

Since this benchmark is sending all outputs to the default output collector,
after the benchmark completes we can get the results with
```
kubectl logs cnsbench-output-collector -ncnsbench-system
```

The output collector may have results from prior benchmark runs, to select just the results from our most recent run we can do:
```
starttime=$(kubectl get benchmark -ojson | jq .items[0].status.initCompletionTimeUnix)
kubectl logs cnsbench-output-collector -ncnsbench-system | jq -s ".[]  | select (.timestamp > $starttime)"
```

Since we're sending all of our metrics, metadata, and workload output to the
output collector we want to split the different pieces of out into separate
files.  We'll split the workload output into one file, and the metrics that
tell us when the workload was scaled into a different file:
```
kubectl logs cnsbench-output-collector -ncnsbench-system | jq -s ".[] | select (.timestamp > $starttime) | select(.endpoint | contains(\"workload\")) | .data | fromjson" > workload.out
echo $starttime,1 >scaletimes.out
kubectl logs cnsbench-output-collector -ncnsbench-system | jq -rs ".[] | select (.timestamp > $starttime) | select(.endpoint | contains(\"metrics\")) | select (.data | fromjson | select(.type | contains(\"rateFired\"))) | [.timestamp, (.data | fromjson | .n)] | join(\",\")" >> scaletimes.out
```

# Developer information

Scaling workloads in a generic fashion is difficult, since many workloads that
support multi-node configurations are now deployed using operators (e.g., the
Rook operators for CassandraDB and CockroachDB).  These operators wrap the
deployment in a custom resource, and scaling the number of nodes in the cluster
is often done by modifying this custom resource.  Since this can not be done in
a generic fashion, we rely on "scaling scripts" to be supplied by the workload
author.  These scripts take two arguments: the name of the object being scaled,
and the number of instances the object should be scaled to.  When a CNSBench
executes a Scale control operation, it runs the scaling script in a pod.  Since
the pod needs to have permission to make changes to the target object, we
require that as part of the Scale control operation specification, the user
supplies the name of a service account which will have the necessary
permissions.
