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

// ConfigurableHorizontalPodAutoscalerSpec defines the desired state of ConfigurableHorizontalPodAutoscaler
type ConfigurableHorizontalPodAutoscalerSpec struct {
	DownscaleStabilizationWindowSeconds  int32   `json:"downscaleStabilizationWindow"`
	DownscaleLimit                       int32   `json:"downscaleLimit,omitempty"`
	Tolerance                            float64 `json:"tolerance,omitempty"`
	CpuInitializationPeriodSeconds       int32   `json:"cpuInitializationPeriod,omitempty"`
	DelayOfInitialReadinessStatusSeconds int32   `json:"delayOfInitialReadinessStatus,omitempty"`
}

// ConfigurableHorizontalPodAutoscalerStatus defines the observed state of ConfigurableHorizontalPodAutoscaler
type ConfigurableHorizontalPodAutoscalerStatus struct {
	ObservedGeneration *int64       `json:"observedGeneration,omitempty"`
	LastScaleTime      *metav1.Time `json:"lastScaleTime,omitempty"`
	CurrentReplicas    int32        `json:"currentReplicas"`
	DesiredReplicas    int32        `json:"desiredReplicas"`
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
