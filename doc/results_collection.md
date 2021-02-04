### Running CNSBench

The general workflow for using CNSBench is:
1. Run a benchmark: `kubectl apply -f benchmark.yaml`
2. Wait for the benchmark to complete: `kubectl wait --for=condition=Complete benchmark`
3. Collect results and metrics

### Collecting Results & Metrics

How results and metrics are collected depend on what the selected parsers output
from the workloads, where output is configured to be sent, and what metrics
collection agents are running in the cluster.  In general, it is useful to get
the start and end times of the benchmark run:
```
starttime=`kubectl get benchmark -ojson | jq .items[0].status.initCompletionTimeUnix`
endtime=`kubectl get benchmark -ojson | jq .items[0].status.completionTimeUnix`
```

These timestamps can be used to query whichever metrics collection database is
in use.  For example, to query Metricbeat metrics in an Elasticsearch database,
you could do:
```
curl -X GET "es-instance:9200/metricbeat-index/_search?pretty" -H 'Content-Type: application/json' -d'
{
  "query": {
    "range": {
      "@timestamp": {
       "gte": $starttime,
       "lte": $endtime,
       "format": "epoch_second"
      }
    }
  }
  "sort": [
    {
      "@timestamp": {
        "order: "asc"
      }
    }
  ]
}
'
```
