This quickstart example uses
[LocalVolumes](https://kubernetes.io/docs/concepts/storage/storage-classes/#local),
which creates a storage class that binds PVCs to PVs that have been
pre-provisioned using local storage on a node in the cluster.  So, first select
a node in your cluster and create a directory on that node that will be used for
our workload's storage volume.  Update `pv.yaml` accordingly:
1. Update the selector in the Node Affinity section to refer to the name of the
   node that you have selected.  The default value is "minikube".
2. Update the local path to point to the directory created on your selected
   node.  The default value is "/mnt/sda1/data/pv1".

Then, create the PV and the storage class:
```
kubectl apply -f pv.yaml
kubectl apply -f local-sc.yaml
```

Run the benchmark by creating the Benchmark resource specified in
`benchmark.yaml`:
```
kubectl apply -f benchmark.yaml
```

The CNSBench controller should start an fio workload.  The output from the
benchmark will be sent to the default CNSBench output collector, which you can
watch with:
```
kubectl logs -f cnsbench-output-collector -ncnsbench-system
```

### Cleanup

To delete any resources created during the benchmark run, do
```
kubeclt delete -f benchmark.yaml
```
The PV needs to be deleted as well:
```
kubectl delete -f pv.yaml
```
