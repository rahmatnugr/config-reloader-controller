/*
Copyright 2025.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ConfigReloaderSpec defines the desired state of ConfigReloader.
type ConfigReloaderSpec struct {
	ConfigMaps []string         `json:"configMaps,omitempty"`
	Secrets    []string         `json:"secrets,omitempty"`
	Workloads  []WorkloadTarget `json:"workloads"`
}

type WorkloadTarget struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// ConfigReloaderStatus defines the observed state of ConfigReloader.
type ConfigReloaderStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	LastUpdated        *metav1.Time       `json:"lastUpdated,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ConfigReloader is the Schema for the configreloaders API.
type ConfigReloader struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigReloaderSpec   `json:"spec,omitempty"`
	Status ConfigReloaderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigReloaderList contains a list of ConfigReloader.
type ConfigReloaderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigReloader `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigReloader{}, &ConfigReloaderList{})
}
