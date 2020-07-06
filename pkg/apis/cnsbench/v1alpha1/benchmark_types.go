package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

	RateName string `json:"rateName"`
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

	RateName string `json:"rateName"`
}

type Action struct {
	Name string `json:"name"`

	// +optional
	RunOnceSpec RunOnce `json:"runOnceSpec"`
	// +optional
	RunSpec Run `json:"runSpec"`
	// +optional
	ScaleSpec Scale `json:"scaleSpec"`
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
	Rates []Rate `json:"rates"`
}

type BenchmarkState string
const (
	Complete BenchmarkState = "Complete"
	Running BenchmarkState = "Running"
	Initializing BenchmarkState = "Initializing"
)

// BenchmarkStatus defines the observed state of Benchmark
type BenchmarkStatus struct {
	State BenchmarkState `json:"state"`

	// This doesn't include RuneOnce actions
	RunningActions int `json:"runningActions"`
	RunningRates int `json:"runningRates"`
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
