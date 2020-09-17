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

// Snapshots and deletions can operate on an individual object or a selector
// if a selector, then there may be multiple objects that match - should
// specify different policies for deciding which object to delete, e.g.
// "newest", "oldest", "random", ???
type Snapshot struct {
	ActionName string `json:"actionName"`
	SnapshotClass string `json:"snapshotClass"`
}

type Delete struct {
	//ObjName string `json:"objName"`
	Selector metav1.LabelSelector `json:"selector"`
}

// TODO: need a way of specifying how to scale - up or down, and by how much
type Scale struct {
	ObjName string `json:"objName"`
	ScriptConfigMap string `json:"scriptConfigMap"`
}

type CreateObj struct {
	Workload string `json:"workload"`

	// +optional
	// +nullable
	Vars map[string]string `json:"vars"`

	// +optional
	// +nullable
	//VolName string `json:"volName"`

	// +optional
	// +nullable
	//StorageClass string `json:"storageClass"`

	// +optional
	// +nullable
	//Config string `json:"config"`

	// +optional
	// +nullable
	Count int `json:"count"`

	// +optional
	// +nullable
	SyncStart bool `json:"syncStart"`

	// +optional
	// +nullable
	//SameVolume bool `json:"sameVolume"`

	// +optional
	// +nullable
	SyncGroup string `json:"syncGroup"`
}

type Action struct {
	Name string `json:"name"`

	// +optional
	//ScaleSpec Scale `json:"scaleSpec"`

	// +optional
	// +nullable
	CreateObjSpec CreateObj `json:"createObjSpec"`
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
	Outputs ActionOutput `json:"outputs"`

	// +optional
	// +nullable
	RateName string `json:"rateName"`
}

// BenchmarkSpec defines the desired state of Benchmark
type BenchmarkSpec struct {
	// +optional
	Runtime	string `json:"runtime"`

	Actions []Action `json:"actions"`

	// +optional
	// +nullable
	Rates []Rate `json:"rates"`

	// +optional
	// +nullable
	AllResultsOutput string `json:"allResultsOutput"`

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

	// +optional
	// +nullable
	InitCompletionTime metav1.Time `json:"initCompletionTime"`

	CompletionTimeUnix int64 `json:"completionTimeUnix"`
	StartTimeUnix int64 `json:"startTimeUnix"`
	InitCompletionTimeUnix int64 `json:"initCompletionTimeUnix"`

	NumCompletedObjs int `json:"numCompletedObjs"`

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
