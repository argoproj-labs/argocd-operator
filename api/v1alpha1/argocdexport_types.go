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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ArgoCDExportSpec defines the desired state of ArgoCDExport
type ArgoCDExportSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Argocd is the name of the ArgoCD instance to export.
	Argocd string `json:"argocd"`

	// Image is the container image to use for the export Job.
	Image string `json:"image,omitempty"`

	// Schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	Schedule *string `json:"schedule,omitempty"`

	// Storage defines the storage configuration options.
	Storage *ArgoCDExportStorageSpec `json:"storage,omitempty"`

	// Version is the tag/digest to use for the export Job container image.
	Version string `json:"version,omitempty"`
}

// ArgoCDExportStorageSpec defines the desired state for ArgoCDExport storage options.
type ArgoCDExportStorageSpec struct {
	// Backend defines the storage backend to use, must be "local" (the default), "aws", "azure" or "gcp".
	Backend string `json:"backend,omitempty"`

	// PVC is the desired characteristics for a PersistentVolumeClaim.
	PVC *corev1.PersistentVolumeClaimSpec `json:"pvc,omitempty"`

	// SecretName is the name of a Secret with encryption key, credentials, etc.
	SecretName string `json:"secretName,omitempty"`
}

// ArgoCDExportStatus defines the observed state of ArgoCDExport
type ArgoCDExportStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ArgoCDExport is the Schema for the argocdexports API
type ArgoCDExport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ArgoCDExportSpec   `json:"spec,omitempty"`
	Status ArgoCDExportStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ArgoCDExportList contains a list of ArgoCDExport
type ArgoCDExportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ArgoCDExport `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ArgoCDExport{}, &ArgoCDExportList{})
}
