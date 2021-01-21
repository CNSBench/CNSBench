package controllers

import (
	"context"
	"strconv"
	"strings"

	cnsbench "github.com/cnsbench/cnsbench/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *BenchmarkReconciler) getParserContainerImage(parserName string) (string, error) {
	parserCm := &corev1.ConfigMap{}

	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: parserName, Namespace: LIBRARY_NAMESPACE}, parserCm)
	if err != nil {
		r.Log.Error(err, "Error getting ConfigMap", "spec", parserName)
		return "", err
	}

	// The parser ConfigMap should indicate what container image to use to run the parser, but
	// default to busybox if none is specified
	if imageName, exists := parserCm.ObjectMeta.Annotations["container"]; !exists {
		r.Log.Info("Container annotation does not exist for parser", "parser", parserName)
		return "busybox", nil
	} else {
		return imageName, nil
	}
}

/* The ConfigMaps that contain the parsers are in the library namespace, but the workload pods that
 * will need access to the parsers are created in the default namespace.  Since pods can't attach to
 * ConfigMaps in different namespaces, we create a copy of the parser ConfigMap in the default namespace
 */
func (r *BenchmarkReconciler) createParserClone(bm *cnsbench.Benchmark, parserName string) (string, error) {
	parserCm := &corev1.ConfigMap{}

	err := r.Client.Get(context.TODO(), client.ObjectKey{Name: parserName, Namespace: LIBRARY_NAMESPACE}, parserCm)
	if err != nil {
		r.Log.Error(err, "Error getting ConfigMap", "spec", parserName)
		return "", err
	}

	newCm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.NameGenerator.GenerateName(names.SimpleNameGenerator, parserName+"-"),
			Namespace: "default",
		},
		Data: make(map[string]string, 0),
	}
	for k, v := range parserCm.Data {
		newCm.Data[k] = v
	}

	if err := r.createObj(bm, client.Object(&newCm), true); err != nil {
		r.Log.Error(err, "Creating temp ConfigMap")
	}

	return newCm.ObjectMeta.Name, err
}

func (r *BenchmarkReconciler) addParserContainer(bm *cnsbench.Benchmark, obj client.Object, parser string, outfile string, num int) (client.Object, error) {
	if parser == "" {
		parser = "null-parser"
	}

	if tmpCmName, err := r.createParserClone(bm, parser); err != nil {
		r.Log.Error(err, "Error adding parser container", "parser", parser)
		return obj, err
	} else {
		imageName, err := r.getParserContainerImage(parser)
		if err != nil {
			r.Log.Error(err, "Error getting parser container image", "parser", parser)
			return obj, err
		}
		r.Log.Info("Creating parser", "cmname", parser, "image", imageName)
		obj, err = r.AddParserContainer(bm, obj, tmpCmName, outfile, imageName, num)
		if err != nil {
			r.Log.Error(err, "Error adding parser container", "outfile", outfile)
			return obj, err
		}
		r.Log.Info("Created temp parser", "name", tmpCmName)
	}
	return obj, nil
}

func (r *BenchmarkReconciler) addOutputContainer(bm *cnsbench.Benchmark, obj client.Object, outputName string, outputFile string) (client.Object, error) {
	outputArgs := ""
	outputScript := "null-output.sh"
	for _, output := range bm.Spec.Outputs {
		if output.Name == outputName {
			r.Log.Info("Matched an output", "output", output)
			if output.HttpPostSpec.URL != "" {
				outputScript = "http-output.sh"
				outputArgs = output.HttpPostSpec.URL
			}
		}
	}

	obj, err := r.AddOutputContainer(bm, obj, outputArgs, outputScript, outputFile)
	if err != nil {
		r.Log.Error(err, "Error adding output container", "outfile", outputFile)
		return obj, err
	}
	r.Log.Info("Added output container", "name", outputFile)

	return obj, nil
}

func (r *BenchmarkReconciler) replaceVars(cmString string, workloadSpec cnsbench.Workload, instanceNum, numInstances int, workloadName string, workloadConfigMap *corev1.ConfigMap) string {
	for variable, value := range workloadSpec.Vars {
		r.Log.Info("Searching for var", "var", variable, "replacement", value)
		cmString = strings.ReplaceAll(cmString, "{{"+variable+"}}", value)
	}

	// Use workload spec's annotations as default values for any remaining variables
	for k, v := range workloadConfigMap.ObjectMeta.Annotations {
		s := strings.SplitN(k, ".", 3)
		if len(s) == 3 && s[0] == "cnsbench" && s[1] == "default" {
			r.Log.Info("Searching for var", "var", s[2], "default replacement", v)
			cmString = strings.ReplaceAll(cmString, "{{"+s[2]+"}}", v)
		}
	}
	cmString = strings.ReplaceAll(cmString, "{{ACTION_NAME}}", workloadName)
	cmString = strings.ReplaceAll(cmString, "{{ACTION_NAME_CAPS}}", strings.ToUpper(workloadName))
	cmString = strings.ReplaceAll(cmString, "{{INSTANCE_NUM}}", strconv.Itoa(instanceNum))
	cmString = strings.ReplaceAll(cmString, "{{NUM_INSTANCES}}", strconv.Itoa(numInstances))

	if strings.Contains(cmString, "{{") {
		r.Log.Info("Object definition contains double curly brackets ({{), possible unset variable", "cmstring", cmString)
	}

	return cmString
}

func (r *BenchmarkReconciler) decodeConfigMap(cmString string) (client.Object, error) {
	// Decode the yaml object from the workload spec
	objBytes := []byte(cmString)
	decode := scheme.Codecs.UniversalDeserializer().Decode
	robj, _, err := decode(objBytes, nil, nil)
	if err != nil {
		r.Log.Info("cm", "cm", cmString)
		r.Log.Error(err, "Error decoding yaml")
		return nil, err
	}
	obj := robj.(client.Object)

	return obj, nil
}

func getRole(annotations map[string]string) string {
	if _, exists := annotations["role"]; exists {
		return annotations["role"]
	}
	// if no role is set, consider the object a helper (e.g., PVC, ConfigMap)
	return "helper"
}

func (r *BenchmarkReconciler) addContainers(bm *cnsbench.Benchmark, obj client.Object, annotations map[string]string, workloadSpec cnsbench.Workload) client.Object {
	var err error
	role := getRole(annotations)

	// If user is specifying output files, use those.  Otherwise, use the object's
	// annotations
	if len(workloadSpec.OutputFiles) > 0 {
		for i, output := range workloadSpec.OutputFiles {
			if role == output.Target || (role == "workload" && output.Target == "") {
				// This object needs a parser container added.  The parsers are defined in the
				// "library" namespace, so every time a parser is used make a temporary ConfigMap
				// in the default (TODO: should be same namespace as Benchmark obj, not necessarily
				// default) namespace.  Make it controlled by the Benchmark object so it will be
				// cleaned up when the benchmark exits.
				// XXX: Since the parser might be packaged with the current workload, need to make sure
				// the parser object is seen before any objects referencing it in an Output
				obj, err = r.addParserContainer(bm, obj, output.Parser, output.Filename, i)
				if err != nil {
					r.Log.Error(err, "Error adding parser container")
				}
				if output.Sink != "" {
					obj, err = r.addOutputContainer(bm, obj, output.Sink, output.Filename)
				} else {
					obj, err = r.addOutputContainer(bm, obj, bm.Spec.AllWorkloadOutput, output.Filename)
				}
				if err != nil {
					r.Log.Error(err, "Error adding output container")
				}
			}
		}
	} else if _, exists := annotations["outputFile"]; exists {
		// TODO: Allow more than one default output file
		obj, err = r.addParserContainer(bm, obj, annotations["parser"], annotations["outputFile"], 0)
		if err != nil {
			r.Log.Error(err, "Error adding parser container")
		}
		obj, err = r.addOutputContainer(bm, obj, bm.Spec.AllWorkloadOutput, annotations["outputFile"])
		if err != nil {
			r.Log.Error(err, "Error adding output container")
		}
	}

	return obj
}

func getCount(count int) int {
	if count == 0 {
		return 1
	}
	return count
}

func (r *BenchmarkReconciler) addLabels(workloadSpec cnsbench.Workload, obj client.Object, annotations map[string]string) (client.Object, error) {
	// Add workloadname and multiinstance labels to object
	accessor := meta.NewAccessor()
	labels, err := accessor.Labels(obj)
	if err != nil {
		r.Log.Error(err, "Error getting labels")
		return nil, err
	}
	if labels == nil {
		labels = make(map[string]string)
	}
	if workloadSpec.SyncGroup != "" {
		labels["syncgroup"] = workloadSpec.SyncGroup
	}
	labels["workloadname"] = workloadSpec.Name //workloadName
	labels["role"] = getRole(annotations)

	/*
		var multipleInstanceObjs []string
		if mis, found := cm.ObjectMeta.Annotations["multipleInstances"]; found {
			multipleInstanceObjs = strings.Split(mis, ",")
		}
		if utils.Contains(multipleInstanceObjs, k) {
			labels["multiinstance"] = "true"
		}
	*/

	r.Log.Info("labels", "labels", labels)

	accessor.SetLabels(obj, labels)
	obj, err = r.AddLabelsGeneric(obj, labels)
	if err != nil {
		r.Log.Error(err, "Error updating workload labels")
	}

	return obj, err
}

func (r *BenchmarkReconciler) prepareAndRun(bm *cnsbench.Benchmark, w int, k string, workloadName string, a cnsbench.Workload, cm *corev1.ConfigMap, objBytes []byte) error {
	var err error
	var obj client.Object
	var objAnnotations map[string]string
	accessor := meta.NewAccessor()

	count := getCount(a.Count)

	// Replace vars in workload spec with values from benchmark object
	cmString := r.replaceVars(string(objBytes), a, w, count, workloadName, cm)
	if obj, err = r.decodeConfigMap(cmString); err != nil {
		return err
	}

	if objAnnotations, err = accessor.Annotations(obj); err != nil {
		r.Log.Error(err, "Error getting object annotations")
		return err
	}

	// Add containers for parsing and outputting
	obj = r.addContainers(bm, obj, objAnnotations, a)

	// Set env variables in workload container
	r.SetEnvVar("INSTANCE_NUM", strconv.Itoa(w), obj)
	r.SetEnvVar("NUM_INSTANCES", strconv.Itoa(count), obj)

	// Add workloadname and multiinstance labels to object
	if obj, err = r.addLabels(a, obj, objAnnotations); err != nil {
		return err
	}

	// Add sync container if sync start requested
	if _, exists := objAnnotations["sync"]; exists {
		if obj, err = r.AddSyncContainer(bm, obj, a.Count, workloadName, a.SyncGroup); err != nil {
			return err
		}
	}

	// Ownership can't transcend namespaces
	makeOwner := true
	if ns, _ := accessor.Namespace(obj); ns != "default" {
		makeOwner = false
	}

	// Make the actual object
	name, _ := accessor.Name(obj)
	//kind, _ := accessor.Kind(obj)
	if err := r.createObj(bm, obj, makeOwner); err != nil {
		if !errors.IsAlreadyExists(err) {
			return err
		} else {
			r.Log.Info("Already exists", "name", name)
			return nil
		}
	}

	return nil
}
