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

package controllers

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cnsbench "github.com/cnsbench/cnsbench/api/v1alpha1"

	"github.com/cnsbench/cnsbench/pkg/output"
	"github.com/cnsbench/cnsbench/pkg/rates"
	"github.com/cnsbench/cnsbench/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const LIBRARY_NAMESPACE = "cnsbench-library"

// BenchmarkReconciler reconciles a Benchmark object
type BenchmarkReconciler struct {
	client.Client
	Log             logr.Logger
	Scheme          *runtime.Scheme
	controlChannels map[string](chan bool)
	controller      controller.Controller
	ScriptsDir      string
}

// +kubebuilder:rbac:groups=cnsbench.example.com,resources=benchmarks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cnsbench.example.com,resources=benchmarks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cnsbench.example.com,resources=benchmarks/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,namespace=default,resources=services/finalizers;services;pods;endpoints;persistentvolumeclaims;events;configmaps;secrets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=apps,namespace=default,resources=deployments;daemonsets;replicasets;statefulsets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=batch,namespace=default,resources=jobs,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=core,resources=services/finalizers;services;pods;endpoints;persistentvolumeclaims;events;configmaps;secrets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs=create;delete;get;list;patch;update;watch

func (r *BenchmarkReconciler) metric(instance *cnsbench.Benchmark, metricType string, metrics ...string) {
	metrics = append([]string{"type", metricType}, metrics...)
	output.Metric(instance.Spec.Outputs, instance.Spec.MetricsOutput, instance.ObjectMeta.Name, metrics...)
}

func (r *BenchmarkReconciler) cleanup(instance *cnsbench.Benchmark) error {
	r.Log.Info("Deleting", "finalizers", instance.GetFinalizers())
	r.Log.Info("status", "status", instance.Status)
	r.stopRoutines(instance)
	if utils.Contains(instance.GetFinalizers(), "RateFinalizer") {
		instance.SetFinalizers(utils.Remove(instance.GetFinalizers(), "RateFinalizer"))
		if err := r.Client.Update(context.TODO(), instance); err != nil {
			r.Log.Error(err, "Remove RateFinalizer")
			return err
		}
	}
	r.Log.Info("Done Deleting", "finalizers", instance.GetFinalizers())

	return nil
}

func (r *BenchmarkReconciler) createVolumes(instance *cnsbench.Benchmark, vols []cnsbench.Volume) {
	for _, v := range vols {
		r.CreateVolume(instance, v)
	}
}

// Only starts workloads that do not have any rates associated
func (r *BenchmarkReconciler) startWorkloads(instance *cnsbench.Benchmark, workloads []cnsbench.Workload) error {
	instance.Status.RunningWorkloads = 0
	for _, a := range workloads {
		if err := r.RunWorkload(instance, a, a.Name); err != nil {
			return err
		}
	}
	return nil
}

func (r *BenchmarkReconciler) startRates(instance *cnsbench.Benchmark) error {
	instance.Status.RunningRates = 0
	var c chan int
	r.controlChannels[instance.ObjectMeta.Name] = make(chan bool)
	for _, rate := range instance.Spec.Rates {
		if rate.ConstantRateSpec.Interval != 0 {
			c = r.createConstantRate(rate.ConstantRateSpec, r.controlChannels[instance.ObjectMeta.Name])
		} else if rate.ConstantIncreaseDecreaseRateSpec.IncInterval != 0 {
			c = r.createConstantIncreaseDecreaseRate(rate.ConstantIncreaseDecreaseRateSpec, r.controlChannels[instance.ObjectMeta.Name])
		} else {
			unknownRate := errors.New("Unknown rate")
			r.Log.Error(unknownRate, rate.Name)
			return unknownRate
		}
		go r.runControlOps(instance, c, r.controlChannels[instance.ObjectMeta.Name], rate.Name)
		instance.Status.RunningRates += 1
	}

	if instance.Status.RunningRates > 0 {
		if !utils.Contains(instance.GetFinalizers(), "RateFinalizer") {
			instance.SetFinalizers(append(instance.GetFinalizers(), "RateFinalizer"))
		}
	}

	return nil
}

func (r *BenchmarkReconciler) doOutputs(bm *cnsbench.Benchmark, startTime, completionTime, initCompletionTime int64) {
	r.Log.Info("Do outputs")

	if err := output.Output(bm.Spec.MetadataOutput, bm, startTime, completionTime, initCompletionTime); err != nil {
		r.Log.Error(err, "Error sending outputs")
	}
}

func (r *BenchmarkReconciler) stopRoutines(instance *cnsbench.Benchmark) {
	instanceName := instance.ObjectMeta.Name
	for i := 0; i < instance.Status.RunningRates; i++ {
		// For every rate there's the routine for the rate itself and the routine
		// that listens for the rate and runs actions.  So send two messages for
		// each rate we have running
		r.controlChannels[instanceName] <- true
		r.controlChannels[instanceName] <- true
	}
}

func (r *BenchmarkReconciler) getCompletedPods(workloads []cnsbench.Workload, endruntime time.Time) (int, error) {
	complete := 0
	for _, a := range workloads {
		ls := &metav1.LabelSelector{}
		ls = metav1.AddLabelToSelector(ls, "workloadname", a.Name)
		ls = metav1.AddLabelToSelector(ls, "duplicate", "true")

		selector, err := metav1.LabelSelectorAsSelector(ls)
		if err != nil {
			return -1, err
		}
		pods := &corev1.PodList{}
		if err := r.Client.List(context.TODO(), pods, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
			return -1, err
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase == "Succeeded" {
				numContainers := len(pod.Status.ContainerStatuses)
				if numContainers > 0 && pod.Status.ContainerStatuses[numContainers-1].State.Terminated != nil {
					if !pod.Status.ContainerStatuses[numContainers-1].State.Terminated.FinishedAt.After(endruntime) {
						r.Log.Info("Pod", "endtime", endruntime, "finshedat", pod.Status.ContainerStatuses[numContainers-1].State.Terminated.FinishedAt.Unix())
						complete += 1
					} else {
						r.Log.Info("Too late!", "endtime", endruntime, "finshedat", pod.Status.ContainerStatuses[numContainers-1].State.Terminated.FinishedAt.Unix())
					}
				}
			}
		}
	}
	return complete, nil
}

func (r *BenchmarkReconciler) addDefaultOutputs(instance *cnsbench.Benchmark) {
	endpoints := map[string]string{
		"defaultWorkloadsOutput": "workloads",
		"defaultMetadataOutput":  "metadata",
		"defaultMetricsOutput":   "metrics",
	}

	for outputName, endpoint := range endpoints {
		found := false
		for _, o := range instance.Spec.Outputs {
			if o.Name == outputName {
				found = true
			}
		}
		if !found {
			newOutput := cnsbench.Output{
				Name: outputName,
				HttpPostSpec: cnsbench.HttpPost{
					URL: "http://cnsbench-output-collector.cnsbench-system.svc.cluster.local:8888/" + endpoint + "/",
				},
			}
			instance.Spec.Outputs = append(instance.Spec.Outputs, newOutput)
		}
	}
}

func (r *BenchmarkReconciler) updateInstanceStatus(instance *cnsbench.Benchmark) error {
	if err := r.Client.Status().Update(context.TODO(), instance); err != nil {
		r.Log.Error(err, "Updating instance")
		r.cleanup(instance)
		return err
	}
	return nil
}

func (r *BenchmarkReconciler) updateInstance(instance *cnsbench.Benchmark) error {
	if err := r.Client.Update(context.TODO(), instance); err != nil {
		r.Log.Error(err, "Updating instance")
		r.cleanup(instance)
		return err
	}
	return nil
}

func (r *BenchmarkReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Reconciling Benchmark")

	// Fetch the Benchmark instance
	instance := &cnsbench.Benchmark{}
	err := r.Client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			r.Log.Info("Not found")
			return ctrl.Result{}, nil
		}
		r.Log.Error(err, "Error getting instance")
		return ctrl.Result{}, err
	}
	instanceName := instance.ObjectMeta.Name

	// Is it being deleted?
	if instance.GetDeletionTimestamp() != nil {
		r.Log.Info("Being deleted")
		if _, exists := r.controlChannels[instanceName]; exists {
			if err := r.cleanup(instance); err != nil {
				return ctrl.Result{}, err
			}
			delete(r.controlChannels, instance.ObjectMeta.Name)
			r.Log.Info("Deleted from r.state", "", instance.ObjectMeta.Name)
		}
		return ctrl.Result{}, nil
	}

	// If we're Complete but not deleted yet, nothing to do but return
	if instance.Status.State == cnsbench.Complete {
		return ctrl.Result{}, nil
	}

	// if we're here, then we're either still running or haven't started yet
	if instance.Status.State == cnsbench.Running {
		// If we're running, and there's a runtime set, check if we've reached the runtime
		// And if not, check that we still have the correct number of workload instances running.
		runtimeEnd := time.Now()
		if instance.Spec.Runtime != "" && time.Now().Before(instance.Status.TargetCompletionTime.Time) {
			r.Log.Info("Before target completion time", "completion time", instance.Status.TargetCompletionTime, "now", time.Now().Unix())
			err = r.ReconcileInstances(instance, instance.Spec.Workloads)
			return ctrl.Result{}, err
		} else if instance.Spec.Runtime == "" {
			// If we're running and there's no runtime set, check if the workloads are complete
			r.Log.Info("Checking status...")
			complete := true
			for _, w := range instance.Spec.Workloads {
				workloadsComplete, _, err := CountCompletions(r.Client, w.Name)
				if err != nil {
					r.Log.Error(err, "Error checking Job status")
					return ctrl.Result{}, err
				} else if workloadsComplete < w.Count {
					complete = false
					break
				}
			}
			// No runtime set, not complete, just return
			if !complete {
				return ctrl.Result{}, err
			}
		}

		// Either runtime is set and we've reached it, or it's not set but all workloads are complete:
		r.Log.Info("Pods are complete, doing outputs")
		instance.Status.NumCompletedObjs, _ = r.getCompletedPods(instance.Spec.Workloads, runtimeEnd)
		r.doOutputs(instance, instance.ObjectMeta.CreationTimestamp.Unix(), time.Now().Unix(), instance.Status.InitCompletionTimeUnix)

		instance.Status.State = cnsbench.Complete
		instance.Status.CompletionTime = metav1.Now()
		instance.Status.CompletionTimeUnix = time.Now().Unix()
		instance.Status.StartTimeUnix = instance.ObjectMeta.CreationTimestamp.Unix()
		instance.Status.Conditions[0] = cnsbench.BenchmarkCondition{LastTransitionTime: metav1.Now(), Status: "True", Type: "Complete"}

		if err := r.updateInstanceStatus(instance); err != nil {
			return ctrl.Result{}, err
		}
		r.Log.Info("Updated status")
		if err := r.cleanup(instance); err != nil {
			return ctrl.Result{}, err
		}
	} else if instance.Status.State == cnsbench.Initializing {
		doneInit, err := CheckInit(r.Client, instance.Spec.Workloads)
		if err != nil {
			r.Log.Error(err, "Error checking init")
			return ctrl.Result{}, err
		} else if !doneInit {
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}

		// init done, start rates and reconcile workloads:
		err = r.startRates(instance)
		if err != nil {
			r.stopRoutines(instance)
			return ctrl.Result{}, err
		}

		instance.Status.State = cnsbench.Running
		instance.Status.InitCompletionTime = metav1.Now()
		instance.Status.InitCompletionTimeUnix = time.Now().Unix()

		if instance.Spec.Runtime != "" {
			if runtime, err := time.ParseDuration(instance.Spec.Runtime); err != nil {
				r.Log.Error(err, "Error parsing duration")
			} else {
				instance.Status.TargetCompletionTime = metav1.NewTime(instance.Status.InitCompletionTime.Time.Add(runtime))
			}
		}

		if err := r.updateInstanceStatus(instance); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.updateInstance(instance); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// if we're not running and we're not complete, we must need to be started
		// Once each of the goroutines are started, the only way they exit is if we tell
		// them to via the control channel - i.e., even if they encounter errors they just
		// keep going
		r.Log.Info("", "", instance.Spec)

		r.addDefaultOutputs(instance)

		r.createVolumes(instance, instance.Spec.Volumes)
		if err = r.startWorkloads(instance, instance.Spec.Workloads); err != nil {
			return ctrl.Result{}, err
		}

		// if we're here we started everything successfully
		instance.Status.State = cnsbench.Initializing
		instance.Status.Conditions = []cnsbench.BenchmarkCondition{{LastTransitionTime: metav1.Now(), Status: "False", Type: "Complete"}}

		if err := r.updateInstanceStatus(instance); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.updateInstance(instance); err != nil {
			return ctrl.Result{}, err
		}

		// Want to check right away if we're done initializing so requeue
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// This will create the rate and run it in a separate goroutine.  Returns the
// channel that the rate will trigger on
func (r *BenchmarkReconciler) createConstantRate(spec cnsbench.ConstantRate, c chan bool) chan int {
	r.Log.Info("Launching SingleRate")
	consumerChan := make(chan int)
	rate := rates.Rate{Consumer: consumerChan, ControlChannel: c}
	go rate.SingleRate(rates.ConstTimer{Interval: spec.Interval})
	return consumerChan
}

// This will create the rate and run it in a separate goroutine.  Returns the
// channel that the rate will trigger on
func (r *BenchmarkReconciler) createConstantIncreaseDecreaseRate(spec cnsbench.ConstantIncreaseDecreaseRate, c chan bool) chan int {
	r.Log.Info("Launching IncDecRate")
	consumerChan := make(chan int)
	rate := rates.Rate{Consumer: consumerChan, ControlChannel: c}
	go rate.IncDecRate(rates.ConstTimer{Interval: spec.IncInterval}, rates.ConstTimer{Interval: spec.DecInterval}, spec.Min, spec.Max)
	return consumerChan
}

// This is triggered by a rate via the rateCh channel
func (r *BenchmarkReconciler) runControlOps(bm *cnsbench.Benchmark, rateCh chan int, controlCh chan bool, rateName string) {
	for {
		select {
		case <-controlCh:
			r.Log.Info("Exiting run goroutine", "name", rateName)
			return
		case n := <-rateCh:
			r.Log.Info("Got rate!", "n", n)
			r.metric(bm, "rateFired", "rateName", rateName, "n", strconv.Itoa(n))
			for _, a := range bm.Spec.Volumes {
				if a.RateName == rateName {
					r.CreateVolume(bm, a)
				}
			}
			for _, a := range bm.Spec.Workloads {
				if a.RateName == rateName {
					if err := r.RunWorkload(bm, a, a.Name); err != nil {
						r.Log.Error(err, "Running spec")
					}
				}
			}
			for _, a := range bm.Spec.ControlOperations {
				if a.RateName == rateName {
					if err := r.runControlOp(bm, a); err != nil {
						r.Log.Error(err, "Error running action")
					}
				}
			}
		}
	}
}

func (r *BenchmarkReconciler) runControlOp(bm *cnsbench.Benchmark, a cnsbench.ControlOperation) error {
	r.Log.Info("Running action", "name", a, "deletespec", metav1.FormatLabelSelector(&a.DeleteSpec.Selector))
	if a.SnapshotSpec.SnapshotClass != "" {
		return r.CreateSnapshot(bm, a.SnapshotSpec, a.Name)
	} else if metav1.FormatLabelSelector(&a.DeleteSpec.Selector) != "" &&
		metav1.FormatLabelSelector(&a.DeleteSpec.Selector) != "<none>" {
		return r.DeleteObj(bm, a.DeleteSpec)
	} else if a.ScaleSpec.ObjName != "" {
		return r.ScaleObj(bm, a.ScaleSpec)
	} else {
		r.Log.Info("Unknown kind of action")
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BenchmarkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.controlChannels = make(map[string](chan bool))
	var err error
	r.controller, err = ctrl.NewControllerManagedBy(mgr).
		For(&cnsbench.Benchmark{}).
		Build(r)
	return err
}
