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
	"log"
	"time"

	configurablehpav1beta1 "github.com/postmates/configurable-hpa/pkg/apis/configurablehpa/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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
	err = c.Watch(&source.Kind{Type: &configurablehpav1beta1.ConfigurableHorizontalPodAutoscaler{}}, &handler.EnqueueRequestForObject{})
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
	log.Printf("Reconcile request: %v\n", request)
	instance := &configurablehpav1beta1.ConfigurableHorizontalPodAutoscaler{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	log.Printf("instance spec: %v\n", instance.Spec)
	return reconcile.Result{RequeueAfter: defaultSyncPeriod}, nil
}
