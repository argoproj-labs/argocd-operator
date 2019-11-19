// Copyright 2019 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ArgoCD{}, &ArgoCDList{})
}

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ArgoCD is the Schema for the argocds API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type ArgoCD struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ArgoCDSpec   `json:"spec,omitempty"`
	Status ArgoCDStatus `json:"status,omitempty"`
}

// ArgoCDApplicationControllerProcessorsSpec defines the options for the ArgoCD Application Controller processors.
type ArgoCDApplicationControllerProcessorsSpec struct {
	// Operation is the number of application operation processors.
	Operation int32 `json:"operation,omitempty"`

	// Status is the number of application status processors.
	Status int32 `json:"status,omitempty"`
}

// ArgoCDApplicationControllerSpec defines the options for the ArgoCD Application Controller component.
type ArgoCDApplicationControllerSpec struct {
	Processors ArgoCDApplicationControllerProcessorsSpec `json:"processors"`
}

// ArgoCDCASpec defines the CA options for ArgCD.
type ArgoCDCASpec struct {
	// ConfigMapName is the name of the ConfigMap containing the CA Certificate.
	ConfigMapName string `json:"configMapName,omitempty"`

	// SecretName is the name of the Secret containing the CA Certificate and Key.
	SecretName string `json:"secretName,omitempty"`
}

// ArgoCDCertificateSpec defines the options for the ArgoCD certificates.
type ArgoCDCertificateSpec struct {
	// SecretName is the name of the Secret containing the Certificate and Key.
	SecretName string `json:"secretName"`
}

// ArgoCDDexSpec defines the desired state for the Dex server component.
type ArgoCDDexSpec struct {
	// Image is the Dex container image.
	Image string `json:"image,omitempty"`

	// Version is the Dex container image tag.
	Version string `json:"version,omitempty"`
}

// ArgoCDGrafanaSpec defines the desired state for the Grafana server component.
type ArgoCDGrafanaSpec struct {
	// Enabled will toggle Grafana support globally for ArgoCD.
	Enabled bool `json:"enabled"`

	// Host is the hostname to use for Ingress/Route resources.
	Host string `json:"host,omitempty"`

	// Image is the Grafana container image.
	Image string `json:"image,omitempty"`

	// Size is the replica count for the Grafana Deployment.
	Size int32 `json:"size,omitempty"`

	// Version is the Grafana container image tag.
	Version string `json:"version,omitempty"`
}

// ArgoCDIngressSpec defines the desired state for the Ingress resources.
type ArgoCDIngressSpec struct {
	// Annotations is the map of annotations to use for the Ingress resource.
	Annotations map[string]string `json:"annotations,omitempty"`

	// Enabled will toggle Ingress support globally for ArgoCD.
	Enabled bool `json:"enabled"`

	// Host is the hostname to use for the Ingress resource.
	Host string `json:"host,omitempty"`

	// Path is the path to use for the Ingress resource.
	Path string `json:"path,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ArgoCDList contains a list of ArgoCD
type ArgoCDList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ArgoCD `json:"items"`
}

// ArgoCDPrometheusSpec defines the desired state for the Prometheus component.
type ArgoCDPrometheusSpec struct {
	// Enabled will toggle Prometheus support globally for ArgoCD.
	Enabled bool `json:"enabled"`

	// Host is the hostname to use for Ingress/Route resources.
	Host string `json:"host,omitempty"`

	// Size is the replica count for the Prometheus StatefulSet.
	Size int32 `json:"size,omitempty"`
}

// ArgoCDRedisSpec defines the desired state for the Redis server component.
type ArgoCDRedisSpec struct {
	// Image is the Redis container image.
	Image string `json:"image,omitempty"`

	// Version is the Redis container image tag.
	Version string `json:"version,omitempty"`
}

// ArgoCDSpec defines the desired state of ArgoCD
// +k8s:openapi-gen=true
type ArgoCDSpec struct {
	// Controller defines the Application Controller options for ArgoCD.
	Controller ArgoCDApplicationControllerSpec `json:"controller,omitempty"`

	// Dex defines the Dex server options for ArgoCD.
	Dex ArgoCDDexSpec `json:"dex,omitempty"`

	// Grafana defines the Grafana server options for ArgoCD.
	Grafana ArgoCDGrafanaSpec `json:"grafana,omitempty"`

	// Image is the ArgoCD container image for all ArgoCD components.
	Image string `json:"image,omitempty"`

	// Ingress defines the Ingress options for ArgoCD.
	Ingress ArgoCDIngressSpec `json:"ingress,omitempty"`

	// Prometheus defines the Prometheus server options for ArgoCD.
	Prometheus ArgoCDPrometheusSpec `json:"prometheus,omitempty"`

	// Redis defines the Redis server options for ArgoCD.
	Redis ArgoCDRedisSpec `json:"redis,omitempty"`

	// Server defines the options for the ArgoCD Server component.
	Server ArgoCDServerSpec `json:"server,omitempty"`

	// TLS defines the TLS options for ArgoCD.
	TLS ArgoCDTLSSpec `json:"tls,omitempty"`

	// Version is the tag to use with the ArgoCD container image for all ArgoCD components.
	Version string `json:"version,omitempty"`
}

// ArgoCDServerSpec defines the options for the ArgoCD Server component.
type ArgoCDServerSpec struct {
	// Insecure toggles the insecure flag.
	Insecure bool `json:"insecure,omitempty"`

	// Service defines the options for the Service backing the ArgoCD Server component.
	Service ArgoCDServerServiceSpec `json:"service,omitempty"`
}

// ArgoCDServerServiceSpec defines the Service options for Argo CD Server component.
type ArgoCDServerServiceSpec struct {
	// Type is the ServiceType to use for the Service resource.
	Type corev1.ServiceType `json:"type"`
}

// ArgoCDStatus defines the observed state of ArgoCD
// +k8s:openapi-gen=true
type ArgoCDStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
}

// ArgoCDTLSSpec defines the TLS options for ArgCD.
type ArgoCDTLSSpec struct {
	// CA defines the CA options.
	CA ArgoCDCASpec `json:"ca"`
}
