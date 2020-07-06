package benchmark

import (
	"fmt"
	"context"
	//"reflect"
	"strconv"
	"net/http"
	"bytes"
	"io"
	"io/ioutil"
	"encoding/json"

	"github.com/cnsbench/pkg/rates"

	cnsbenchv1alpha1 "github.com/cnsbench/pkg/apis/cnsbench/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil" 
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"k8s.io/client-go/kubernetes/scheme"
	//"k8s.io/client-go/pkg/api"
	//_ "k8s.io/client-go/pkg/api/install"
	//_ "k8s.io/client-go/pkg/apis/extensions/install"
	snapshotscheme "github.com/kubernetes-csi/external-snapshotter/v2/pkg/client/clientset/versioned/scheme"
)

type NameKind struct {
	Name string
	Kind string
}

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
	//RateControlChannels map[string]chan bool
	//ActionControlChannels map[string]chan bool
	ActionControlChannel chan bool
	RateControlChannel chan bool
	Actions []string
	Rates []string
	StopAfterName string
	StopAfterKind string
}

// Add creates a new Benchmark Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	snapshotscheme.AddToScheme(scheme.Scheme)

	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileBenchmark{client: mgr.GetClient(), scheme: mgr.GetScheme(), state: make(map[string]*BenchmarkControllerState)}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("benchmark-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Benchmark
	err = c.Watch(&source.Kind{Type: &cnsbenchv1alpha1.Benchmark{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Benchmark
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType: &cnsbenchv1alpha1.Benchmark{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Benchmark
	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: false,
		OwnerType: &cnsbenchv1alpha1.Benchmark{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileBenchmark implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileBenchmark{}

// ReconcileBenchmark reconciles a Benchmark object
type ReconcileBenchmark struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme

	state map[string]*BenchmarkControllerState
}

func (r *ReconcileBenchmark) cleanup(instance *cnsbenchv1alpha1.Benchmark) error {
	log.Info("Deleting", "finalizers", instance.GetFinalizers())
	log.Info("status", "status", instance.Status)
	if contains(instance.GetFinalizers(), "RateFinalizer") {
		log.Info("Rate finalizer set, stopping goroutines and removing finalizer")
		for i := 0; i < instance.Status.RunningRates; i++ {
			r.state[instance.ObjectMeta.Name].RateControlChannel <- true
		}
		instance.SetFinalizers(remove(instance.GetFinalizers(), "RateFinalizer"))
		if err := r.client.Update(context.TODO(), instance); err != nil {
			log.Error(err, "Remove RateFinalizer")
			return err
		}
	}
	if contains(instance.GetFinalizers(), "ActionFinalizer") {
		log.Info("Action finalizer set, stopping goroutines and removing finalizer")
		for i := 0; i < instance.Status.RunningActions; i++ {
			r.state[instance.ObjectMeta.Name].ActionControlChannel <- true
		}
		instance.SetFinalizers(remove(instance.GetFinalizers(), "ActionFinalizer"))
		if err := r.client.Update(context.TODO(), instance); err != nil {
			log.Error(err, "Remove ActionFinalizer")
			return err
		}
	}
	log.Info("Done Deleting", "finalizers", instance.GetFinalizers())

	return nil
}

func (r *ReconcileBenchmark) actionExists(instanceName string, name string) bool {
	return contains(r.state[instanceName].Actions, name)
}

func (r *ReconcileBenchmark) rateExists(instanceName string, name string) bool {
	return contains(r.state[instanceName].Rates, name)
}

func (r *ReconcileBenchmark) addAction(instanceName string, name string) {
	r.state[instanceName].Actions = append(r.state[instanceName].Actions, name)
}

func (r *ReconcileBenchmark) addRate(instanceName string, name string) {
	r.state[instanceName].Rates = append(r.state[instanceName].Rates, name)
}

func (r *ReconcileBenchmark) startActions(instance *cnsbenchv1alpha1.Benchmark, actions []cnsbenchv1alpha1.Action, stopAfter string) (map[string][]chan int, error) {
	rateConsumers := make(map[string][]chan int)
	createdAction := false
	instance.Status.RunningActions = 0
	for _, a := range actions {
		r.addAction(instance.ObjectMeta.Name, a.Name)
		if a.RunOnceSpec.Count != 0 {
			log.Info("Run once")
			objName, err := r.runSpec(instance, a.RunOnceSpec.SpecName, a.RunOnceSpec.Count, "")
			if a.Name == stopAfter && err == nil {
				log.Info("saildalsd", "alksjdsakld", a.Name, "alksjda", stopAfter)
				r.state[instance.ObjectMeta.Name].StopAfterName = objName.Name
				r.state[instance.ObjectMeta.Name].StopAfterKind = objName.Kind
			}
		} else if a.RunSpec.RateName != "" || a.ScaleSpec.RateName != "" {
			c := make(chan int)

			if a.RunSpec.RateName != "" {
				log.Info("Launching doRun", "name", a.Name)
				rateConsumers[a.RunSpec.RateName] = append(rateConsumers[a.RunSpec.RateName], c)
				go r.doRun(instance, a.Name, c, r.state[instance.ObjectMeta.Name].ActionControlChannel, a.RunSpec)
			} else {
				log.Info("Launching doScale", "name", a.Name)
				rateConsumers[a.ScaleSpec.RateName] = append(rateConsumers[a.ScaleSpec.RateName], c)
				go r.doScale(a.Name, c, r.state[instance.ObjectMeta.Name].ActionControlChannel, a.ScaleSpec)
			}
			createdAction = true
			instance.Status.RunningActions += 1
		} else {
			return rateConsumers, fmt.Errorf("Unknown kind of action")
		}
	}

	if createdAction {
		if !contains(instance.GetFinalizers(), "ActionFinalizer") {
			instance.SetFinalizers(append(instance.GetFinalizers(), "ActionFinalizer"))
		}
	}

	return rateConsumers, nil
}

func (r *ReconcileBenchmark) startRates(instance *cnsbenchv1alpha1.Benchmark, rates []cnsbenchv1alpha1.Rate, rateConsumers map[string][]chan int) error {
	createdRate := false
	instance.Status.RunningRates = 0

	for _, rate := range rates {
		log.Info(rate.Name, "two", rate.ConstantRateSpec, "three", rate.ConstantIncreaseDecreaseRateSpec)
		if consumers, ok := rateConsumers[rate.Name]; ok {
			if rate.ConstantRateSpec.Interval != 0 {
				r.createConstantRate(rate.ConstantRateSpec, consumers, r.state[instance.ObjectMeta.Name].RateControlChannel)
				createdRate = true
			} else if rate.ConstantIncreaseDecreaseRateSpec.IncInterval != 0 {
				r.createConstantIncreaseDecreaseRate(rate.ConstantIncreaseDecreaseRateSpec, consumers, r.state[instance.ObjectMeta.Name].RateControlChannel)
			} else {
				return fmt.Errorf("Unknown kind of rate")
			}
			r.addRate(instance.ObjectMeta.Name, rate.Name)
			instance.Status.RunningRates += 1
			createdRate = true
		} else {
			return fmt.Errorf("Unused Rate %s", rate.Name)
		}
	}

	if createdRate {
		if !contains(instance.GetFinalizers(), "RateFinalizer") {
			instance.SetFinalizers(append(instance.GetFinalizers(), "RateFinalizer"))
		}
	}

	return nil
}

// We're waiting for an object (Pod or Job?) to complete to stop running the benchmark - check if that
// object is complete
// XXX For now just assume it's a Job
func (r *ReconcileBenchmark) checkWaitForObject(name string) (bool, error) {
	obj := &batchv1.Job{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: "default"}, obj); err != nil {
		return false, err
	}
	log.Info("", "status", obj.Status.Succeeded)
	log.Info("", "spec", obj.Spec.Completions)
	return obj.Status.Succeeded >= *obj.Spec.Completions, nil
}

func (r *ReconcileBenchmark) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Benchmark")

	// Fetch the Benchmark instance
	instance := &cnsbenchv1alpha1.Benchmark{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		log.Info("got error", "", err)
		if errors.IsNotFound(err) {
			obj := &unstructured.Unstructured{}
			if err := r.client.Get(context.TODO(), request.NamespacedName, obj); err != nil {
				log.Info("but did get", "obj", obj)
				gvk, _ := apiutil.GVKForObject(obj, r.scheme)
				log.Info("aaa", "bbb", gvk)
			}
			return reconcile.Result{}, nil
		}
		log.Error(err, "Error getting instance")
		return reconcile.Result{}, err
	}
	instanceName := instance.ObjectMeta.Name

	log.Info("here", "here", instanceName)

	if instance.GetDeletionTimestamp() != nil {
		if _, exists := r.state[instanceName]; exists {
			if err := r.cleanup(instance); err != nil {
				return reconcile.Result{}, err
			}
			delete(r.state, instance.ObjectMeta.Name)
		}
		return reconcile.Result{}, nil
	}

	// If we're Complete but not deleted yet, nothing to do but return
	if instance.Status.State == cnsbenchv1alpha1.Complete {
		return reconcile.Result{}, nil
	}

	if _, exists := r.state[instanceName]; !exists {
		r.state[instanceName] = &BenchmarkControllerState{make(chan bool), make(chan bool), make([]string, 0), make([]string, 0), "", ""}
	}

	log.Info("", "", instance.Status)
	// if we're here, then the object isn't being deleted and exists...
	if instance.Status.State == cnsbenchv1alpha1.Running {
		log.Info("RUNNING")
		saName := r.state[instanceName].StopAfterName
		log.Info(saName)
		if saName != "" {
			complete, err := r.checkWaitForObject(saName)
			if err != nil {
				log.Error(err, "Error checking Job status")
				return reconcile.Result{}, err
			} else if complete {
				if err := r.cleanup(instance); err != nil {
					log.Error(err, "Error cleaning up")
					return reconcile.Result{}, err
				}
				instance.Status.State = cnsbenchv1alpha1.Complete
				if err := r.client.Status().Update(context.TODO(), instance); err != nil {
					log.Error(err, "Updating instance")
					return reconcile.Result{}, err
				}
			}
		}
	} else {
		// if we're not running and we're not complete, we must need to be started
		// Once each of the goroutines are started, the only way they exit is if we tell
		// them to via the control channel - i.e., even if they encounter errors they just
		// keep going
		rateConsumers := make(map[string][]chan int)
		log.Info("", "", instance.Spec)
		rateConsumers, err = r.startActions(instance, instance.Spec.Actions, instance.Spec.StopAfter)
		if err != nil {
			for i := 0; i < instance.Status.RunningActions; i++ {
				r.state[instanceName].ActionControlChannel <- true
			}
			log.Error(err, "")
			return reconcile.Result{}, err
		}
		err = r.startRates(instance, instance.Spec.Rates, rateConsumers)
		if err != nil {
			for i := 0; i < instance.Status.RunningRates; i++ {
				r.state[instanceName].RateControlChannel <- true
			}
			for i := 0; i < instance.Status.RunningActions; i++ {
				r.state[instanceName].ActionControlChannel <- true
			}
			log.Error(err, "")
			return reconcile.Result{}, err
		}
		// if we're here we started everything successfully
		instance.Status.State = cnsbenchv1alpha1.Running
		log.Info("Updating...", "aaa", instance.Status)
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

func (r *ReconcileBenchmark) sendToES(buf bytes.Buffer, url string) error {
	req, err := http.NewRequest("POST", url+"/_doc/", buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	log.Info("response body", url, string(body))

	return err
}

func (r *ReconcileBenchmark) doScale(name string, rateCh chan int, controlCh chan bool, spec cnsbenchv1alpha1.Scale) {
	var target = &appsv1.Deployment{}
	targetName := types.NamespacedName{Name: spec.Name, Namespace: "default"}

	for {
		select {
			case <- controlCh:
				log.Info("Exiting scale goroutine", "name", name)
				return
			case n:= <- rateCh:
				log.Info("Got rate!", "n", n)

				err := r.client.Get(context.TODO(), targetName, target)
				if err != nil {
					log.Error(err, "Error getting Deployment", target)
				} else {
					replicas := int32(n)
					target.Spec.Replicas = &replicas
					if err := r.client.Update(context.TODO(), target); err != nil {
						log.Error(err, "Error updating target")
					}
				}
		}
	}
}

func (r *ReconcileBenchmark) createConstantRate(spec cnsbenchv1alpha1.ConstantRate, consumers []chan int, c chan bool) {
	rate := rates.Rate{consumers, c}
	log.Info("Launching SingleRate")
	go rate.SingleRate(rates.ConstTimer{spec.Interval})
}

func (r *ReconcileBenchmark) createConstantIncreaseDecreaseRate(spec cnsbenchv1alpha1.ConstantIncreaseDecreaseRate, consumers []chan int, c chan bool) {
	rate := rates.Rate{consumers, c}
	log.Info("Launching IncDecRate")
	go rate.IncDecRate(rates.ConstTimer{spec.IncInterval}, rates.ConstTimer{spec.DecInterval}, spec.Min, spec.Max)
}

func (r *ReconcileBenchmark) doRun(bm *cnsbenchv1alpha1.Benchmark, name string, rateCh chan int, controlCh chan bool, spec cnsbenchv1alpha1.Run) {
	count := 0
	for {
		select {
			case <- controlCh:
				log.Info("Exiting run goroutine", "name", name)
				return
			case n:= <- rateCh:
				log.Info("Got rate!", "n", n)
				r.runSpec(bm, spec.SpecName, 1, "-"+strconv.Itoa(count))
				count += 1
		}
	}
}

func (r *ReconcileBenchmark) runSpec(bm *cnsbenchv1alpha1.Benchmark, specName string, count int, suffix string) (NameKind, error) {
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: specName, Namespace: "default"}, cm)
	if err != nil {
		log.Error(err, "Error getting ConfigMap", specName)
		return NameKind{}, err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode

	ret := NameKind{"", ""}
	for k := range cm.Data {
		//log.Info(k)
		//log.Info(cm.Data[k])
		obj, _, err := decode([]byte(cm.Data[k]), nil, nil)
		if err != nil {
			log.Error(err, "Error decoding yaml")
			return ret, err
		}
	
		//log.Info("obj", "obj", obj)
		for i := 0; i < count; i++ {
			obj2 := obj.DeepCopyObject()
			name, err := meta.NewAccessor().Name(obj2)
			kind, err := meta.NewAccessor().Kind(obj2)
			if err != nil {
				log.Error(err, "Error getting name")
				return ret, err
			}
			name += "-"+strconv.Itoa(i)+suffix
			meta.NewAccessor().SetName(obj2, name)
			//log.Info("Creating object", "obj", obj2)
			objMeta, err := meta.Accessor(obj2)
			if err != nil {
				log.Error(err, "Error getting ObjectMeta from obj", name)
				return ret, err
			}
			if err := controllerutil.SetControllerReference(bm, objMeta, r.scheme); err != nil {
                                log.Error(err, "Error making object child of Benchmark")
				return ret, err
                        }
			if err := controllerutil.SetOwnerReference(bm, objMeta, r.scheme); err != nil {

                                log.Error(err, "Error making object child of Benchmark")
				return ret, err
                        }
			if err := r.client.Create(context.TODO(), obj2); err != nil {
				log.Error(err, "Error creating object")
				return ret, err
			}
			ret = NameKind{name, kind}
		}
	}
	return ret, nil
}
