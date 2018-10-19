/*
Copyright 2018 Ivan Glushkov.

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

package configurablehorizontalpodautoscaler

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	chpa "github.com/postmates/configurable-hpa/pkg/apis/configurablehpa/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	resourceclient "k8s.io/metrics/pkg/client/clientset_generated/clientset/typed/metrics/v1beta1"
	"k8s.io/metrics/pkg/client/custom_metrics"
	"k8s.io/metrics/pkg/client/external_metrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	//podSyncPeriod                         = time.Second * 1
	defaultSyncPeriod                     = time.Second * 15
	defaultTargetCPUUtilizationPercentage = 80
	scaleUpLimitMinimum                   = 4
	scaleUpLimitFactor                    = 2
	tolerance                             = 0.1
)

// Add creates a new ConfigurableHorizontalPodAutoscaler Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	clientConfig := mgr.GetConfig()
	metricsClient := metrics.NewRESTMetricsClient(
		resourceclient.NewForConfigOrDie(clientConfig),
		custom_metrics.NewForConfigOrDie(clientConfig),
		external_metrics.NewForConfigOrDie(clientConfig),
	)
	fmt.Printf("hello: %v\n", metricsClient)
	clientSet, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		log.Fatal(err)
	}

	pods2, _ := clientSet.CoreV1().Pods("default").List(metav1.ListOptions{})
	fmt.Printf("mylister pods2: %v\n", pods2)

	replicaCalc := NewReplicaCalculator(metricsClient, clientSet.CoreV1())
	return &ReconcileConfigurableHorizontalPodAutoscaler{Client: mgr.GetClient(), scheme: mgr.GetScheme(), clientSet: clientSet, replicaCalc: replicaCalc}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("configurablehorizontalpodautoscaler-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to ConfigurableHorizontalPodAutoscaler
	err = c.Watch(&source.Kind{Type: &chpa.ConfigurableHorizontalPodAutoscaler{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileConfigurableHorizontalPodAutoscaler{}

// ReconcileConfigurableHorizontalPodAutoscaler reconciles a ConfigurableHorizontalPodAutoscaler object
type ReconcileConfigurableHorizontalPodAutoscaler struct {
	client.Client
	//replicaCalculator *podautoscaler.ReplicaCalculator
	scheme      *runtime.Scheme
	clientSet   kubernetes.Interface
	syncPeriod  time.Duration
	replicaCalc *ReplicaCalculator
}

// Reconcile reads that state of the cluster for a ConfigurableHorizontalPodAutoscaler object and makes changes based on the state read
// and what is in the ConfigurableHorizontalPodAutoscaler.Spec
// The implementation repeats kubernetes hpa implementation in v1.5.8
//		(last version before k8s.io/api/autoscaling/v2beta1 MetricsSpec was added)
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// TODO: decide, what to use: patch or update in rbac
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;update;patch
// +kubebuilder:rbac:groups=apps,resources=pods,verbs=get;list;create;update;patch;delete
// +kubebuilder:rbac:groups=configurablehpa.k8s.io,resources=configurablehorizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileConfigurableHorizontalPodAutoscaler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the ConfigurableHorizontalPodAutoscaler instance
	log.Printf("\nReconcile request: %v\n", request)

	// prepare results:
	// resRepeat will be returned if we want to re-run reconcile process
	resRepeat := reconcile.Result{RequeueAfter: defaultSyncPeriod}
	// resStop will be returned in case if we found some problem that can't be fixed, and we want to stop repeating reconcile process
	resStop := reconcile.Result{}

	hpa := &chpa.ConfigurableHorizontalPodAutoscaler{}
	err := r.Get(context.TODO(), request.NamespacedName, hpa)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Do not repeat the Reconcile again
			return resStop, nil
		}
		// Error reading the object (intermittent problems?) - requeue the request,
		return resRepeat, err
	}

	log.Printf("  -> hpa spec: %v\n", hpa.Spec)
	log.Printf("  -> hpa status: %v\n", hpa.Status)
	hasValidSpec, err := isHPAValid(hpa)
	if !hasValidSpec {
		log.Printf("  -> invalid spec: %v\n", err)
		// the hpa spec might change, we shouldn't stop repeating reconcile process
		return resRepeat, err
	}
	log.Printf("  -> spec is valid")

	// kind := hpa.Spec.ScaleTargetRef.Kind
	namespace := hpa.Namespace
	name := hpa.Spec.ScaleTargetRef.Name
	namespacedName := types.NamespacedName{Namespace: namespace, Name: name}

	deploy := &appsv1.Deployment{}
	if r.Get(context.TODO(), namespacedName, deploy) != nil {
		// Error reading the object, repeat later
		return resRepeat, err
	}
	if err := controllerutil.SetControllerReference(hpa, deploy, r.scheme); err != nil {
		// Error communicating with apiserver, repeat later
		return resRepeat, err
	}
	log.Printf("  -> found deploy for an hpa: %v/%v\n", deploy.Namespace, deploy.Name)

	return r.ReconcileCHPA(hpa, deploy)
}

func (r *ReconcileConfigurableHorizontalPodAutoscaler) ReconcileCHPA(hpa *chpa.ConfigurableHorizontalPodAutoscaler, deploy *appsv1.Deployment) (reconcile.Result, error) {
	// resRepeat will be returned if we want to re-run reconcile process
	resRepeat := reconcile.Result{RequeueAfter: defaultSyncPeriod}
	currentReplicas := deploy.Status.Replicas
	log.Printf("  -> current number of replicas: %v\n", currentReplicas)

	var err error
	cpuDesiredReplicas := int32(0)
	cpuCurrentUtilization := new(int32)
	cpuTimestamp := time.Time{}

	desiredReplicas := int32(0)
	rescaleReason := ""
	timestamp := time.Now()

	rescale := true

	if currentReplicas == 0 {
		// Autoscaling is disabled for this resource
		desiredReplicas = 0
		rescale = false
	} else if currentReplicas > hpa.Spec.MaxReplicas {
		rescaleReason = "Current number of replicas above Spec.MaxReplicas"
		desiredReplicas = hpa.Spec.MaxReplicas
	} else if hpa.Spec.MinReplicas != nil && currentReplicas < *hpa.Spec.MinReplicas {
		rescaleReason = "Current number of replicas below Spec.MinReplicas"
		desiredReplicas = *hpa.Spec.MinReplicas
	} else if currentReplicas == 0 {
		rescaleReason = "Current number of replicas must be greater than 0"
		desiredReplicas = 1
	} else {
		// All basic scenarios covered, the state should be sane, lets use metrics.
		if hpa.Spec.TargetCPUUtilizationPercentage != nil {
			cpuDesiredReplicas, cpuCurrentUtilization, cpuTimestamp, err = r.computeReplicasForCPUUtilization(hpa, deploy)
			if err != nil {
				reference := fmt.Sprintf("%s/%s/%s", hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name)
				err := fmt.Errorf("failed to compute desired number of replicas based on CPU utilization for %s: %v", reference, err)
				return resRepeat, err
			}
		}

		rescaleMetric := ""
		if cpuDesiredReplicas > desiredReplicas {
			desiredReplicas = cpuDesiredReplicas
			timestamp = cpuTimestamp
			rescaleMetric = "CPU utilization"
		}
		// TODO: add custom metrics here later
		if desiredReplicas > currentReplicas {
			rescaleReason = fmt.Sprintf("%s above target", rescaleMetric)
		}
		if desiredReplicas < currentReplicas {
			rescaleReason = "All metrics below target"
		}
		if hpa.Spec.MinReplicas != nil && desiredReplicas < *hpa.Spec.MinReplicas {
			desiredReplicas = *hpa.Spec.MinReplicas
		}

		//  never scale down to 0, reserved for disabling autoscaling
		if desiredReplicas == 0 {
			desiredReplicas = 1
		}

		if desiredReplicas > hpa.Spec.MaxReplicas {
			desiredReplicas = hpa.Spec.MaxReplicas
		}

		// Do not upscale too much to prevent incorrect rapid increase of the number of master replicas caused by
		// bogus CPU usage report from heapster/kubelet (like in issue #32304).
		if desiredReplicas > calculateScaleUpLimit(currentReplicas) {
			desiredReplicas = calculateScaleUpLimit(currentReplicas)
		}

		rescale = shouldScale(hpa, currentReplicas, desiredReplicas, timestamp)
	}

	if rescale {
		deploy.Spec.Replicas = &desiredReplicas
		r.Update(context.TODO(), deploy)
		log.Printf("Successfull rescale of %s, old size: %d, new size: %d, reason: %s",
			hpa.Name, currentReplicas, desiredReplicas, rescaleReason)
	} else {
		desiredReplicas = currentReplicas
	}

	err = r.updateStatus(hpa, currentReplicas, desiredReplicas, cpuCurrentUtilization, rescale)
	return resRepeat, err
}

func isHPAValid(hpa *chpa.ConfigurableHorizontalPodAutoscaler) (bool, error) {
	if hpa.Spec.ScaleTargetRef.Kind != "Deployment" {
		msg := fmt.Sprintf("configurable hpa doesn't support %s kind, use Deployment instead", hpa.Spec.ScaleTargetRef.Kind)
		log.Printf(msg)
		return false, fmt.Errorf(msg)
	}
	return isHPASpecValid(hpa.Spec)
}

func isHPASpecValid(spec chpa.ConfigurableHorizontalPodAutoscalerSpec) (bool, error) {
	// TODO:
	return true, nil
}

func calculateScaleUpLimit(currentReplicas int32) int32 {
	return int32(math.Max(scaleUpLimitFactor*float64(currentReplicas), scaleUpLimitMinimum))
}

func shouldScale(hpa *chpa.ConfigurableHorizontalPodAutoscaler, currentReplicas, desiredReplicas int32, timestamp time.Time) bool {
	if desiredReplicas == currentReplicas {
		return false
	}

	if hpa.Status.LastScaleTime == nil {
		return true
	}

	// Going down only if the usageRatio dropped significantly below the target
	// and there was no rescaling in the last downscaleForbiddenWindow.
	downscaleForbiddenWindow := time.Duration(hpa.Spec.DownscaleForbiddenWindowSeconds) * time.Second
	if desiredReplicas < currentReplicas && hpa.Status.LastScaleTime.Add(downscaleForbiddenWindow).Before(timestamp) {
		return true
	}

	// Going up only if the usage ratio increased significantly above the target
	// and there was no rescaling in the last upscaleForbiddenWindow.
	upscaleForbiddenWindow := time.Duration(hpa.Spec.UpscaleForbiddenWindowSeconds) * time.Second
	if desiredReplicas > currentReplicas && hpa.Status.LastScaleTime.Add(upscaleForbiddenWindow).Before(timestamp) {
		return true
	}
	return false
}

func (r *ReconcileConfigurableHorizontalPodAutoscaler) computeReplicasForCPUUtilization(hpa *chpa.ConfigurableHorizontalPodAutoscaler, deploy *appsv1.Deployment) (int32, *int32, time.Time, error) {
	utilization := int32(70)
	return 2, &utilization, time.Now(), nil
}
func (r *ReconcileConfigurableHorizontalPodAutoscaler) updateStatus(hpa *chpa.ConfigurableHorizontalPodAutoscaler, currentReplicas, desiredReplicas int32, cpuCurrentUtilization *int32, rescale bool) error {
	hpa.Status.CurrentReplicas = currentReplicas
	hpa.Status.DesiredReplicas = desiredReplicas
	hpa.Status.CurrentCPUUtilizationPercentage = cpuCurrentUtilization

	if rescale {
		now := metav1.NewTime(time.Now())
		hpa.Status.LastScaleTime = &now
	}

	return nil
}
