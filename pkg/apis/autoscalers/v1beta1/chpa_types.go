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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CrossVersionObjectReference contains enough information to let you identify the referred resource.
type CrossVersionObjectReference struct {
	// Kind of the referent; More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds"
	Kind string `json:"kind"`
	// Name of the referent; More info: http://kubernetes.io/docs/user-guide/identifiers#names
	Name string `json:"name"`
	// API version of the referent
	// +optional
	APIVersion string `json:"apiVersion,omitempty"`
}

// Configurable HPA (CHPA) controller starts every 15 seconds, on every iteration it follows the instruction:
//		* check all CHPA objects
//		* for every CHPA object find the correspondend Deployment object
//		* check metrics for all the Containers for all the Pods of the Deployment object
//		* calculate the desired number of Replicas (terms Replicas and Pods mean the same in CHPA context)
//		* start new Replicas
//
// Upscale Forbidden Window - is the duration window from the previous "ScaleUp" event
//		for the particular CHPA object when we won't try to ScaleUp again
// "Scale Up Limit" parameters limit the number of replicas for the next ScaleUp event.
//      If the Pods metrics shows that that we should increase number of replicas,
//      the algorithm will try to limit the increase by the "ScaleUpLimit":
//			NextReplicas = min(ScaleUpLimit, DesiredReplicas)
//		ScaleUpLimit is found as a maximum of an absolute number (ScaleUpLimitMinimum) and
//		of a multiplication of currentReplicas by a koefficient (ScaleUpLimitFactor):
//			ScaleUpLimit = max(ScaleUpLimitMinimum, ScaleUpLimitFactor * currentReplicas)

// Examples:
//		* currentReplicas = 1, ScaleUpLimitMinimum = 4, ScaleUpLimitFactor = 2
//			=> ScaleUpLimit = 4
//			i.e. if metrics shows that we should scale up to 10 Replicas, we'll scale up to 4 Replicas
//			i.e. if metrics shows that we should scale up to 3 Replicas, we'll scale up to 3 Replicas
//		* currentReplicas = 10, ScaleUpLimitMinimum = 4, ScaleUpLimitFactor = 3
//		    => ScaleUpLimit = 30
//			i.e. if metrics shows that we should scale up to 10 Replicas, we'll scale up to 10 Replicas
//			i.e. if metrics shows that we should scale up to 40 Replicas, we'll scale up to 30 Replicas

// specification of horizontal pod autoscaler
// was copied from HorizontalPodAutoscalerSpec + HPAControllerConfiguration
type CHPASpec struct {
	// part of HorizontalController, see comments in the k8s repo: pkg/controller/podautoscaler/horizontal.go
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=600
	DownscaleForbiddenWindowSeconds int32 `json:"downscaleForbiddenWindowSeconds,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=600
	UpscaleForbiddenWindowSeconds int32 `json:"upscaleForbiddenWindowSeconds,omitempty"`
	// See the comment about this parameter above
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	ScaleUpLimitFactor float64 `json:"scaleUpLimitFactor,omitempty"`
	// See the comment about this parameter above
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	ScaleUpLimitMinimum int32 `json:"scaleUpLimitMinimum,omitempty"`
	// +kubebuilder:validation:Minimum=0.01
	// +kubebuilder:validation:Maximum=0.99
	Tolerance float64 `json:"tolerance,omitempty"`

	// part of HorizontalPodAutoscalerSpec, see comments in the k8s-1.10.8 repo: staging/src/k8s.io/api/autoscaling/v1/types.go
	// reference to scaled resource; horizontal pod autoscaler will learn the current resource consumption
	// and will set the desired number of pods by using its Scale subresource.
	ScaleTargetRef CrossVersionObjectReference `json:"scaleTargetRef"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	MaxReplicas int32 `json:"maxReplicas"`
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`
}

// CHPAStatus defines the observed state of CHPA
type CHPAStatus struct {
	ObservedGeneration              *int64       `json:"observedGeneration,omitempty"`
	LastScaleTime                   *metav1.Time `json:"lastScaleTime,omitempty"`
	CurrentReplicas                 int32        `json:"currentReplicas"`
	DesiredReplicas                 int32        `json:"desiredReplicas"`
	CurrentCPUUtilizationPercentage *int32       `json:"currentCPUUtilizationPercentage,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CHPA is the Schema for the chpas API
// +k8s:openapi-gen=true
type CHPA struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CHPASpec   `json:"spec,omitempty"`
	Status CHPAStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CHPAList contains a list of CHPA
type CHPAList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CHPA `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CHPA{}, &CHPAList{})
}
