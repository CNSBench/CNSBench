package benchmark

import (
	"fmt"
	"context"
	"time"
	"strings"
	"encoding/json"

	"github.com/cnsbench/pkg/rates"
	"github.com/cnsbench/pkg/utils"
	"github.com/cnsbench/pkg/output"

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

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}

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
	if contains(instance.GetFinalizers(), "RateFinalizer") {
		instance.SetFinalizers(remove(instance.GetFinalizers(), "RateFinalizer"))
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
		if !contains(instance.GetFinalizers(), "RateFinalizer") {
			//instance.SetFinalizers(append(instance.GetFinalizers(), "RateFinalizer"))
		}
	}

	return nil
}

type WorkloadResult struct {
	PodName string
	NodeName string
	Results map[string]interface{}
}

func (r *ReconcileBenchmark) doOutputs(bm *cnsbench.Benchmark, startTime int64, completionTime int64) {
	log.Info("Do outputs")
	results := make(map[string]interface{})

	workloadResults := []WorkloadResult{}

	for _, action := range bm.Spec.Actions {
		pods := &corev1.PodList{}
		opts := []client.ListOption {
			client.InNamespace("default"),
		}
		if err := r.client.List(context.TODO(), pods, opts...); err != nil {
			log.Error(err, "Error getting pods")
		} else {
			for _, pod := range pods.Items {
				foundParserContainer := false
				for _, c := range pod.Spec.Containers {
					if c.Name == "parser-container" {
						foundParserContainer = true
					}
				}
				if !foundParserContainer {
					continue
				}

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
				var r map[string]interface{}
				if err := json.NewDecoder(strings.NewReader(lastLine)).Decode(&r); err != nil {
					log.Info("out", "out", out)
					log.Error(err, "Error decoding result")
					continue
				}
				workloadResults = append(workloadResults, WorkloadResult{pod.Name, pod.Spec.NodeName, r})
			}
		}
		results["WorkloadResults"] = workloadResults
		if err := output.Output(results, action.Outputs.OutputName, bm, startTime, completionTime); err != nil {
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

func (r *ReconcileBenchmark) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Benchmark")

	// Fetch the Benchmark instance
	instance := &cnsbench.Benchmark{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		log.Error(err, "Error getting instance")
		return reconcile.Result{}, err
	}
	instanceName := instance.ObjectMeta.Name

	// Is it being deleted?
	if instance.GetDeletionTimestamp() != nil {
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
		// If we're running, check to see if we're complete
		log.Info("Checking status...")
		complete, err := utils.CheckCompletion(r.client, r.state[instanceName].RunningObjs)
		if err != nil {
			log.Error(err, "Error checking Job status")
			return reconcile.Result{}, err
		} else if complete {
			log.Info("Pods are complete, doing outputs")
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
			r.doOutputs(instance, instance.ObjectMeta.CreationTimestamp.Unix(), time.Now().Unix())
			if err := r.cleanup(instance); err != nil {
				log.Error(err, "Error cleaning up")
				return reconcile.Result{}, err
			}
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
		err = r.startRates(instance)
		if err != nil {
			r.stopRoutines(instance)
			log.Error(err, "")
			return reconcile.Result{}, err
		}
		// if we're here we started everything successfully
		instance.Status.State = cnsbench.Running
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
	log.Info("Running action", "name", a)
	if a.SnapshotSpec.VolName != "" {
		return r.CreateSnapshot(bm, a.SnapshotSpec)
	} else if a.DeleteSpec.ObjName != "" {
		return r.DeleteObj(bm, a.DeleteSpec)
	} else if a.ScaleSpec.ObjName != "" {
		return r.ScaleObj(bm, a.ScaleSpec)
	} else {
		log.Info("Unknown kind of action")
	}
	return nil
}
