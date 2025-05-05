/*
Copyright 2025 LoongCollector Sigs.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LoongCollectorSpec defines the desired state of LoongCollector.
type LoongCollectorSpec struct {
	Image              string                      `json:"image"`
	ImagePullPolicy    corev1.PullPolicy           `json:"imagePullPolicy,omitempty"`
	Resources          corev1.ResourceRequirements `json:"resources,omitempty"`
	Env                []corev1.EnvVar             `json:"env,omitempty"`
	VolumeMounts       []corev1.VolumeMount        `json:"volumeMounts,omitempty"`
	Volumes            []corev1.Volume             `json:"volumes,omitempty"`
	Tolerations        []corev1.Toleration         `json:"tolerations,omitempty"`
	DNSPolicy          corev1.DNSPolicy            `json:"dnsPolicy,omitempty"`
	HostNetwork        bool                        `json:"hostNetwork,omitempty"`
	Lifecycle          *corev1.Lifecycle           `json:"lifecycle,omitempty"`
	LivenessProbe      *corev1.Probe               `json:"livenessProbe,omitempty"`
	ReadinessProbe     *corev1.Probe               `json:"readinessProbe,omitempty"`
	StartupProbe       *corev1.Probe               `json:"startupProbe,omitempty"`
	SecurityContext    *corev1.SecurityContext     `json:"securityContext,omitempty"`
	PodSecurityContext *corev1.PodSecurityContext  `json:"podSecurityContext,omitempty"`
}

// LoongCollectorStatus defines the observed state of LoongCollector.
type LoongCollectorStatus struct {
	Replicas int32 `json:"replicas"`
	AvailableReplicas int32 `json:"availableReplicas"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.replicas"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LoongCollector is the Schema for the loongcollectors API.
type LoongCollector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoongCollectorSpec   `json:"spec,omitempty"`
	Status LoongCollectorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoongCollectorList contains a list of LoongCollector.
type LoongCollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoongCollector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LoongCollector{}, &LoongCollectorList{})
}
