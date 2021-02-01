# About

CNSBench is a tool that enables benchmarking storage in cloud native
environments.  It does so by orchestrating the execution of one or more I/O
workloads along with control workloads operating on the same storage system as
the I/O workloads.  The I/O workloads are generated by instantiating standard
Kubernetes resources such as pods or volumes.  In other words, any application
that can be run on Kubernetes could be used to generate an I/O workload.

CNSBench has been tested on Kubernetes v1.20.0.

# Download & Install<a name="download-install"></a>

Clone CNSBench with the Workload Library:
```
git clone git@github.com:CNSBench/CNSBench.git --recurse-submodules
```

To install either do
```
make deploy-from-manifests
```
or
```
kubectl apply -f deploy/cnsbench_operator.yaml
cd workload-library && sh install.sh && cd ..
```

This will install the custom resource definitions and begin the CNSBench
controller and default output collector pods.  CNSBench uses the cnsbench-system
namespace for the controller and related resources and the cnsbench-library
namespace for workload definitions.

# Usage

If minikube is installed, running `sh minikube-test.sh` will start minikube,
install CNSBench, setup a local storage provider, and run a sample benchmark.

Running these steps manually is also easy:

[![asciicast](doc/examples/quickstart/asciinema.svg)](https://asciinema.org/a/OT0LQXxUe6dtowlxOwkL4zSdu)

See the [quickstart guide](doc/examples/quickstart) for more details.

When a new Benchmark resource is created the CNSBench controller will run the
I/O and control workloads specified in that resource.  See
[here](doc/benchmark\_resource.md) for details on how to specify a Benchmark
resource.

# Paper

[FAST 2021
presentation](https://www.usenix.org/conference/fast21/presentation/merenstein)

#### Abstract

Modern hybrid cloud infrastructures require software to be easily portable
between heterogeneous clusters. Application containerization is a proven
technology to provide this portability for the functionalities of an
application. However, to ensure performance portability, dependable verification
of a cluster’s performance under realistic workloads is required. Such
verification is usually achieved through benchmarking the target environment and
its storage in particular, as I/O is often the slowest component in an
application. Alas, existing storage benchmarks are not suitable to generate
cloud native workloads as they do not generate any storage control operations
(e.g., volume or snapshot creation), cannot easily orchestrate a high number of
simultaneously running distinct workloads, and are limited in their ability to
dynamically change workload characteristics during a run.

In this paper, we present the design and prototype for the first-ever Cloud
Native Storage Benchmark - CNSBench. CNSBench treats control operations as
first-class citizens and allows to easily combine traditional storage benchmark
workloads with user-defined control operation workloads. As CNSBench is a cloud
native application itself, it natively supports orchestration of different
control and I/O workload combinations at scale. We built a prototype of CNSBench
for Kubernetes, leveraging several existing containerized storage benchmarks for
data and meta-data I/O generation. We demonstrate CNSBench’s usefulness with
case studies of Ceph and OpenEBS, two popular storage providers for Kubernetes,
uncovering and analyzing previously unknown performance characteristics.
