/*
Copyright 2021.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NamespaceManagementSpec defines the desired state of NamespaceManagement
type NamespaceManagementSpec struct {
	ManagedBy string `json:"managedBy"`
}

// NamespaceManagementStatus defines the observed state of NamespaceManagement
type NamespaceManagementStatus struct {
	// Conditions is an array of the NamespaceManagement's status conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// NamespaceManagement is the Schema for the namespacemanagements API
type NamespaceManagement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NamespaceManagementSpec   `json:"spec,omitempty"`
	Status NamespaceManagementStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// NamespaceManagementList contains a list of NamespaceManagement
type NamespaceManagementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NamespaceManagement `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NamespaceManagement{}, &NamespaceManagementList{})
}
