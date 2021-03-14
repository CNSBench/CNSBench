/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HttpPost struct {
	URL string `json:"url"`
}

type ActionOutput struct {
	OutputName string `json:"outputName"`
}

type Output struct {
	Name string `json:"name"`
	// +optional
	HttpPostSpec HttpPost `json:"httpPostSpec"`
}

type ConstantIncreaseDecreaseRate struct {
	IncInterval int `json:"incInterval"`
	DecInterval int `json:"decInterval"`
	Max         int `json:"max"`
	Min         int `json:"min"`
}

type ConstantRate struct {
	Interval int `json:"interval"`
}

type Rate struct {
	Name string `json:"name"`

	// +optional
	ConstantRateSpec ConstantRate `json:"constantRateSpec,omitempty"`
	// +optional
	ConstantIncreaseDecreaseRateSpec ConstantIncreaseDecreaseRate `json:"constantIncreaseDecreaseRateSpec,omitempty"`
}

// Snapshots and deletions can operate on an individual object or a selector
// if a selector, then there may be multiple objects that match - should
// specify different policies for deciding which object to delete, e.g.
// "newest", "oldest", "random", ???
type Snapshot struct {
	// +optional
	// +nullable
	WorkloadName string `json:"workloadName"`

	// +optional
	// +nullable
	VolumeName string `json:"volumeName"`

	SnapshotClass string `json:"snapshotClass"`
}

type Delete struct {
	APIVersion string               `json:"apiVersion"`
	Kind       string               `json:"kind"`
	Selector   metav1.LabelSelector `json:"selector"`
}

// TODO: need a way of specifying how to scale - up or down, and by how much
type Scale struct {
	// +optional
	// +nullable
	ObjName string `json:"objName"`
	// +optional
	// +nullable
	ScaleScripts string `json:"scaleScripts"`

	// +optional
	// +nullable
	WorkloadName string `json:"workloadName"`

	ServiceAccountName string `json:"serviceAccountName"`
}

type OutputFile struct {
	// Filename of output file, as it will exist inside the workload container
	Filename string `json:"filename"`

	// Name of parser configmap.  Defaults to the null-parser if not specified,
	// which is a no-op.
	// +optional
	// +nullable
	// +kubebuilder:default:=null-parser
	Parser string `json:"parser"`

	// If there are multiple resources created by the workload (e.g., client and
	// server), target specifies which resource this is referring to.  See the
	// workload spec's documentation to see what targets are available.  If none
	// is specified, defaults to "workload"
	// +optional
	// +nullable
	// +kubebuilder:default:=workload
	Target string `json:"target"`

	// +optional
	// +nullable
	// +kubebuilder:default:=defaultWorkloadsOutput
	Sink string `json:"sink"`
}

// Creates PVCs with given name.  If count is provided, the name will be
// name-<volume number>.  Workloads that require volumes should parameterize
// the name of the volume, and the user should provide the name of a Volume
// as the value.
type Volume struct {
	Name string `json:"name"`

	// +optional
	// +nullable
	// +kubebuilder:default:=1
	Count int `json:"count"`

	Spec corev1.PersistentVolumeClaimSpec `json:"spec"`

	// +optional
	// +nullable
	RateName string `json:"rateName"`
}

type Workload struct {
	Name string `json:"name"`

	Workload string `json:"workload"`

	// +optional
	// +nullable
	Vars map[string]string `json:"vars"`

	// +optional
	// +nullable
	// +kubebuilder:default:=1
	Count int `json:"count"`

	// +optional
	// +nullable
	SyncGroup string `json:"syncGroup"`

	// +optional
	// +nullable
	OutputFiles []OutputFile `json:"outputFiles"`

	// +optional
	// +nullable
	RateName string `json:"rateName"`
}

type ControlOperation struct {
	Name string `json:"name"`

	// +optional
	// +nullable
	SnapshotSpec Snapshot `json:"snapshotSpec"`

	// +optional
	// +nullable
	ScaleSpec Scale `json:"scaleSpec"`

	// +optional
	// +nullable
	DeleteSpec Delete `json:"deleteSpec"`

	// +optional
	// +nullable
	Outputs ActionOutput `json:"outputs"`

	// +optional
	// +nullable
	RateName string `json:"rateName"`
}

// BenchmarkSpec defines the desired state of Benchmark
type BenchmarkSpec struct {
	// +optional
	// +nullable
	Runtime string `json:"runtime"`

	// +optional
	// +nullable
	Volumes []Volume `json:"volumes"`

	// +optional
	// +nullable
	Workloads []Workload `json:"workloads"`

	// +optional
	// +nullable
	ControlOperations []ControlOperation `json:"controlOperations"`

	// +optional
	// +nullable
	Rates []Rate `json:"rates"`

	// +optional
	// +nullable
	// +kubebuilder:default:=defaultWorkloadsOutput
	WorkloadsOutput string `json:"workloadsOutput"`

	// Output sink for the benchmark metadata, e.g. the spec and
	// start and completion times
	// +optional
	// +nullable
	// +kubebuilder:default:=defaultMetadataOutput
	MetadataOutput string `json:"metadataOutput"`

	// +optional
	// +nullable
	// +kubebuilder:default:=defaultMetricsOutput
	MetricsOutput string `json:"metricsOutput"`

	// +optional
	// +nullable
	Outputs []Output `json:"outputs"`
}

type BenchmarkState string

const (
	Complete     BenchmarkState = "Complete"
	Running      BenchmarkState = "Running"
	Initializing BenchmarkState = "Initializing"
)

type BenchmarkCondition struct {
	// +optional
	// +nullable
	LastProbeTime      metav1.Time `json:"lastProbeTime"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// +optional
	// +nullable
	Message string `json:"message"`
	// +optional
	// +nullable
	Reason string `json:"reason"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

// BenchmarkStatus defines the observed state of Benchmark
type BenchmarkStatus struct {
	State BenchmarkState `json:"state"`

	// +optional
	// +nullable
	CompletionTime metav1.Time `json:"completionTime"`

	// +optional
	// +nullable
	InitCompletionTime metav1.Time `json:"initCompletionTime"`

	// +optional
	// +nullable
	TargetCompletionTimeUnix int64 `json:"targetCompletionTimeUnix"`
	// +optional
	// +nullable
	TargetCompletionTime metav1.Time `json:"targetCompletionTime"`

	CompletionTimeUnix     int64 `json:"completionTimeUnix"`
	StartTimeUnix          int64 `json:"startTimeUnix"`
	InitCompletionTimeUnix int64 `json:"initCompletionTimeUnix"`

	NumCompletedObjs int `json:"numCompletedObjs"`

	// This doesn't include RuneOnce actions
	RunningWorkloads int `json:"runningWorkloads"`
	RunningRates     int `json:"runningRates"`

	Conditions []BenchmarkCondition `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Benchmark is the Schema for the benchmarks API
type Benchmark struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BenchmarkSpec   `json:"spec,omitempty"`
	Status BenchmarkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BenchmarkList contains a list of Benchmark
type BenchmarkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Benchmark `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Benchmark{}, &BenchmarkList{})
}
