package benchmark

import (
	"fmt"
	"context"
	"time"
	//"strings"
	//"encoding/json"

	"github.com/cnsbench/pkg/rates"
	"github.com/cnsbench/pkg/utils"
	"github.com/cnsbench/pkg/output"
	"github.com/cnsbench/pkg/objecttiming"

	cnsbench "github.com/cnsbench/pkg/apis/cnsbench/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"k8s.io/client-go/kubernetes/scheme"
	snapshotscheme "github.com/kubernetes-csi/external-snapshotter/v2/pkg/client/clientset/versioned/scheme"
)

var log = logf.Log.WithName("controller_benchmark")

type BenchmarkControllerState struct {
	// Send "true" to these when it's time to exit
	//ControlChannels map[string]chan bool
	//ActionControlChannels map[string]chan bool
	ActionControlChannel chan bool
	ControlChannel chan bool
	Actions []string
	Rates []string
	RunningObjs []utils.NameKind
}

// Add creates a new Benchmark Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	snapshotscheme.AddToScheme(scheme.Scheme)

	r := newReconciler(mgr)
	c, err := add(mgr, r)
	if err != nil {
		return err
	}

	r.controller = c
	return nil

	//return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
//func newReconciler(mgr manager.Manager) reconcile.Reconciler {
func newReconciler(mgr manager.Manager) *ReconcileBenchmark {
	return &ReconcileBenchmark{client: mgr.GetClient(), scheme: mgr.GetScheme(), state: make(map[string]*BenchmarkControllerState)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) (controller.Controller, error) {
	// Create a new controller
	c, err := controller.New("benchmark-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}

	// Watch for changes to primary resource Benchmark
	err = c.Watch(&source.Kind{Type: &cnsbench.Benchmark{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	return c, nil
}

// blank assignment to verify that ReconcileBenchmark implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileBenchmark{}

// ReconcileBenchmark reconciles a Benchmark object
type ReconcileBenchmark struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme

	controller controller.Controller

	state map[string]*BenchmarkControllerState
}

func (r *ReconcileBenchmark) cleanup(instance *cnsbench.Benchmark) error {
	log.Info("Deleting", "finalizers", instance.GetFinalizers())
	log.Info("status", "status", instance.Status)
	r.stopRoutines(instance)
	if utils.Contains(instance.GetFinalizers(), "RateFinalizer") {
		instance.SetFinalizers(utils.Remove(instance.GetFinalizers(), "RateFinalizer"))
		if err := r.client.Update(context.TODO(), instance); err != nil {
			log.Error(err, "Remove RateFinalizer")
			return err
		}
	}
	log.Info("Done Deleting", "finalizers", instance.GetFinalizers())

	r.state[instance.ObjectMeta.Name].RunningObjs = []utils.NameKind{}

	return nil
}

// Only starts actions that do not have any rates associated
func (r *ReconcileBenchmark) startActions(instance *cnsbench.Benchmark, actions []cnsbench.Action) error {
	instance.Status.RunningActions = 0
	for _, a := range actions {
		if a.RateName == "" {
			log.Info("Run once")
			if a.CreateObjSpec.Workload != "" {
				objs , err := r.RunWorkload(instance, a.CreateObjSpec, a.Name)
				if err != nil {
					log.Error(err, "Running spec")
					return err
				}
				for _, o := range objs {
					r.state[instance.ObjectMeta.Name].RunningObjs = append(r.state[instance.ObjectMeta.Name].RunningObjs, o)
				}
			} else {
				if err := r.runAction(instance, a); err != nil {
					log.Error(err, "Error running action")
				}
			}
		}
	}
	return nil
}

func (r *ReconcileBenchmark) startRates(instance *cnsbench.Benchmark) error {
	instance.Status.RunningRates = 0
	var c chan int
	for _, rate := range instance.Spec.Rates {
		if rate.ConstantRateSpec.Interval != 0 {
			c = r.createConstantRate(rate.ConstantRateSpec, r.state[instance.ObjectMeta.Name].ControlChannel)
		} else if rate.ConstantIncreaseDecreaseRateSpec.IncInterval != 0 {
			c = r.createConstantIncreaseDecreaseRate(rate.ConstantIncreaseDecreaseRateSpec, r.state[instance.ObjectMeta.Name].ControlChannel)
		} else {
			return fmt.Errorf("Unknown kind of rate")
		}
		go r.runActions(instance, c, r.state[instance.ObjectMeta.Name].ControlChannel, rate.Name)
		instance.Status.RunningRates += 1
	}

	if instance.Status.RunningRates > 0 {
		if !utils.Contains(instance.GetFinalizers(), "RateFinalizer") {
			instance.SetFinalizers(append(instance.GetFinalizers(), "RateFinalizer"))
		}
	}

	return nil
}

type WorkloadResult struct {
	ActionName string
	PodName string
	NodeName string
	TimeToCreate int64
	//Results map[string]interface{}
	Results string
}

func (r *ReconcileBenchmark) getTiming(name string, timings []map[string]interface{}) int64 {
	for _, t := range timings {
		if t["name"] == name {
			switch v := t["duration"].(type) {
			case int64:
				return t["duration"].(int64)
			default:
				log.Info("Duration not int64", "t", t, "type", v)
				return -1
			}
		}
	}
	return -1
}

func (r *ReconcileBenchmark) doOutputs(bm *cnsbench.Benchmark, startTime, completionTime, initCompletionTime int64) {
	log.Info("Do outputs")

	// TODO: how to specify url in benchmark spec?
	auditLogs, err := utils.GetAuditLogs(startTime, completionTime, "http://loadbalancer:9200")
	if err != nil {
		log.Error(err, "Error getting audit logs")
	}
	log.Info("auditlogs", "logs", len(auditLogs))
	// TODO: better way of specifying flags
	operationTimes, err := objecttiming.ParseLogs(auditLogs, 1)
	log.Info("ops", "ops", operationTimes)
	if err != nil {
		log.Error(err, "Error parsing audit logs")
	}

	allResults := []WorkloadResult{}
	for _, action := range bm.Spec.Actions {
		workloadResults := []WorkloadResult{}
		ls := &metav1.LabelSelector{}
		ls = metav1.AddLabelToSelector(ls, "actionname", action.Name)
		selector, err := metav1.LabelSelectorAsSelector(ls)
		if err != nil {
			log.Error(err, "Error making selector")
			continue
		}

		pods := &corev1.PodList{}
		if err := r.client.List(context.TODO(), pods, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
			log.Error(err, "Error getting pods")
			continue
		}
		for _, pod := range pods.Items {
			log.Info("Getting output", "pod", pod.Name, "action", action.Name)
			for _, c := range pod.Spec.Containers {
				if c.Name == "parser-container" {
					out, err := utils.ReadContainerLog(pod.Name, "parser-container")
					if err != nil {
						log.Error(err, "Reading pod output")
						continue
					}
					lastLine, err := utils.GetLastLine(out)
					if err != nil {
						log.Error(err, "Reading getting last line")
						continue
					}
					log.Info("Adding output to lists")
					workloadResults = append(workloadResults, WorkloadResult{action.Name, pod.Name, pod.Spec.NodeName, r.getTiming(pod.Name, operationTimes), lastLine})
					//allResults = append(allResults, WorkloadResult{action.Name, pod.Name, pod.Spec.NodeName, r.getTiming(pod.Name, operationTimes), lastLine})
					log.Info("Last line length", "len", len(lastLine))
					allResults = append(allResults, WorkloadResult{action.Name, pod.Name, pod.Spec.NodeName, -1, lastLine})
				}
			}
		}
		if action.Outputs.OutputName != "" {
			results := make(map[string]interface{})
			results["WorkloadResults"] = workloadResults
			if err := output.Output(results, action.Outputs.OutputName, bm, startTime, completionTime, initCompletionTime); err != nil {
				log.Error(err, "Error sending outputs")
			}
		}
	}

	if bm.Spec.AllResultsOutput != "" {
		log.Info("Output size", "size", len(allResults))
		results := make(map[string]interface{})
		results["AllResults"] = allResults
		if err := output.Output(results, bm.Spec.AllResultsOutput, bm, startTime, completionTime, initCompletionTime); err != nil {
			log.Error(err, "Error sending outputs")
		}
	}
}

func (r *ReconcileBenchmark) stopRoutines(instance *cnsbench.Benchmark) {
	instanceName := instance.ObjectMeta.Name
	for i := 0; i < instance.Status.RunningRates; i++ {
		// For every rate there's the routine for the rate itself and the routine
		// that listens for the rate and runs actions.  So send two messages for
		// each rate we have running
		r.state[instanceName].ControlChannel <- true
		r.state[instanceName].ControlChannel <- true
	}
}

func (r *ReconcileBenchmark) getCompletedPods(actions []cnsbench.Action, endruntime time.Time) (int, error) {
	complete := 0
	for _, a := range actions {
		ls := &metav1.LabelSelector{}
		ls = metav1.AddLabelToSelector(ls, "actionname", a.Name)
		ls = metav1.AddLabelToSelector(ls, "multiinstance", "true")

		selector, err := metav1.LabelSelectorAsSelector(ls)
		if err != nil {
			return -1, err
		}
		pods := &corev1.PodList{}
		if err := r.client.List(context.TODO(), pods, &client.ListOptions{Namespace: "default", LabelSelector: selector}); err != nil {
			return -1, err
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase == "Succeeded" {
				numContainers := len(pod.Status.ContainerStatuses)
				if numContainers > 0 && pod.Status.ContainerStatuses[numContainers-1].State.Terminated != nil {
					if !pod.Status.ContainerStatuses[numContainers-1].State.Terminated.FinishedAt.After(endruntime) {
						log.Info("Pod", "endtime", endruntime, "finshedat", pod.Status.ContainerStatuses[numContainers-1].State.Terminated.FinishedAt.Unix())
						complete += 1
					} else {
						log.Info("Too late!", "endtime", endruntime, "finshedat", pod.Status.ContainerStatuses[numContainers-1].State.Terminated.FinishedAt.Unix())
					}
				}
			}
		}
	}
	return complete, nil
}

func (r *ReconcileBenchmark) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Benchmark")

	// Fetch the Benchmark instance
	instance := &cnsbench.Benchmark{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Not found")
			return reconcile.Result{}, nil
		}
		log.Error(err, "Error getting instance")
		return reconcile.Result{}, err
	}
	instanceName := instance.ObjectMeta.Name

	// Is it being deleted?
	if instance.GetDeletionTimestamp() != nil {
		log.Info("Being deleted")
		if _, exists := r.state[instanceName]; exists {
			if err := r.cleanup(instance); err != nil {
				return reconcile.Result{}, err
			}
			delete(r.state, instance.ObjectMeta.Name)
			log.Info("Deleted from r.state", "", instance.ObjectMeta.Name)
		}
		return reconcile.Result{}, nil
	}

	// If we're Complete but not deleted yet, nothing to do but return
	if instance.Status.State == cnsbench.Complete {
		return reconcile.Result{}, nil
	}

	// If our per-Benchmark obj state doesn't exist, create it
	if _, exists := r.state[instanceName]; !exists {
		r.state[instanceName] = &BenchmarkControllerState{make(chan bool), make(chan bool), make([]string, 0), make([]string, 0), []utils.NameKind{}}
	}

	// XXX This shouldn't be necessary, but if optional arrays are omitted from
	// the object they get set to "null" which causes Updates to throw an error
	// We don't need to actually do the Update here, but setting these values
	// now will ensure if we do Update later on in this reconcile there won't be
	// an error
	if instance.Spec.Outputs == nil {
		instance.Spec.Outputs = []cnsbench.Output{}
	}
	for i := 0; i < len(instance.Spec.Actions); i++ {
		if instance.Spec.Actions[i].Outputs.Files == nil {
			instance.Spec.Actions[i].Outputs.Files = []cnsbench.OutputFile{}
		}
	}

	// if we're here, then we're either still running or haven't started yet
	if instance.Status.State == cnsbench.Running {
		// If we're running, and there's a runtime set, check if we've reached the runtime
		// And if not, check that we still have the correct number of workload instances running.

		if err := utils.CleanupScalePods(r.client); err != nil {
			log.Error(err, "Cleaning up scale pods")
		}

		runtimeEnd := time.Now()
		doneRuntime := false
		if instance.Spec.Runtime != "" {
			if runtime, err := time.ParseDuration(instance.Spec.Runtime); err != nil {
				log.Error(err, "Error parsing duration")
			} else {
				//log.Info("target completion time", "completion time", time.Unix(instance.Status.InitCompletionTimeUnix, 0).Add(runtime), "now", time.Now().Unix())
				// Can count runtime from init complete or start.  Only do from start for now...
				var startTime time.Time
				if true {
					startTime = instance.ObjectMeta.CreationTimestamp.Time
				} else {
					startTime = instance.Status.InitCompletionTime.Time
				}
				log.Info("target completion time", "completion time", startTime.Add(runtime), "now", time.Now().Unix())
				if time.Now().Before(startTime.Add(runtime)) {
					log.Info("Not done yet, reconciling instances")
					if nks, err := r.ReconcileInstances(instance, r.client, instance.Spec.Actions); err != nil {
						log.Error(err, "Error reconciling workload instances")
					} else {
						for _, nk := range nks {
							r.state[instanceName].RunningObjs = append(r.state[instanceName].RunningObjs, nk)
						}
					}
				} else {
					log.Info("Runtime done!")
					runtimeEnd = startTime.Add(runtime)
					doneRuntime = true
				}
			}
		}
		if instance.Spec.Runtime == "" || doneRuntime {
			// If we're running and there's no runtime set, check if the workloads are complete
			log.Info("Checking status...")
			complete, err := utils.CheckCompletion(r.client, r.state[instanceName].RunningObjs)
			if err != nil {
				log.Error(err, "Error checking Job status")
				return reconcile.Result{}, err
			} else if complete {
				log.Info("Pods are complete, doing outputs")
				instance.Status.NumCompletedObjs, _ = r.getCompletedPods(instance.Spec.Actions, runtimeEnd)
				r.doOutputs(instance, instance.ObjectMeta.CreationTimestamp.Unix(), time.Now().Unix(), instance.Status.InitCompletionTimeUnix)
				instance.Status.State = cnsbench.Complete
				instance.Status.CompletionTime = metav1.Now()
				instance.Status.CompletionTimeUnix = time.Now().Unix()
				instance.Status.StartTimeUnix = instance.ObjectMeta.CreationTimestamp.Unix()
				instance.Status.Conditions[0] = cnsbench.BenchmarkCondition{LastTransitionTime: metav1.Now(), Status: "True", Type: "Complete"}
				if err := r.client.Status().Update(context.TODO(), instance); err != nil {
					log.Error(err, "Updating instance")
					return reconcile.Result{}, err
				}
				log.Info("Updated status")
				if err := r.cleanup(instance); err != nil {
					log.Error(err, "Error cleaning up")
					return reconcile.Result{}, err
				}
			}
		}
	} else if instance.Status.State == cnsbench.Initializing {
		doneInit, err := utils.CheckInit(r.client, instance.Spec.Actions)
		if err != nil {
			log.Error(err, "Error checking init")
			return reconcile.Result{}, err
		}
		if instance.Spec.Runtime != "" {
			doneInit = true
		}
		if doneInit {
			err = r.startRates(instance)
			if err != nil {
				r.stopRoutines(instance)
				log.Error(err, "")
				return reconcile.Result{}, err
			}
			// if we're here we started everything successfully
			instance.Status.State = cnsbench.Running
			instance.Status.InitCompletionTime = metav1.Now()
			instance.Status.InitCompletionTimeUnix = time.Now().Unix()
			if err := r.client.Status().Update(context.TODO(), instance); err != nil {
				log.Error(err, "Updating instance")
				r.cleanup(instance)
				return reconcile.Result{}, err
			}
			if err := r.client.Update(context.TODO(), instance); err != nil {
				log.Error(err, "Updating instance")
				r.cleanup(instance)
				return reconcile.Result{}, err
			}

			if instance.Spec.Runtime != "" {
				if runtime, err := time.ParseDuration(instance.Spec.Runtime); err != nil {
					log.Error(err, "")
				} else {
					log.Info("runtime", "runtime", runtime)
					// If runtime should be started from benchmark creation rather than
					// when init completes, subtract current time - start time from runtime
					if true {
						elapsedTime := time.Now().Sub(instance.ObjectMeta.CreationTimestamp.Time)
						runtime -= elapsedTime
						log.Info("times", "runtime", runtime, "elapsed", elapsedTime)
					}
					return reconcile.Result{RequeueAfter: runtime}, nil
				}
			}
		} else {
			return reconcile.Result{RequeueAfter: time.Second*5}, nil
		}
	} else {
		// if we're not running and we're not complete, we must need to be started
		// Once each of the goroutines are started, the only way they exit is if we tell
		// them to via the control channel - i.e., even if they encounter errors they just
		// keep going
		log.Info("", "", instance.Spec)
		err = r.startActions(instance, instance.Spec.Actions)
		if err != nil {
			log.Error(err, "")
			return reconcile.Result{}, err
		}
		// if we're here we started everything successfully
		instance.Status.State = cnsbench.Initializing
		instance.Status.Conditions = []cnsbench.BenchmarkCondition{{LastTransitionTime: metav1.Now(), Status: "False", Type: "Complete"}}
		if err := r.client.Status().Update(context.TODO(), instance); err != nil {
			log.Error(err, "Updating instance")
			r.cleanup(instance)
			return reconcile.Result{}, err
		}
		if err := r.client.Update(context.TODO(), instance); err != nil {
			log.Error(err, "Updating instance")
			r.cleanup(instance)
			return reconcile.Result{}, err
		}
		// Want to check right away if we're done initializing so requeue
		return reconcile.Result{Requeue: true}, nil
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileBenchmark) createConstantRate(spec cnsbench.ConstantRate, c chan bool) chan int {
	log.Info("Launching SingleRate")
	consumerChan := make(chan int)
	rate := rates.Rate{consumerChan, c}
	go rate.SingleRate(rates.ConstTimer{spec.Interval})
	return consumerChan
}

func (r *ReconcileBenchmark) createConstantIncreaseDecreaseRate(spec cnsbench.ConstantIncreaseDecreaseRate, c chan bool) chan int {
	log.Info("Launching IncDecRate")
	consumerChan := make(chan int)
	rate := rates.Rate{consumerChan, c}
	go rate.IncDecRate(rates.ConstTimer{spec.IncInterval}, rates.ConstTimer{spec.DecInterval}, spec.Min, spec.Max)
	return consumerChan
}

func (r *ReconcileBenchmark) runActions(bm *cnsbench.Benchmark, rateCh chan int, controlCh chan bool, rateName string) {
	for {
		select {
		case <- controlCh:
			log.Info("Exiting run goroutine", "name", rateName)
			return
		case n:= <- rateCh:
			log.Info("Got rate!", "n", n)
			for _, a := range bm.Spec.Actions {
				if a.RateName == rateName {
					if err := r.runAction(bm, a); err != nil {
						log.Error(err, "Error running action")
					}
				}
			}
		}
	}
}

func (r *ReconcileBenchmark) runAction(bm *cnsbench.Benchmark, a cnsbench.Action) error {
	log.Info("Running action", "name", a, "deletespec", metav1.FormatLabelSelector(&a.DeleteSpec.Selector))
	if a.SnapshotSpec.SnapshotClass != "" {
		return r.CreateSnapshot(bm, a.SnapshotSpec, a.Name)
	} else if metav1.FormatLabelSelector(&a.DeleteSpec.Selector) != "" &&
		  metav1.FormatLabelSelector(&a.DeleteSpec.Selector) != "<none>" {
		return r.DeleteObj(bm, a.DeleteSpec)
	} else if a.ScaleSpec.ObjName != "" {
		return r.ScaleObj(bm, a.ScaleSpec)
	} else {
		log.Info("Unknown kind of action")
	}
	return nil
}
