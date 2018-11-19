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

// Package chpa and this controller were mostly inspired by
//	k8s-1.10.8/pkg/controller/podautoscaler/horizontal.go
//
package chpa

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"k8s.io/client-go/kubernetes"

	chpav1beta1 "github.com/postmates/configurable-hpa/pkg/apis/autoscalers/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	defaultSyncPeriod                            = time.Second * 15
	defaultTargetCPUUtilizationPercentage  int32 = 80
	defaultTolerance                             = 0.1
	defaultDownscaleForbiddenWindowSeconds       = 300
	defaultUpscaleForbiddenWindowSeconds         = 300
	defaultScaleUpLimitMinimum                   = 4.0
	defaultScaleUpLimitFactor                    = 2.0
)

// Add creates a new CHPA Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
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
	clientSet, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		log.Fatal(err)
	}

	replicaCalc := NewReplicaCalculator(metricsClient, clientSet.CoreV1(), defaultTolerance)
	return &ReconcileCHPA{
		Client:      mgr.GetClient(),
		scheme:      mgr.GetScheme(),
		clientSet:   clientSet,
		replicaCalc: replicaCalc,
		syncPeriod:  defaultSyncPeriod,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("chpa-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to CHPA
	err = c.Watch(&source.Kind{Type: &chpav1beta1.CHPA{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileCHPA{}

// ReconcileCHPA reconciles a CHPA object
type ReconcileCHPA struct {
	client.Client
	//replicaCalculator *podautoscaler.ReplicaCalculator
	scheme      *runtime.Scheme
	clientSet   kubernetes.Interface
	syncPeriod  time.Duration
	replicaCalc *ReplicaCalculator
}

// Reconcile reads that state of the cluster for a CHPA object and makes changes based on the state read
// and what is in the CHPA.Spec
// The implementation repeats kubernetes hpa implementation in v1.5.8
//		(last version before k8s.io/api/autoscaling/v2beta1 MetricsSpec was added)
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// TODO: decide, what to use: patch or update in rbac
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;update;patch
// +kubebuilder:rbac:groups=,resources=pods,verbs=get;list
// +kubebuilder:rbac:groups=autoscalers.postmates.com,resources=chpas,verbs=get;list;watch;create;update;patch;delete
func (r *ReconcileCHPA) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log.Printf("") // to have clear separation between previous and current reconcile run
	log.Printf("")
	log.Printf("Reconcile request: %v\n", request)

	// resRepeat will be returned if we want to re-run reconcile process
	// NB: we can't return non-nil err, as the "reconcile" msg will be added to the rate-limited queue
	// so that it'll slow down if we have several problems in a row
	resRepeat := reconcile.Result{RequeueAfter: r.syncPeriod}
	// resStop will be returned in case if we found some problem that can't be fixed, and we want to stop repeating reconcile process
	resStop := reconcile.Result{}

	chpa := &chpav1beta1.CHPA{}
	err := r.Get(context.TODO(), request.NamespacedName, chpa)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Do not repeat the Reconcile again
			return resStop, nil
		}
		// Error reading the object (intermittent problems?) - requeue the request,
		log.Printf("Can't get CHPA object '%v': %v", request.NamespacedName, err)
		return resRepeat, nil
	}

	setCHPADefaults(chpa)
	log.Printf("-> chpa: %v\n", chpa.String())

	if err := r.checkCHPAValidity(chpa); err != nil {
		log.Printf("Got an invalid CHPA spec '%v': %v", request.NamespacedName, err)
		// the chpa spec still persists, so we should go on processing it
		return resRepeat, nil
	}

	// kind := chpa.Spec.ScaleTargetRef.Kind
	namespace := chpa.Namespace
	name := chpa.Spec.ScaleTargetRef.Name
	namespacedName := types.NamespacedName{Namespace: namespace, Name: name}

	deploy := &appsv1.Deployment{}
	if err := r.Get(context.TODO(), namespacedName, deploy); err != nil {
		// Error reading the object, repeat later
		log.Printf("Error reading Deployment '%v': %v", namespacedName, err)
		return resRepeat, nil
	}
	if err := controllerutil.SetControllerReference(chpa, deploy, r.scheme); err != nil {
		// Error communicating with apiserver, repeat later
		log.Printf("Can't set the controller reference for the deployment %v: %v", namespacedName, err)
		return resRepeat, nil
	}

	return r.reconcileCHPA(chpa, deploy)
}

func (r *ReconcileCHPA) reconcileCHPA(chpa *chpav1beta1.CHPA, deploy *appsv1.Deployment) (reconcile.Result, error) {
	// resRepeat will be returned if we want to re-run reconcile process
	// NB: we can't return non-nil err, as the "reconcile" msg will be added to the rate-limited queue
	// so that it'll slow down if we have several problems in a row
	resRepeat := reconcile.Result{RequeueAfter: r.syncPeriod}
	currentReplicas := deploy.Status.Replicas
	log.Printf("-> deploy for an chpa: {%v/%v replicas:%v}\n", deploy.Namespace, deploy.Name, currentReplicas)

	var err error
	cpuDesiredReplicas := int32(0)
	cpuCurrentUtilization := new(int32)
	cpuTimestamp := time.Time{}

	desiredReplicas := int32(0)
	rescaleReason := ""
	timestamp := time.Now()

	rescale := true

	if *deploy.Spec.Replicas == 0 {
		// Autoscaling is disabled for this resource
		desiredReplicas = 0
		rescale = false
	} else if currentReplicas > chpa.Spec.MaxReplicas {
		rescaleReason = "Current number of replicas above Spec.MaxReplicas"
		desiredReplicas = chpa.Spec.MaxReplicas
	} else if chpa.Spec.MinReplicas != nil && currentReplicas < *chpa.Spec.MinReplicas {
		rescaleReason = "Current number of replicas below Spec.MinReplicas"
		desiredReplicas = *chpa.Spec.MinReplicas
	} else if currentReplicas == 0 {
		rescaleReason = "Current number of replicas must be greater than 0"
		desiredReplicas = 1
	} else {
		// All basic scenarios covered, the state should be sane, lets use metrics.
		if chpa.Spec.TargetCPUUtilizationPercentage != nil {
			cpuDesiredReplicas, cpuCurrentUtilization, cpuTimestamp, err = r.computeReplicasForCPUUtilization(chpa, deploy)
			if err != nil {
				reference := fmt.Sprintf("%s/%s/%s", chpa.Spec.ScaleTargetRef.Kind, chpa.Namespace, chpa.Spec.ScaleTargetRef.Name)
				err := fmt.Errorf("Failed to compute desired number of replicas based on CPU utilization for %s: %v", reference, err)
				log.Printf("%v", err)
				return resRepeat, nil
			}
		}

		rescaleMetric := ""
		if cpuDesiredReplicas > desiredReplicas {
			desiredReplicas = cpuDesiredReplicas
			timestamp = cpuTimestamp
			rescaleMetric = "CPU utilization"
		}
		if desiredReplicas > currentReplicas {
			rescaleReason = fmt.Sprintf("%s above target", rescaleMetric)
		}
		if desiredReplicas < currentReplicas {
			rescaleReason = "All metrics below target"
		}
		if chpa.Spec.MinReplicas != nil && desiredReplicas < *chpa.Spec.MinReplicas {
			desiredReplicas = *chpa.Spec.MinReplicas
		}

		//  never scale down to 0, reserved for disabling autoscaling
		if desiredReplicas == 0 {
			desiredReplicas = 1
		}

		if desiredReplicas > chpa.Spec.MaxReplicas {
			desiredReplicas = chpa.Spec.MaxReplicas
		}

		// Do not upscale too much to prevent incorrect rapid increase of the number of master replicas caused by
		// bogus CPU usage report from heapster/kubelet (like in issue #32304).
		if desiredReplicas > calculateScaleUpLimit(chpa, currentReplicas) {
			desiredReplicas = calculateScaleUpLimit(chpa, currentReplicas)
		}

		rescale = shouldScale(chpa, currentReplicas, desiredReplicas, timestamp)
	}

	if rescale {
		deploy.Spec.Replicas = &desiredReplicas
		r.Update(context.TODO(), deploy)
		log.Printf("Successfull rescale of %s, old size: %d, new size: %d, reason: %s",
			chpa.Name, currentReplicas, desiredReplicas, rescaleReason)
	} else {
		desiredReplicas = currentReplicas
	}

	err = r.updateStatus(chpa, currentReplicas, desiredReplicas, cpuCurrentUtilization, rescale)
	if err != nil {
		log.Printf("Failed to update CHPA status of '%s/%s': %v", chpa.Namespace, chpa.Name, err)
	}
	return resRepeat, nil
}

func setCHPADefaults(chpa *chpav1beta1.CHPA) {
	if chpa.Spec.DownscaleForbiddenWindowSeconds == 0 {
		chpa.Spec.DownscaleForbiddenWindowSeconds = defaultDownscaleForbiddenWindowSeconds
	}
	if chpa.Spec.UpscaleForbiddenWindowSeconds == 0 {
		chpa.Spec.UpscaleForbiddenWindowSeconds = defaultUpscaleForbiddenWindowSeconds
	}
	if chpa.Spec.ScaleUpLimitFactor == 0.0 {
		chpa.Spec.ScaleUpLimitFactor = defaultScaleUpLimitFactor
	}
	if chpa.Spec.ScaleUpLimitMinimum == 0 {
		chpa.Spec.ScaleUpLimitMinimum = defaultScaleUpLimitMinimum
	}
	if chpa.Spec.Tolerance == 0 {
		chpa.Spec.Tolerance = defaultTolerance
	}
	targetCPUUtilizationPercentage := defaultTargetCPUUtilizationPercentage
	if chpa.Spec.TargetCPUUtilizationPercentage == nil {
		chpa.Spec.TargetCPUUtilizationPercentage = &targetCPUUtilizationPercentage
	}
}
func (r *ReconcileCHPA) checkCHPAValidity(chpa *chpav1beta1.CHPA) error {
	if chpa.Spec.ScaleTargetRef.Kind != "Deployment" {
		msg := fmt.Sprintf("configurable chpa doesn't support %s kind, use Deployment instead", chpa.Spec.ScaleTargetRef.Kind)
		log.Printf(msg)
		return fmt.Errorf(msg)
	}
	return nil
}

func calculateScaleUpLimit(chpa *chpav1beta1.CHPA, currentReplicas int32) int32 {
	return int32(math.Max(chpa.Spec.ScaleUpLimitFactor*float64(currentReplicas), float64(chpa.Spec.ScaleUpLimitMinimum)))
}

func shouldScale(chpa *chpav1beta1.CHPA, currentReplicas, desiredReplicas int32, timestamp time.Time) bool {
	if desiredReplicas == currentReplicas {
		log.Printf("Will not scale: number of replicas is not changed")
		return false
	}

	if chpa.Status.LastScaleTime == nil {
		return true
	}

	// Scale down only if the usageRatio dropped significantly below the target
	// and there was no rescaling in the last downscaleForbiddenWindow.
	downscaleForbiddenWindow := time.Duration(chpa.Spec.DownscaleForbiddenWindowSeconds) * time.Second
	if desiredReplicas < currentReplicas {
		if chpa.Status.LastScaleTime.Add(downscaleForbiddenWindow).Before(timestamp) {
			return true
		}
		log.Printf("Too early to scale. Last scale was at %s, next scale will be at %s", chpa.Status.LastScaleTime, chpa.Status.LastScaleTime.Add(downscaleForbiddenWindow))
	}

	// Scale up only if the usage ratio increased significantly above the target
	// and there was no rescaling in the last upscaleForbiddenWindow.
	upscaleForbiddenWindow := time.Duration(chpa.Spec.UpscaleForbiddenWindowSeconds) * time.Second
	if desiredReplicas > currentReplicas {
		if chpa.Status.LastScaleTime.Add(upscaleForbiddenWindow).Before(timestamp) {
			return true
		}
		log.Printf("Too early to scale. Last scale was at %s, next scale will be at %s", chpa.Status.LastScaleTime, chpa.Status.LastScaleTime.Add(upscaleForbiddenWindow))
	}
	return false
}

func (r *ReconcileCHPA) computeReplicasForCPUUtilization(chpa *chpav1beta1.CHPA, deploy *appsv1.Deployment) (int32, *int32, time.Time, error) {
	targetUtilization := *chpa.Spec.TargetCPUUtilizationPercentage
	currentReplicas := deploy.Status.Replicas

	if deploy.Spec.Selector == nil {
		errMsg := "selector is required"
		log.Printf("%s\n", errMsg)
		// a.eventRecorder.Event(chpa, api.EventTypeWarning, "SelectorRequired", errMsg)
		return 0, nil, time.Time{}, fmt.Errorf(errMsg)
	}
	selector, err := metav1.LabelSelectorAsSelector(deploy.Spec.Selector)
	if err != nil {
		errMsg := fmt.Sprintf("couldn't convert selector string to a corresponding selector object: %v", err)
		log.Printf("%s\n", errMsg)
		//a.eventRecorder.Event(chpa, api.EventTypeWarning, "InvalidSelector", errMsg)
		return 0, nil, time.Time{}, fmt.Errorf(errMsg)
	}

	desiredReplicas, utilization, _, timestamp, err := r.replicaCalc.GetResourceReplicas(currentReplicas, targetUtilization, apiv1.ResourceCPU, chpa.Namespace, selector)
	if err != nil {
		lastScaleTime := getLastScaleTime(chpa)
		upscaleForbiddenWindow := time.Duration(chpa.Spec.UpscaleForbiddenWindowSeconds) * time.Second
		if time.Now().After(lastScaleTime.Add(upscaleForbiddenWindow)) {
			log.Printf("FailedGetMetrics: %v\n", err)
			//a.eventRecorder.Event(chpa, api.EventTypeWarning, "FailedGetMetrics", err.Error())
		} else {
			log.Printf("MetricsNotAvailableYet: %v\n", err)
			//a.eventRecorder.Event(chpa, api.EventTypeNormal, "MetricsNotAvailableYet", err.Error())
		}

		return 0, nil, time.Time{}, fmt.Errorf("failed to get CPU utilization: %v", err)
	}

	if desiredReplicas != currentReplicas {
		//a.eventRecorder.Eventf(chpa, api.EventTypeNormal, "DesiredReplicasComputed",
		//	"Computed the desired num of replicas: %d (avgCPUutil: %d, current replicas: %d)",
		log.Printf("-> Computed the desired num of replicas: %d (avgCPUutil: %d, current replicas: %d)",
			desiredReplicas, utilization, deploy.Status.Replicas)
	}

	return desiredReplicas, &utilization, timestamp, nil
}

func (r *ReconcileCHPA) updateStatus(chpa *chpav1beta1.CHPA, currentReplicas, desiredReplicas int32, cpuCurrentUtilization *int32, rescale bool) error {
	chpa.Status.CurrentReplicas = currentReplicas
	chpa.Status.DesiredReplicas = desiredReplicas
	chpa.Status.CurrentCPUUtilizationPercentage = cpuCurrentUtilization

	if rescale {
		now := metav1.NewTime(time.Now())
		chpa.Status.LastScaleTime = &now
	}
	r.Update(context.TODO(), chpa)

	return nil
}

// getLastScaleTime returns the chpa's last scale time or the chpa's creation time if the last scale time is nil.
func getLastScaleTime(chpa *chpav1beta1.CHPA) time.Time {
	lastScaleTime := chpa.Status.LastScaleTime
	if lastScaleTime == nil {
		lastScaleTime = &chpa.CreationTimestamp
	}
	return lastScaleTime.Time
}
