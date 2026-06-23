/*
Copyright 2019, 2021.

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

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

//+kubebuilder:object:root=true

// ArgoCDExport is the Schema for the argocdexports API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=argocdexports,scope=Namespaced
// +operator-sdk:csv:customresourcedefinitions:resources={{ArgoCD,v1alpha1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ArgoCDExport,v1alpha1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ConfigMap,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{CronJob,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Deployment,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Ingress,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Job,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{PersistentVolumeClaim,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Pod,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Prometheus,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ReplicaSet,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Route,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Secret,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{Service,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{ServiceMonitor,v1,""}}
// +operator-sdk:csv:customresourcedefinitions:resources={{StatefulSet,v1,""}}
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

// ArgoCDExportSpec defines the desired state of ArgoCDExport
// +k8s:openapi-gen=true
type ArgoCDExportSpec struct {
	// Argocd is the name of the ArgoCD instance to export.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="ArgoCD",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Argocd string `json:"argocd"`

	// Image is the container image to use for the export Job.
	Image string `json:"image,omitempty"`

	// Schedule in Cron format, see https://en.wikipedia.org/wiki/Cron.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Schedule",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Schedule *string `json:"schedule,omitempty"`

	// Storage defines the storage configuration options.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Storage"
	Storage *ArgoCDExportStorageSpec `json:"storage,omitempty"`

	// Version is the tag/digest to use for the export Job container image.
	Version string `json:"version,omitempty"`
}

// ArgoCDExportStatus defines the observed state of ArgoCDExport
// +k8s:openapi-gen=true
type ArgoCDExportStatus struct {
	// Phase is a simple, high-level summary of where the ArgoCDExport is in its lifecycle.
	// There are five possible phase values:
	// Pending: The ArgoCDExport has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Running: All of the containers for the ArgoCDExport are still running, or in the process of starting or restarting.
	// Succeeded: All containers for the ArgoCDExport have terminated in success, and will not be restarted.
	// Failed: At least one container has terminated in failure, either exited with non-zero status or was terminated by the system.
	// Unknown: For some reason the state of the ArgoCDExport could not be obtained.
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="Phase",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Phase string `json:"phase"`
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

func init() {
	SchemeBuilder.Register(&ArgoCDExport{}, &ArgoCDExportList{})
}
