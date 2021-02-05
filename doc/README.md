# Documentation

Instructions for downloading and installing CNSBench can be found [here](../README.md#download-install).

## Specifying a Benchmark resource
Users specify the I/O workloads they want to instantiate and the control
operation workload they want executed using CNSBench's custom Benchmark
resource.

Below is a heavily commented sample Benchmark resource.  Only the more common
fields are included, for full details of all possible fields see [here](benchmark_resource.md).

```YAML
# A Benchmark resource starts with the type information that tells Kubernetes what
# kind of resource this is.  These first two lines will always be the same:
apiVersion: cnsbench.example.com/v1alpha1
kind: Benchmark

# Next is the metadata section, which is the same as the metadata section for any
# other standard Kubernetes resource.  It is used to name this Benchmark resource:
metadata:
  name: example-benchmark

# The previous lines are common to most Kubernetes resources, but the rest of
# the manifest will be custom to the CNSBench Benchmark resource:
spec:

  # The workloads field constists of an array of Workload objects:
  # https://github.com/CNSBench/CNSBench/blob/alex-dev/doc/benchmark_resource.md#cnsbenchworkload
  workloads:
    - name: test-workload
    # Name of the workload: references one of the workloads installed in the
    # cnsbench-library namespace.  # Do `kubectl get configmaps -ltype=workload
    # -ncnsbench-library` to see installed workloads, and use one of these ConfigMap
    # names in this field:
    - workload: example-workload
    # You can optionally specify which files you want collected from the output.
    # Often using the default values defined in the I/O workload specification is
    # ok.  # See the full Benchmark resource documentation for details.
    outputFiles:
    - filename: /output/output
      parser: basic-parser
      target: workload
      sink: es
    - filename: /output/other-output
      parser: null-parser
      target: server
      sink: es
    # I/O workloads are often parameterized.  The "var" field here is a key:
    # value map that lets users provide the # values for those parameters.  See the
    # I/O workload specification's documentation for available parameters.
    vars:
      param1: value1
      param2: value2
    # count is used to create multiple instances of a workload.  What that means
    # exactly depends on the workload - for example, for simple workloads that just
    # run a single synthetic I/O benchmark application it probably means that multiple
    # instances of the application will be created.  For workloads that instantiate
    # both a server and a client, it might mean that multiple instances of the client
    # will be instantiated.  See the workload's documentation for details.
    count: 1

    # This is a second example workload.  It will be instantiated at the same
    # time as the previous workload, when the Benchmark resource is created.
    - name: second-test-workload
      workload: second-example-workload
      vars:
        # This workload does not include a volume, so it references a Volume
        # defined later on in this Benchmark resource.  That volume will then be
        # used for this workload.
        volname: test-vol
      
  # This is an optional array of Volumes.  Many I/O workloads include a
  # volume, so the volume will be instantiated when the workload is
  # instantiated.  Define a volume here if instantiating a workload that requires a
  # volume but does not create one itself.  Check your I/O workloads' documentation
  # to see if this is needed.
  # https://github.com/CNSBench/CNSBench/blob/alex-dev/doc/benchmark_resource.md#cnsbenchvolume
  volumes:
    - name: test-vol

      # This is a PersistentVolumeClaimSpec, the same as if you were defining a
      # standard PVC resource.  See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#persistentvolumeclaimspec-v1-core
      spec:
        ...

  # Array of ControlOperations.  Each should have a name, a rateName, and
  # then the operation-specific specification field (e.g., "snapshotSpec" or
  # "deleteSpec").  Each ControlOperation should only have one
  # operation-specific field.
  # https://github.com/CNSBench/CNSBench/blob/alex-dev/doc/benchmark_resource.md#cnsbenchcontroloperation
  controlOperations:
    - name: snapshot-op

      # Operation-specific field, for snapshot operations:
      snapshotSpec:
        # Snapshots can specify either a WorkloadName or a VolumeName.  By
        # specifying a WorkloadName, that tells CNSBench to snapshot the volume
        # being used by that workload.
        - workloadName: test-workload

      # Rate that will trigger this control operation to be executed.
      # Corresponds to a rate defined below in this same Benchmark resource.
      rateName: rate-one

    - name: snapshot-op2
      snapshotSpec:
        # Snapshots can specify either a WorkloadName or a VolumeName.  By
        # specifying a VolumeName, that tells CNSBench to snapshot the volume
        # created by this Volume specification in this Benchmark resource.
        - volumeName: test-workload
      rateName: rate-two

  # Array of Rates.  Each should have a name and then a specification field
  # specific to the kind of rate being defined.
  # https://github.com/CNSBench/CNSBench/blob/alex-dev/doc/benchmark_resource.md#cnsbenchrate
  rates:
    - name: rate-one

      # This rate is a "constant" rate, so it uses the "constantRateSpec" field.
      constantRateSpec:
        interval: 60s

  # Array of Outputs.  Each output is identified with a name and specifies a
  # URL.  Workload, ControlOperation, and the "metadataOutput" and
  # "allWorkloadOutput" fields can reference one of these outputs to indicate the
  # URL their output should be sent to.
  outputs:
    - name: es
      httpPostSpec:
        url: http://es:9200/workload-index/
```

## Examples
1. [quickstart](examples/quickstart) demonstrates how to use CNSBench to run a
   synthetic I/O benchmark ([fio](https://github.com/axboe/fio).)  It includes
   instructions on provisioning volumes from local directories, so that CNSBench
   can run I/O workloads without needing to install a separate storage provider.
2. [output](examples/output) demonstrates how to specify which parser to use
   with each output file, and what the resulting output will look like.
3. [warmup](examples/warmup) demonstrates running an I/O workload that requires
   a warmup period prior to running the main workload.
4. [complex-workload](examples/complex-workload) demonstrates a more complex I/O
   workload, which instantiates a replicated server and multiple clients.
