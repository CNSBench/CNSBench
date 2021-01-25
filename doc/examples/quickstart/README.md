### Quickstart

This example will demonstrate using CNSBench to instantiate an I/O workload
([fio](https://github.com/CNSBench/workload-library/tree/master/workloads/fio)).  It does not require a storage
provider to be installed or configured, instead it uses
[LocalVolumes](https://kubernetes.io/docs/concepts/storage/storage-classes/#local)
to provision storage volumes.

### Running

First, [install CNSBench](https://github.com/CNSBench/CNSBench/#download-install).

Next, setup the LocalVolume storage: select a node in your cluster and create a
directory on that node that will be used for our workload's storage volume.
Update `pv.yaml` accordingly:
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
