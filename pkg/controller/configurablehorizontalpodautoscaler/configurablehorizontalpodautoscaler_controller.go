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
	"time"

	chpa "github.com/postmates/configurable-hpa/pkg/apis/configurablehpa/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const defaultSyncPeriod = time.Second * 15

// Add creates a new ConfigurableHorizontalPodAutoscaler Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileConfigurableHorizontalPodAutoscaler{Client: mgr.GetClient(), scheme: mgr.GetScheme()}
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
	scheme     *runtime.Scheme
	syncPeriod time.Duration
}

// Reconcile reads that state of the cluster for a ConfigurableHorizontalPodAutoscaler object and makes changes based on the state read
// and what is in the ConfigurableHorizontalPodAutoscaler.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// TODO: decide, what to use: patch or update
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

	return r.ReconcileCHPA(hpa, deploy)
}

func (r *ReconcileConfigurableHorizontalPodAutoscaler) ReconcileCHPA(hpa *chpa.ConfigurableHorizontalPodAutoscaler, deploy *appsv1.Deployment) (reconcile.Result, error) {
	// resRepeat will be returned if we want to re-run reconcile process
	resRepeat := reconcile.Result{RequeueAfter: defaultSyncPeriod}
	curReplicas := deploy.Status.Replicas
	log.Printf("  -> current number of replicas: %v\n", curReplicas)

	//cpuDesiredReplicas := int32(0)
	//cpuCurrentUtilization := new(int32)
	//cpuTimestamp := time.Time{}

	return resRepeat, nil
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
