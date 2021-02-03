# Benchmark Specification

Describes the fields of a cnsbench.Benchmark object.  Field names that are in
**bold** indicates a required field.

### cnsbench.Benchmark
| Field | Description |
| :- | - |
| **apiVersion**<br />*string* | Should be set to `cnsbench.example.com/v1alpha1`. |
| **kind**<br />*string* | Should be set to `Benchmark`. |
| **metadata**<br />*[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta)* | Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata. |
| **spec**<br />*[cnsbench.BenchmarkSpec](#cnsbenchbenchmarkspec)* | Benchmark's specification. |
| status<br />*[cnsbench.BenchmarkStatus](#cnsbenchbenchmarkstatus)* | Benchmark's status. Updated by CNSBench, not set by the user. |

### cnsbench.BenchmarkSpec
| Field | Description |
| :- | - |
| runtime<br />*string*| Duration of the benchmark run.  Must be a string that can be parsed with [time.ParseDuration](https://golang.org/pkg/time/#ParseDuration).  Currently Runtime is only used to restart workloads if they complete before the Runtime duration has elapsed; CNSBench cannot stop workloads if they continue running past Runtime.  |
| volumes<br />*[][cnsbench.Volume](#cnsbenchvolume)* | Array of cnsbench.Volume specifications for volumes that will be created by CNSBench. |
| workloads<br />*[][cnsbench.Workload](#cnsbenchworkload)* | Array of cnsbench.Workload specifications for the I/O workloads that CNSBench will instantiate. |
| controlOperations<br />*[][cnsbench.ControlOperation](#cnsbenchcontroloperation)* | Array of cnsbench.ControlOperation specifications for the control operations CNSBench will execute. |
| rates<br />*[][cnsbench.Rate](#cnsbenchrate)* | Array of Rates that are used to trigger the creation of a volume, instantiation of an I/O workload, or execution of control operations. |
| outputs<br />*[][cnsbench.Ouptut](#cnsbenchoutput)* | Array of cnsbench.Output specifications. |

### cnsbench.BenchmarkStatus
| Field | Description |
| :- | - |
| **state**<br />*[cnsbench.BenchmarkState](#cnsbenchbenchmarkstate)* | |
| **startTimeUnix**<br />*int64* | Time that the benchmark started,as a Unix timestamp. |
| InitCompletionTime<br />*[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#time-v1-meta)* | Time that all workloads finished initialization. |
| **initCompletionTimeUnix**<br />*int64* | Time that all workloads finished initialization, as a Unix timestamp. |
| CompletionTime<br />*[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#time-v1-meta)* | Time that the benchmark finished. |
| **completionTimeUnix**<br />*int64* | Time that the benchmark finished, as a Unix timestamp. |
| **numCompletedObjs**<br />*int* | Number of workload objects that were started and have completed since the beginning of the benchmark. |
| **conditions**<br />[][cnsbench.BenchmarkCondition](#cnsbenchbenchmarkcondition) | Array of cnsbench.BenchmarkCondition objects, used to indicate if the benchmark has completed.  |

### cnsbench.BenchmarkCondition
Same as the condition resources for other kinds of resource, e.g. [PodCondition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#podcondition-v1-core).  Currently the only condition type is "Complete", which indicates if the benchmark has finished (i.e., all workloads have finished running.)  The `kubectl wait` command can be used to watch for this condition to become True:
```
kubectl wait --for=condition=Complete benchmark/benchmark-name
```
| Field | Description |
| :- | - |
| lastProbeTime<br />*[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#time-v1-meta)* | Last time we probed the condition. |
| **lastTransitionTime**<br />*[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#time-v1-meta)* | Last time the condition transitioned from one status to another. |
| message<br />*string* | String describing last transition. |
| reason<br />*string* | Reason for last transition. |
| **status**<br />*string* | Status of the condition, can be True or False. |
| **type**<br />*string* | Type of condition.  Currently the only type is "Complete", which is used to indicate if the benchmark is complete (Status = True) or not (Status = False). |

# Workloads
### cnsbench.Workload
| Field | Description |
| :- | - |
| **name**<br />*string* | Name of workload instance.  If multiple workloads are instantiated (e.g. if Count is specified, or if an associated rate causes more workloads to be instantiated), the number of the workload will be appended. |
| **workload**<br />*string* | Name of workload to instantiate. See https://github.com/CNSBench/workload-library/tree/master/workloads for available workloads. |
| vars<br />*map[string]string* | Map of parameter and the values they will be set to.  See the workload's documentation for available parameters.
| count<br />*int* | Number of workloads to instantiate. |
| syncGroup<br />*string* | All workloads with the same sync group label are 
| outputFiles<br />*[][cnsbench.OutputFile](#cnsbenchoutputfile)* | Array of cnsbench.OutputFiles. |
| rateName<br />*string* | Rate that will run this workload. Workload is instantiated when Benchmark is instantiated if no rate is provided. |

### cnsbench.OutputFile
| Field | Description |
| :- | - |
| **filename**<br />*string* | Filename from workload to parse. |
| parser<br />*string* | Parser to use. |
| target<br />*string* | Part of the workload with the file that should be parsed. |
| sink<br />*string* | Name of the cnsbench.Output that the parsed output will be sent to. |

# ControlOperations
### cnsbench.ControlOperation
| Field | Description |
| :- | - |
| **name**<br />*string*| Name of the control operation. |
| snapshotSpec <br />*[cnsbench.Snapshot](#snapshot)* | cnsbench.Snapshot specification.  This control operation will snapshot a volume. |
| scaleSpec<br />*[cnsbench.Scale](#scale)* | cnsbench.Scale specification. This control operation will scale a resource. |
| deleteSpec<br />*[cnsbench.Delete](#delete)* | cnsbench.Delete specification. This control operation will delete a resource. |
| outputs<br />*[cnsbench.ActionOutput](#action-output)* | cnsbench.ActionOutput that specifies where output from this control operation should be sent. |
| rateName<br />*string* | Name of the cnsbench.Rate that triggers this control operation. |

### cnsbench.Snapshot
| Field | Description |
| :- | - |
| workloadName<br />*string* | Name of a [cnsbench.Workload](#cnsbenchworkload). CNSBench will snapshot all volumes created for this workload (i.e., volumes whose resource definition is included as part of the I/O workload specification). |
| volumeName<br />*string* | Name of a [cnsbench.Volume](#cnsbenchvolume). CNSBench will snapshot all volumes created for this Volume specification. |
| **snapshotClass**<br />*string* | Name of the [VolumeSnapshotClass](https://kubernetes.io/docs/concepts/storage/volume-snapshot-classes/) used to create the snapshot. |

### cnsbench.Scale
!! Support for scaling control operations is very much in-progress.  See the [scaling control operation design document](scaling_design_doc) for details.
| Field | Description |
| :- | - |
| **objName**<br />*string* | Name of object that should be scaled. |
| **scriptConfigMap**<br />*string* | Name of ConfigMap that contains the script that does the actual scaling. |

### cnsbench.Delete
!! Currently only VolumeSnapshots can be deleted.  Our implementation needs to
be generalized to support additional kinds of resources.
| Field | Description |
| :- | - |
| **selector**<br />*[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#labelselector-v1-meta) | Label query used to lookup snapshots.  The oldest snapshot in the result list will be deleted. |

### cnsbench.ActionOutput
| Field | Description |
| :- | - |
| outputName<br />*string* | Name of cnsbench.Output which specifies where a control operation's output should be sent.

# Volumes
### cnsbench.Volume
| Field | Description |
| :- | - |
| **name**<br />*string* | Name of volume.  If multiple volumes are to be created (e.g. if a Count is provided or volumes are created via a Rate), the number of volume is appended to the name. |
| count<br />*int* | Number of volumes to be instantiated. |
| **spec**<br />*[PersistentVolumeClaimSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#persistentvolumeclaimspec-v1-core)* | Specification of PVC to be instantiated. |
| rateName<br />*string* | Rate that will trigger creation of volumes.  If not specified, a single volume will be instantiated when the Benchmark is instantiated. |

# Rates
### cnsbench.Rate
Wrapper for rates.
| Field | Description |
| :- | - |
| **name**<br />*string* |
| constantRateSpec<br />*[cnsbench.ConstantRate](#cnsbenchconstantrate)* | Specification for a constant counter rate. |
| constantIncreaseDecreaseRateSpec<br />*[cnsbench.ConstantIncreaseDecreaseRate](#cnsbenchconstantincreasedecreaserate)* | Specification for a constant increasing/decreasing rate. |

### cnsbench.ConstantRate
Rate based on a single counter.  Counts up indefinitely.
| Field | Description |
| :- | - |
| **interval**<br />*int* | Interval to count up by. |

### cnsbench.ConstantIncreaseDecreaseRate
Rate based on a single counter, which counts up and then back down between a
maximum and minimum value.  Runs indefinitely.
| Field | Description |
| :- | - |
| **incInterval**<br />*int* | Interval to count up by. |
| **decInterval**<br />*int* | Interval to count down by. |
| **max**<br />*int* | Number to count up to. |
| **min**<br />*int* | Number to count down to (and number to start counting at). |

# Outputs
### cnsbench.Output
Wrapper for outputs.
| Field | Description |
| :- | - |
| **name**<br />*string* | Name of output, used to specify this output in other sections of the Benchmark specification |
| httpPostSpec<br />*[cnsbench.HttpPost](#cnsbenchhttppost)* | Specification of HTTP POST output sink. |

### cnsbench.HttpPost
| Field | Description |
| :- | - |
| **url**<br />*string* | URL of output sink.  Must include the scheme (e.g. http). |
