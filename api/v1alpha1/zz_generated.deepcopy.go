// +build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ActionOutput) DeepCopyInto(out *ActionOutput) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ActionOutput.
func (in *ActionOutput) DeepCopy() *ActionOutput {
	if in == nil {
		return nil
	}
	out := new(ActionOutput)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Benchmark) DeepCopyInto(out *Benchmark) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Benchmark.
func (in *Benchmark) DeepCopy() *Benchmark {
	if in == nil {
		return nil
	}
	out := new(Benchmark)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Benchmark) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BenchmarkCondition) DeepCopyInto(out *BenchmarkCondition) {
	*out = *in
	in.LastProbeTime.DeepCopyInto(&out.LastProbeTime)
	in.LastTransitionTime.DeepCopyInto(&out.LastTransitionTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BenchmarkCondition.
func (in *BenchmarkCondition) DeepCopy() *BenchmarkCondition {
	if in == nil {
		return nil
	}
	out := new(BenchmarkCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BenchmarkList) DeepCopyInto(out *BenchmarkList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Benchmark, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BenchmarkList.
func (in *BenchmarkList) DeepCopy() *BenchmarkList {
	if in == nil {
		return nil
	}
	out := new(BenchmarkList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *BenchmarkList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BenchmarkSpec) DeepCopyInto(out *BenchmarkSpec) {
	*out = *in
	if in.Volumes != nil {
		in, out := &in.Volumes, &out.Volumes
		*out = make([]Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Workloads != nil {
		in, out := &in.Workloads, &out.Workloads
		*out = make([]Workload, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ControlOperations != nil {
		in, out := &in.ControlOperations, &out.ControlOperations
		*out = make([]ControlOperation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Rates != nil {
		in, out := &in.Rates, &out.Rates
		*out = make([]Rate, len(*in))
		copy(*out, *in)
	}
	if in.Outputs != nil {
		in, out := &in.Outputs, &out.Outputs
		*out = make([]Output, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BenchmarkSpec.
func (in *BenchmarkSpec) DeepCopy() *BenchmarkSpec {
	if in == nil {
		return nil
	}
	out := new(BenchmarkSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BenchmarkStatus) DeepCopyInto(out *BenchmarkStatus) {
	*out = *in
	in.CompletionTime.DeepCopyInto(&out.CompletionTime)
	in.InitCompletionTime.DeepCopyInto(&out.InitCompletionTime)
	in.TargetCompletionTime.DeepCopyInto(&out.TargetCompletionTime)
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]BenchmarkCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BenchmarkStatus.
func (in *BenchmarkStatus) DeepCopy() *BenchmarkStatus {
	if in == nil {
		return nil
	}
	out := new(BenchmarkStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConstantIncreaseDecreaseRate) DeepCopyInto(out *ConstantIncreaseDecreaseRate) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConstantIncreaseDecreaseRate.
func (in *ConstantIncreaseDecreaseRate) DeepCopy() *ConstantIncreaseDecreaseRate {
	if in == nil {
		return nil
	}
	out := new(ConstantIncreaseDecreaseRate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConstantRate) DeepCopyInto(out *ConstantRate) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConstantRate.
func (in *ConstantRate) DeepCopy() *ConstantRate {
	if in == nil {
		return nil
	}
	out := new(ConstantRate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ControlOperation) DeepCopyInto(out *ControlOperation) {
	*out = *in
	out.SnapshotSpec = in.SnapshotSpec
	out.ScaleSpec = in.ScaleSpec
	in.DeleteSpec.DeepCopyInto(&out.DeleteSpec)
	out.Outputs = in.Outputs
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ControlOperation.
func (in *ControlOperation) DeepCopy() *ControlOperation {
	if in == nil {
		return nil
	}
	out := new(ControlOperation)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Delete) DeepCopyInto(out *Delete) {
	*out = *in
	in.Selector.DeepCopyInto(&out.Selector)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Delete.
func (in *Delete) DeepCopy() *Delete {
	if in == nil {
		return nil
	}
	out := new(Delete)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HttpPost) DeepCopyInto(out *HttpPost) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HttpPost.
func (in *HttpPost) DeepCopy() *HttpPost {
	if in == nil {
		return nil
	}
	out := new(HttpPost)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Output) DeepCopyInto(out *Output) {
	*out = *in
	out.HttpPostSpec = in.HttpPostSpec
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Output.
func (in *Output) DeepCopy() *Output {
	if in == nil {
		return nil
	}
	out := new(Output)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OutputFile) DeepCopyInto(out *OutputFile) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OutputFile.
func (in *OutputFile) DeepCopy() *OutputFile {
	if in == nil {
		return nil
	}
	out := new(OutputFile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Rate) DeepCopyInto(out *Rate) {
	*out = *in
	out.ConstantRateSpec = in.ConstantRateSpec
	out.ConstantIncreaseDecreaseRateSpec = in.ConstantIncreaseDecreaseRateSpec
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Rate.
func (in *Rate) DeepCopy() *Rate {
	if in == nil {
		return nil
	}
	out := new(Rate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Scale) DeepCopyInto(out *Scale) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Scale.
func (in *Scale) DeepCopy() *Scale {
	if in == nil {
		return nil
	}
	out := new(Scale)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Snapshot) DeepCopyInto(out *Snapshot) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Snapshot.
func (in *Snapshot) DeepCopy() *Snapshot {
	if in == nil {
		return nil
	}
	out := new(Snapshot)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Volume) DeepCopyInto(out *Volume) {
	*out = *in
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Volume.
func (in *Volume) DeepCopy() *Volume {
	if in == nil {
		return nil
	}
	out := new(Volume)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Workload) DeepCopyInto(out *Workload) {
	*out = *in
	if in.Vars != nil {
		in, out := &in.Vars, &out.Vars
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.OutputFiles != nil {
		in, out := &in.OutputFiles, &out.OutputFiles
		*out = make([]OutputFile, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Workload.
func (in *Workload) DeepCopy() *Workload {
	if in == nil {
		return nil
	}
	out := new(Workload)
	in.DeepCopyInto(out)
	return out
}
