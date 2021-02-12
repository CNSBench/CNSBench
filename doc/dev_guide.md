## Developer Notes

To test changes without rebuilding the CNSBench docker image, make sure all the
CRDs and tokens are installed (`make deploy`).  This will also run the CNSBench
controller using the docker image, so to stop this controller scale the
deployment to zero replicas:
```
kubectl scale deployment cnsbench-controller-manager --replicas=0 -ncnsbench-system
```

Then run the controller locally with
```
make run ENABLE_WEBHOOKS=false
```

The CNSBench default output collector won't be accessible to this locally run
controller, but the output collector can also be run locally:
```
go run output-collector/main.go
```

The benchmark resource then needs to be updated so the metrics and metadata
outputs are sent to this localhost output collector by adding the following
outputs:
```
outputs:
  - name: defaultMetricsOutput
    httpPostSpec:
      url: http://localhost:8888/metrics
  - name: defaultMetadataOutput
    httpPostSpec:
      url: http://localhost:8888/metadata
```

The workloads' outputs are sent from a helper container within the workload
pods, so those output can be sent to the default output collector.  They can be
viewed like normal, with
```
kubectl logs cnsbench-output-collector -ncnsbench-system
```
