package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type HttpPost struct {
	URL string `json:"url"`
}

type OutputFile struct {
	Path string `json:"path"`
	// +optional
	Parser string `json:"parser"`
	// +optional
	Label string `json:"label"`
}

type ActionOutput struct {
	OutputName string `json:"outputName"`
	// +optional
	Files []OutputFile `json:"files"`
}

type Output struct {
	Name string `json:"name"`
	// +optional
	HttpPostSpec HttpPost `json:"httpPostSpec"`
}

type ConstantIncreaseDecreaseRate struct {
	IncInterval int `json:"incInterval"`
	DecInterval int `json:"decInterval"`
	Max int `json:"max"`
	Min int `json:"min"`
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

/*
type RunOnce struct {
	// Name of ConfigMap that contains the yaml definition of the object to be
	// scaled.  If the object does not exist it will be created.
	// If the ConfigMap contains more than one object, all are created.
	SpecName string `json:"specName"`

	// How many instances of the object to create.  Multiple instances are all
	// created simultaneously (use the Run Action type to create multiple
	// instances non-simultaneously).
	Count int `json:"count"`
}

type Run struct {
	// Name of ConfigMap that contains the yaml definition of the object to be
	// scaled.  If the object does not exist it will be created.
	// If the ConfigMap contains more than one object, all are created.
	SpecName string `json:"specName"`

}

type Scale struct {
	// Used to select the object that will be scaled (object should already exist)
	// (Unimplemented)
	// +optional
	Selector metav1.LabelSelector `json:"selector,omitempty"`

	// Name of the object that will be scaled (object should already exist)
	// +optional
	Name string `json:"name,omitempty"`

	// Name of ConfigMap that contains the yaml definition of the object to be
	// scaled.  If the object does not exist it will be created.  The ConfigMap
	// should only contain one object.
	// +optional
	SpecName string `json:"specName,omitempty"`
}*/

type Snapshot struct {
	VolName string `json:"volName"`
	SnapshotClass string `json:"snapshotClass"`
}

type CreateObj struct {
	Workload string `json:"workload"`

	// +optional
	// +nullable
	VolName string `json:"volName"`

	// +optional
	// +nullable
	StorageClass string `json:"storageClass"`
}

type Action struct {
	Name string `json:"name"`

	// +optional
	//RunOnceSpec RunOnce `json:"runOnceSpec"`
	// +optional
	//RunSpec Run `json:"runSpec"`
	// +optional
	//ScaleSpec Scale `json:"scaleSpec"`

	// +optional
	CreateObjSpec CreateObj `json:"createObjSpec"`
	// +optional
	SnapshotSpec Snapshot `json:"snapshotSpec"`

	// +optional
	Outputs ActionOutput `json:"outputs"`

	// +optional
	// +nullable
	RateName string `json:"rateName"`
}

// BenchmarkSpec defines the desired state of Benchmark
type BenchmarkSpec struct {
	//Runtime	int `json:"runtime"`

	// Runtime, numactions, ...?
	// For each action have an exit condition? (or each rate?)
	// +optional
	StopAfter string `json:"stopAfter"`

	Actions []Action `json:"actions"`

	// +optional
	// +nullable
	Rates []Rate `json:"rates"`

	// +optional
	Outputs []Output `json:"outputs"`
}

type BenchmarkState string
const (
	Complete BenchmarkState = "Complete"
	Running BenchmarkState = "Running"
	Initializing BenchmarkState = "Initializing"
)

type BenchmarkCondition struct {
	// +optional
	// +nullable
	LastProbeTime metav1.Time `json:"lastProbeTime"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// +optional
	// +nullable
	Message string `json:"message"`
	// +optional
	// +nullable
	Reason string `json:"reason"`
	Status string `json:"status"`
	Type string `json:"type"`
}

// BenchmarkStatus defines the observed state of Benchmark
type BenchmarkStatus struct {
	State BenchmarkState `json:"state"`

	// +optional
	// +nullable
	CompletionTime metav1.Time `json:"completionTime"`

	CompletionTimeUnix int64 `json:"completionTimeUnix"`
	StartTimeUnix int64 `json:"startTimeUnix"`

	// This doesn't include RuneOnce actions
	RunningActions int `json:"runningActions"`
	RunningRates int `json:"runningRates"`

	Conditions []BenchmarkCondition `json:"conditions"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Benchmark is the Schema for the benchmarks API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=benchmarks,scope=Namespaced
type Benchmark struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BenchmarkSpec   `json:"spec,omitempty"`
	Status BenchmarkStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BenchmarkList contains a list of Benchmark
type BenchmarkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Benchmark `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Benchmark{}, &BenchmarkList{})
}
