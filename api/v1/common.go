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

package v1

import corev1 "k8s.io/api/core/v1"

// PodSpec defines the desired state of Pod
type PodSpec struct {
	Annotation                    map[string]string                 `json:"annotation,omitempty"`
	Labels                        map[string]string                 `json:"labels,omitempty"`
	NodeSelector                  map[string]string                 `json:"nodeSelector,omitempty"`
	Tolerations                   []corev1.Toleration               `json:"tolerations,omitempty"`                   //schedule tolerations
	TerminationGracePeriodSeconds *int64                            `json:"terminationGracePeriodSeconds,omitempty"` // 在规定时间内停止pod，俗称 优雅停机
	SchedulerName                 string                            `json:"schedulerName,omitempty"`
	PodSecurityContext            *corev1.PodSecurityContext        `json:"podSecurityContext,omitempty"`
	ServiceAccountName            string                            `json:"serviceAccountName,omitempty"`
	ServiceName                   string                            `json:"serviceName,omitempty"`
	Version                       string                            `json:"version,omitempty"`
	Containers                    []ContainerSpec                   `json:"containers,omitempty"` // container spec
	PersistentVolumeClaimTemplate *corev1.PersistentVolumeClaimSpec `json:"persistentVolumeClaimTemplate,omitempty"`
	DnsPolicy                     corev1.DNSPolicy                  `json:"dnsPolicy,omitempty"`
}

// ContainerSpec defines the desired state of Container
type ContainerSpec struct {
	Name             string                        `json:"name"`                       // Name of the container
	Image            string                        `json:"image"`                      // Image of the container
	ImagePullPolicy  corev1.PullPolicy             `json:"imagePullPolicy,omitempty"`  // Image pull policy
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"` // Image pull secrets
	Resources        corev1.ResourceRequirements   `json:"resources,omitempty"`        // Resource requirements
	StartupProbe     corev1.Probe                  `json:"startupProbe,omitempty"`     // Startup probe
	ReadinessProbe   corev1.Probe                  `json:"readinessProbe,omitempty"`   // Readiness probe
	LivenessProbe    corev1.Probe                  `json:"livenessProbe,omitempty"`    // Liveness probe
	SecurityContext  *corev1.SecurityContext       `json:"securityContext,omitempty"`  // Security context for the container
	Envs             []corev1.EnvVar               `json:"env,omitempty"`              // Environment variables
}
