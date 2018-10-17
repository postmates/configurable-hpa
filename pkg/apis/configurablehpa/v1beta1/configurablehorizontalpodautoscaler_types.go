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

// specification of horizontal pod autoscaler
// was copied from HorizontalPodAutoscalerSpec + HPAControllerConfiguration
type ConfigurableHorizontalPodAutoscalerSpec struct {
	// part of HPAControllerConfiguration, see comments in the k8s repo: pkg/controller/apis/config/types.go
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=600
	DownscaleForbiddenWindowSeconds int32 `json:"downscaleForbiddenWindowSeconds,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=600
	UpscaleForbiddenWindowSeconds int32 `json:"upscaleForbiddenWindowSeconds,omitempty"`
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=100
	DownscaleLimit int32 `json:"downscaleLimit,omitempty"`
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Maximum=100
	UpscaleLimit int32 `json:"upscaleLimit,omitempty"`
	// +kubebuilder:validation:Minimum=0.01
	// +kubebuilder:validation:Maximum=0.99
	Tolerance float64 `json:"tolerance,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=120
	CpuInitializationPeriodSeconds int32 `json:"cpuInitializationPeriodSeconds,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=120
	DelayOfInitialReadinessStatusSeconds int32 `json:"delayOfInitialReadinessStatusSeconds,omitempty"`

	// part of HorizontalPodAutoscalerSpec, see comments in the k8s repo: staging/src/k8s.io/api/autoscaling/v1/types.go
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

// ConfigurableHorizontalPodAutoscalerStatus defines the observed state of ConfigurableHorizontalPodAutoscaler
type ConfigurableHorizontalPodAutoscalerStatus struct {
	ObservedGeneration              *int64       `json:"observedGeneration,omitempty"`
	LastScaleTime                   *metav1.Time `json:"lastScaleTime,omitempty"`
	CurrentReplicas                 int32        `json:"currentReplicas"`
	DesiredReplicas                 int32        `json:"desiredReplicas"`
	CurrentCPUUtilizationPercentage *int32       `json:"currentCPUUtilizationPercentage,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConfigurableHorizontalPodAutoscaler is the Schema for the configurablehorizontalpodautoscalers API
// +k8s:openapi-gen=true
type ConfigurableHorizontalPodAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigurableHorizontalPodAutoscalerSpec   `json:"spec,omitempty"`
	Status ConfigurableHorizontalPodAutoscalerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConfigurableHorizontalPodAutoscalerList contains a list of ConfigurableHorizontalPodAutoscaler
type ConfigurableHorizontalPodAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigurableHorizontalPodAutoscaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigurableHorizontalPodAutoscaler{}, &ConfigurableHorizontalPodAutoscalerList{})
}
