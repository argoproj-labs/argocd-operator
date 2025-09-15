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
	"strings"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/argoproj-labs/argocd-operator/common"

	autoscaling "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func init() {
	SchemeBuilder.Register(&ArgoCD{}, &ArgoCDList{})
}

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

// +kubebuilder:storageversion
// +kubebuilder:object:root=true

// ArgoCD is the Schema for the argocds API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:resources={{ArgoCD,v1beta1,""}}
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
type ArgoCD struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ArgoCDSpec   `json:"spec,omitempty"`
	Status ArgoCDStatus `json:"status,omitempty"`
}

// ArgoCDApplicationControllerProcessorsSpec defines the options for the ArgoCD Application Controller processors.
type ArgoCDApplicationControllerProcessorsSpec struct {
	// Operation is the number of application operation processors.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Operation Processor Count'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Controller","urn:alm:descriptor:com.tectonic.ui:number"}
	Operation int32 `json:"operation,omitempty"`

	// Status is the number of application status processors.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Status Processor Count'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Controller","urn:alm:descriptor:com.tectonic.ui:number"}
	Status int32 `json:"status,omitempty"`
}

// ArgoCDApplicationControllerSpec defines the options for the ArgoCD Application Controller component.
type ArgoCDApplicationControllerSpec struct {

	// InitContainers defines the list of initialization containers for the Application Controller component.
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Processors contains the options for the Application Controller processors.
	Processors ArgoCDApplicationControllerProcessorsSpec `json:"processors,omitempty"`

	// LogLevel refers to the log level used by the Application Controller component. Defaults to ArgoCDDefaultLogLevel if not configured. Valid options are debug, info, error, and warn.
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat refers to the log format used by the Application Controller component. Defaults to ArgoCDDefaultLogFormat if not configured. Valid options are text or json.
	LogFormat string `json:"logFormat,omitempty"`

	// Resources defines the Compute Resources required by the container for the Application Controller.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Requirements'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Controller","urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// ParallelismLimit defines the limit for parallel kubectl operations
	ParallelismLimit int32 `json:"parallelismLimit,omitempty"`

	// AppSync is used to control the sync frequency, by default the ArgoCD
	// controller polls Git every 3m.
	//
	// Set this to a duration, e.g. 10m or 600s to control the synchronisation
	// frequency.
	// +optional
	AppSync *metav1.Duration `json:"appSync,omitempty"`

	// Sharding contains the options for the Application Controller sharding configuration.
	Sharding ArgoCDApplicationControllerShardSpec `json:"sharding,omitempty"`

	// SidecarContainers defines the list of sidecar containers for the controller deployment
	SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`

	// Env lets you specify environment for application controller pods
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Enabled is the flag to enable the Application Controller during ArgoCD installation. (optional, default `true`)
	Enabled *bool `json:"enabled,omitempty"`

	// Extra Command arguments allows users to pass command line arguments to controller workload. They get added to default command line arguments provided
	// by the operator.
	// Please note that the command line arguments provided as part of ExtraCommandArgs will not overwrite the default command line arguments.
	ExtraCommandArgs []string `json:"extraCommandArgs,omitempty"`
	// Volumes adds volumes to the Argo CD Controller container.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// VolumeMounts adds volumeMounts to the Argo CD Controller container.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Custom annotations to pods deployed by the operator
	Annotations map[string]string `json:"annotations,omitempty"`

	// Custom labels to pods deployed by the operator
	Labels map[string]string `json:"labels,omitempty"`

	// RespectRBAC restricts controller from discovering/syncing specific resources, Defaults is empty if not configured. Valid options are strict and normal.
	RespectRBAC string `json:"respectRBAC,omitempty"`
}

func (a *ArgoCDApplicationControllerSpec) IsEnabled() bool {
	return a.Enabled == nil || (a.Enabled != nil && *a.Enabled)
}

// ArgoCDApplicationControllerShardSpec defines the options available for enabling sharding for the Application Controller component.
type ArgoCDApplicationControllerShardSpec struct {

	// Enabled defines whether sharding should be enabled on the Application Controller component.
	Enabled bool `json:"enabled,omitempty"`

	// Replicas defines the number of replicas to run in the Application controller shard.
	Replicas int32 `json:"replicas,omitempty"`

	// DynamicScalingEnabled defines whether dynamic scaling should be enabled for Application Controller component
	DynamicScalingEnabled *bool `json:"dynamicScalingEnabled,omitempty"`

	// MinShards defines the minimum number of shards at any given point
	// +kubebuilder:validation:Minimum=1
	MinShards int32 `json:"minShards,omitempty"`

	// MaxShards defines the maximum number of shards at any given point
	MaxShards int32 `json:"maxShards,omitempty"`

	// ClustersPerShard defines the maximum number of clusters managed by each argocd shard
	// +kubebuilder:validation:Minimum=1
	ClustersPerShard int32 `json:"clustersPerShard,omitempty"`
}

// ArgoCDApplicationSet defines whether the Argo CD ApplicationSet controller should be installed.
type ArgoCDApplicationSet struct {

	// Env lets you specify environment for applicationSet controller pods
	Env []corev1.EnvVar `json:"env,omitempty"`

	// ExtraCommandArgs allows users to pass command line arguments to ApplicationSet controller.
	// They get added to default command line arguments provided by the operator.
	// Please note that the command line arguments provided as part of ExtraCommandArgs
	// will not overwrite the default command line arguments.
	ExtraCommandArgs []string `json:"extraCommandArgs,omitempty"`

	// Image is the Argo CD ApplicationSet image (optional)
	Image string `json:"image,omitempty"`

	// Version is the Argo CD ApplicationSet image tag. (optional)
	Version string `json:"version,omitempty"`

	// Resources defines the Compute Resources required by the container for ApplicationSet.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// LogLevel describes the log level that should be used by the ApplicationSet controller. Defaults to ArgoCDDefaultLogLevel if not set.  Valid options are debug,info, error, and warn.
	LogLevel string `json:"logLevel,omitempty"`

	WebhookServer WebhookServerSpec `json:"webhookServer,omitempty"`

	// SCMRootCAConfigMap is the name of the config map that stores the Gitlab SCM Provider's TLS certificate which will be mounted on the ApplicationSet Controller (optional).
	SCMRootCAConfigMap string `json:"scmRootCAConfigMap,omitempty"`

	// Enabled is the flag to enable the Application Set Controller during ArgoCD installation. (optional, default `true`)
	Enabled *bool `json:"enabled,omitempty"`

	// SourceNamespaces defines the namespaces applicationset resources are allowed to be created in
	SourceNamespaces []string `json:"sourceNamespaces,omitempty"`

	// SCMProviders defines the list of allowed custom SCM provider API URLs
	SCMProviders []string `json:"scmProviders,omitempty"`

	// Custom annotations to pods deployed by the operator
	Annotations map[string]string `json:"annotations,omitempty"`

	// Custom labels to pods deployed by the operator
	Labels map[string]string `json:"labels,omitempty"`

	// Volumes adds volumes to the Argo CD ApplicationSet Controller container.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// VolumeMounts adds volumeMounts to the Argo CD ApplicationSet Controller container.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// LogFormat refers to the log format used by the ApplicationSet component. Defaults to ArgoCDDefaultLogFormat if not configured. Valid options are text or json.
	// +kubebuilder:validation:Enum=text;json
	LogFormat string `json:"logformat,omitempty"`
}

func (a *ArgoCDApplicationSet) IsEnabled() bool {
	return a.Enabled == nil || (a.Enabled != nil && *a.Enabled)
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
	//Config is the dex connector configuration.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Configuration",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Dex","urn:alm:descriptor:com.tectonic.ui:text"}
	Config string `json:"config,omitempty"`

	// Optional list of required groups a user must be a member of
	Groups []string `json:"groups,omitempty"`

	// Image is the Dex container image.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Dex","urn:alm:descriptor:com.tectonic.ui:text"}
	Image string `json:"image,omitempty"`

	// OpenShiftOAuth enables OpenShift OAuth authentication for the Dex server.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="OpenShift OAuth Enabled'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Dex","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	OpenShiftOAuth bool `json:"openShiftOAuth,omitempty"`

	// Resources defines the Compute Resources required by the container for Dex.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Requirements'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Dex","urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Version is the Dex container image tag.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Dex","urn:alm:descriptor:com.tectonic.ui:text"}
	Version string `json:"version,omitempty"`

	// Env lets you specify environment variables for Dex.
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Volumes adds volumes to the dex server container
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// VolumeMounts adds volumeMounts to the dex server container
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// ArgoCDGrafanaSpec defines the desired state for the Grafana component.
type ArgoCDGrafanaSpec struct {
	// Enabled will toggle Grafana support globally for ArgoCD.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enabled",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Grafana","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`

	// Host is the hostname to use for Ingress/Route resources.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Host",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Grafana","urn:alm:descriptor:com.tectonic.ui:text"}
	Host string `json:"host,omitempty"`

	// Image is the Grafana container image.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Grafana","urn:alm:descriptor:com.tectonic.ui:text"}
	Image string `json:"image,omitempty"`

	// Ingress defines the desired state for an Ingress for the Grafana component.
	Ingress ArgoCDIngressSpec `json:"ingress,omitempty"`

	// Resources defines the Compute Resources required by the container for Grafana.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Requirements'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Grafana","urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Route defines the desired state for an OpenShift Route for the Grafana component.
	Route ArgoCDRouteSpec `json:"route,omitempty"`

	// Size is the replica count for the Grafana Deployment.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Size",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Grafana","urn:alm:descriptor:com.tectonic.ui:podCount"}
	Size *int32 `json:"size,omitempty"`

	// Version is the Grafana container image tag.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Grafana","urn:alm:descriptor:com.tectonic.ui:text"}
	Version string `json:"version,omitempty"`
}

// ArgoCDHASpec defines the desired state for High Availability support for Argo CD.
type ArgoCDHASpec struct {
	// Enabled will toggle HA support globally for Argo CD.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enabled",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:HA","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`

	// RedisProxyImage is the Redis HAProxy container image.
	RedisProxyImage string `json:"redisProxyImage,omitempty"`

	// RedisProxyVersion is the Redis HAProxy container image tag.
	RedisProxyVersion string `json:"redisProxyVersion,omitempty"`

	// Resources defines the Compute Resources required by the container for HA.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ArgoCDImageUpdaterSpec defines whether the Argo CD Image Updater controller should be installed.
type ArgoCDImageUpdaterSpec struct {
	// Enabled defines whether argocd image updater controller should be deployed or not
	Enabled bool `json:"enabled"`

	// Env let you specify environment variables for ImageUpdater pods
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources defines the Compute Resources required by the container for Argo CD Image Updater.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// ArgoCDImportSpec defines the desired state for the ArgoCD import/restore process.
type ArgoCDImportSpec struct {
	// Name of an ArgoCDExport from which to import data.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Name",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Import","urn:alm:descriptor:com.tectonic.ui:text"}
	Name string `json:"name"`

	// Namespace for the ArgoCDExport, defaults to the same namespace as the ArgoCD.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Namespace",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Import","urn:alm:descriptor:com.tectonic.ui:text"}
	Namespace *string `json:"namespace,omitempty"`
}

// ArgoCDIngressSpec defines the desired state for the Ingress resources.
type ArgoCDIngressSpec struct {
	// Annotations is the map of annotations to apply to the Ingress.
	Annotations map[string]string `json:"annotations,omitempty"`

	// Enabled will toggle the creation of the Ingress.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Ingress Enabled'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Grafana","urn:alm:descriptor:com.tectonic.ui:fieldGroup:Prometheus","urn:alm:descriptor:com.tectonic.ui:fieldGroup:Server","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`

	// IngressClassName for the Ingress resource.
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Path used for the Ingress resource.
	Path string `json:"path,omitempty"`

	// TLS configuration. Currently the Ingress only supports a single TLS
	// port, 443. If multiple members of this list specify different hosts, they
	// will be multiplexed on the same port according to the hostname specified
	// through the SNI TLS extension, if the ingress controller fulfilling the
	// ingress supports SNI.
	// +optional
	TLS []networkingv1.IngressTLS `json:"tls,omitempty"`
}

// ArgoCDKeycloakSpec defines the desired state for the Keycloak component.
type ArgoCDKeycloakSpec struct {
	// Image is the Keycloak container image.
	Image string `json:"image,omitempty"`

	// Resources defines the Compute Resources required by the container for Keycloak.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Custom root CA certificate for communicating with the Keycloak OIDC provider
	RootCA string `json:"rootCA,omitempty"`

	// Version is the Keycloak container image tag.
	Version string `json:"version,omitempty"`

	// VerifyTLS set to false disables strict TLS validation.
	VerifyTLS *bool `json:"verifyTLS,omitempty"`

	// Host is the hostname to use for Ingress/Route resources.
	Host string `json:"host,omitempty"`
}

//+kubebuilder:object:root=true

// ArgoCDList contains a list of ArgoCD
type ArgoCDList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ArgoCD `json:"items"`
}

// ArgoCDNotifications defines whether the Argo CD Notifications controller should be installed.
type ArgoCDNotifications struct {

	// Replicas defines the number of replicas to run for notifications-controller
	Replicas *int32 `json:"replicas,omitempty"`

	// Enabled defines whether argocd-notifications controller should be deployed or not
	Enabled bool `json:"enabled"`

	// SourceNamespaces is a list of namespaces from which the notifications controller will watch for ArgoCD Notification resources.
	SourceNamespaces []string `json:"sourceNamespaces,omitempty"`

	// Env let you specify environment variables for Notifications pods
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Image is the Argo CD Notifications image (optional)
	Image string `json:"image,omitempty"`

	// Version is the Argo CD Notifications image tag. (optional)
	Version string `json:"version,omitempty"`

	// Resources defines the Compute Resources required by the container for Argo CD Notifications.
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// LogLevel describes the log level that should be used by the argocd-notifications. Defaults to ArgoCDDefaultLogLevel if not set.  Valid options are debug,info, error, and warn.
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat refers to the log format used by the argocd-notifications. Defaults to ArgoCDDefaultLogFormat if not configured. Valid options are text or json.
	// +kubebuilder:validation:Enum=text;json
	LogFormat string `json:"logformat,omitempty"`
}

// ArgoCDPrometheusSpec defines the desired state for the Prometheus component.
type ArgoCDPrometheusSpec struct {
	// Enabled will toggle Prometheus support globally for ArgoCD.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Enabled",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Prometheus","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`

	// Host is the hostname to use for Ingress/Route resources.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Host",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Prometheus","urn:alm:descriptor:com.tectonic.ui:text"}
	Host string `json:"host,omitempty"`

	// Ingress defines the desired state for an Ingress for the Prometheus component.
	Ingress ArgoCDIngressSpec `json:"ingress,omitempty"`

	// Route defines the desired state for an OpenShift Route for the Prometheus component.
	Route ArgoCDRouteSpec `json:"route,omitempty"`

	// Size is the replica count for the Prometheus StatefulSet.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Size",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Prometheus","urn:alm:descriptor:com.tectonic.ui:podCount"}
	Size *int32 `json:"size,omitempty"`
}

// ArgoCDRBACSpec defines the desired state for the Argo CD RBAC configuration.
type ArgoCDRBACSpec struct {
	// DefaultPolicy is the name of the default role which Argo CD will falls back to, when
	// authorizing API requests (optional). If omitted or empty, users may be still be able to login,
	// but will see no apps, projects, etc...
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Default Policy'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:RBAC","urn:alm:descriptor:com.tectonic.ui:text"}
	DefaultPolicy *string `json:"defaultPolicy,omitempty"`

	// Policy is CSV containing user-defined RBAC policies and role definitions.
	// Policy rules are in the form:
	//   p, subject, resource, action, object, effect
	// Role definitions and bindings are in the form:
	//   g, subject, inherited-subject
	// See https://github.com/argoproj/argo-cd/blob/master/docs/operator-manual/rbac.md for additional information.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Policy",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:RBAC","urn:alm:descriptor:com.tectonic.ui:text"}
	Policy *string `json:"policy,omitempty"`

	// Scopes controls which OIDC scopes to examine during rbac enforcement (in addition to `sub` scope).
	// If omitted, defaults to: '[groups]'.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Scopes",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:RBAC","urn:alm:descriptor:com.tectonic.ui:text"}
	Scopes *string `json:"scopes,omitempty"`

	// PolicyMatcherMode configures the matchers function mode for casbin.
	// There are two options for this, 'glob' for glob matcher or 'regex' for regex matcher.
	PolicyMatcherMode *string `json:"policyMatcherMode,omitempty"`
}

// ArgoCDRedisSpec defines the desired state for the Redis server component.
type ArgoCDRedisSpec struct {
	// Image is the Redis container image.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Redis","urn:alm:descriptor:com.tectonic.ui:text"}
	Image string `json:"image,omitempty"`

	// Resources defines the Compute Resources required by the container for Redis.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Requirements'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Redis","urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Version is the Redis container image tag.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Redis","urn:alm:descriptor:com.tectonic.ui:text"}
	Version string `json:"version,omitempty"`

	// DisableTLSVerification defines whether redis server API should be accessed using strict TLS validation
	DisableTLSVerification bool `json:"disableTLSVerification,omitempty"`

	// AutoTLS specifies the method to use for automatic TLS configuration for the redis server
	// The value specified here can currently be:
	// - openshift - Use the OpenShift service CA to request TLS config
	AutoTLS string `json:"autotls,omitempty"`

	// Enabled is the flag to enable Redis during ArgoCD installation. (optional, default `true`)
	Enabled *bool `json:"enabled,omitempty"`

	// Remote specifies the remote URL of the Redis container. (optional, by default, a local instance managed by the operator is used.)
	Remote *string `json:"remote,omitempty"`
}

func (a *ArgoCDRedisSpec) IsEnabled() bool {
	return a.Enabled == nil || (a.Enabled != nil && *a.Enabled)
}

func (a *ArgoCDRedisSpec) IsRemote() bool {
	return a.Remote != nil && *a.Remote != ""
}

// ArgoCDRepoSpec defines the desired state for the Argo CD repo server component.
type ArgoCDRepoSpec struct {

	// Extra Command arguments allows users to pass command line arguments to repo server workload. They get added to default command line arguments provided
	// by the operator.
	// Please note that the command line arguments provided as part of ExtraRepoCommandArgs will not overwrite the default command line arguments.
	ExtraRepoCommandArgs []string `json:"extraRepoCommandArgs,omitempty"`

	// LogLevel describes the log level that should be used by the Repo Server. Defaults to ArgoCDDefaultLogLevel if not set.  Valid options are debug, info, error, and warn.
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat describes the log format that should be used by the Repo Server. Defaults to ArgoCDDefaultLogFormat if not configured. Valid options are text or json.
	LogFormat string `json:"logFormat,omitempty"`

	// MountSAToken describes whether you would like to have the Repo server mount the service account token
	MountSAToken bool `json:"mountsatoken,omitempty"`

	// Replicas defines the number of replicas for argocd-repo-server. Value should be greater than or equal to 0. Default is nil.
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources defines the Compute Resources required by the container for Redis.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Requirements'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Repo","urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// ServiceAccount defines the ServiceAccount user that you would like the Repo server to use
	ServiceAccount string `json:"serviceaccount,omitempty"`

	// VerifyTLS defines whether repo server API should be accessed using strict TLS validation
	VerifyTLS bool `json:"verifytls,omitempty"`

	// AutoTLS specifies the method to use for automatic TLS configuration for the repo server
	// The value specified here can currently be:
	// - openshift - Use the OpenShift service CA to request TLS config
	AutoTLS string `json:"autotls,omitempty"`

	// Image is the ArgoCD Repo Server container image.
	Image string `json:"image,omitempty"`

	// Version is the ArgoCD Repo Server container image tag.
	Version string `json:"version,omitempty"`

	// ExecTimeout specifies the timeout in seconds for tool execution
	ExecTimeout *int `json:"execTimeout,omitempty"`

	// Env lets you specify environment for repo server pods
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Volumes adds volumes to the repo server deployment
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// VolumeMounts adds volumeMounts to the repo server container
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// InitContainers defines the list of initialization containers for the repo server deployment
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// SidecarContainers defines the list of sidecar containers for the repo
	// server deployment. If the image field is omitted from a SidecarContainer,
	// the image for the repo server will be used.
	SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`

	// Enabled is the flag to enable Repo Server during ArgoCD installation. (optional, default `true`)
	Enabled *bool `json:"enabled,omitempty"`

	// Remote specifies the remote URL of the Repo Server container. (optional, by default, a local instance managed by the operator is used.)
	Remote *string `json:"remote,omitempty"`

	// Custom annotations to pods deployed by the operator
	Annotations map[string]string `json:"annotations,omitempty"`

	// Custom labels to pods deployed by the operator
	Labels map[string]string `json:"labels,omitempty"`
}

func (a *ArgoCDRepoSpec) IsEnabled() bool {
	return a.Enabled == nil || (a.Enabled != nil && *a.Enabled)
}

func (a *ArgoCDRepoSpec) IsRemote() bool {
	return a.Remote != nil && *a.Remote != ""
}

// ArgoCDRouteSpec defines the desired state for an OpenShift Route.
type ArgoCDRouteSpec struct {
	// Annotations is the map of annotations to use for the Route resource.
	Annotations map[string]string `json:"annotations,omitempty"`

	// Labels is the map of labels to use for the Route resource
	Labels map[string]string `json:"labels,omitempty"`

	// Enabled will toggle the creation of the OpenShift Route.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Route Enabled'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Grafana","urn:alm:descriptor:com.tectonic.ui:fieldGroup:Prometheus","urn:alm:descriptor:com.tectonic.ui:fieldGroup:Server","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`

	// Path the router watches for, to route traffic for to the service.
	Path string `json:"path,omitempty"`

	// TLS provides the ability to configure certificates and termination for the Route.
	TLS *routev1.TLSConfig `json:"tls,omitempty"`

	// WildcardPolicy if any for the route. Currently only 'Subdomain' or 'None' is allowed.
	WildcardPolicy *routev1.WildcardPolicyType `json:"wildcardPolicy,omitempty"`
}

// ArgoCDServerAutoscaleSpec defines the desired state for autoscaling the Argo CD Server component.
type ArgoCDServerAutoscaleSpec struct {
	// Enabled will toggle autoscaling support for the Argo CD Server component.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Autoscale Enabled'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Server","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`

	// HPA defines the HorizontalPodAutoscaler options for the Argo CD Server component.
	HPA *autoscaling.HorizontalPodAutoscalerSpec `json:"hpa,omitempty"`
}

// ArgoCDServerGRPCSpec defines the desired state for the Argo CD Server GRPC options.
type ArgoCDServerGRPCSpec struct {
	// Host is the hostname to use for Ingress/Route resources.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="GRPC Host",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Server","urn:alm:descriptor:com.tectonic.ui:text"}
	Host string `json:"host,omitempty"`

	// Ingress defines the desired state for the Argo CD Server GRPC Ingress.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="GRPC Ingress Enabled'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Server","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Ingress ArgoCDIngressSpec `json:"ingress,omitempty"`
}

// ArgoCDServerSpec defines the options for the ArgoCD Server component.
type ArgoCDServerSpec struct {
	// Autoscale defines the autoscale options for the Argo CD Server component.
	Autoscale ArgoCDServerAutoscaleSpec `json:"autoscale,omitempty"`

	// EnableRolloutsUI will add the Argo Rollouts UI extension in ArgoCD Dashboard.
	EnableRolloutsUI bool `json:"enableRolloutsUI,omitempty"`

	// GRPC defines the state for the Argo CD Server GRPC options.
	GRPC ArgoCDServerGRPCSpec `json:"grpc,omitempty"`

	// InitContainers defines the list of initialization containers for the Argo CD Server component.
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Host is the hostname to use for Ingress/Route resources.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Host",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Server","urn:alm:descriptor:com.tectonic.ui:text"}
	Host string `json:"host,omitempty"`

	// Ingress defines the desired state for an Ingress for the Argo CD Server component.
	Ingress ArgoCDIngressSpec `json:"ingress,omitempty"`

	// Insecure toggles the insecure flag.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Insecure",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Server","urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Insecure bool `json:"insecure,omitempty"`

	// LogLevel refers to the log level to be used by the ArgoCD Server component. Defaults to ArgoCDDefaultLogLevel if not set.  Valid options are debug, info, error, and warn.
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat refers to the log level to be used by the ArgoCD Server component. Defaults to ArgoCDDefaultLogFormat if not configured. Valid options are text or json.
	LogFormat string `json:"logFormat,omitempty"`

	// Replicas defines the number of replicas for argocd-server. Default is nil. Value should be greater than or equal to 0. Value will be ignored if Autoscaler is enabled.
	Replicas *int32 `json:"replicas,omitempty"`

	// Resources defines the Compute Resources required by the container for the Argo CD server component.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Requirements'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Server","urn:alm:descriptor:com.tectonic.ui:resourceRequirements"}
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Route defines the desired state for an OpenShift Route for the Argo CD Server component.
	Route ArgoCDRouteSpec `json:"route,omitempty"`

	// Service defines the options for the Service backing the ArgoCD Server component.
	Service ArgoCDServerServiceSpec `json:"service,omitempty"`

	// SidecarContainers defines the list of sidecar containers for the server deployment
	SidecarContainers []corev1.Container `json:"sidecarContainers,omitempty"`

	// Env lets you specify environment for API server pods
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Extra Command arguments that would append to the Argo CD server command.
	// ExtraCommandArgs will not be added, if one of these commands is already part of the server command
	// with same or different value.
	ExtraCommandArgs []string `json:"extraCommandArgs,omitempty"`

	// Enabled is the flag to enable ArgoCD Server during ArgoCD installation. (optional, default `true`)
	Enabled *bool `json:"enabled,omitempty"`

	// Volumes adds volumes to the Argo CD Server container.
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// VolumeMounts adds volumeMounts to the Argo CD Server container.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Custom annotations to pods deployed by the operator
	Annotations map[string]string `json:"annotations,omitempty"`

	// Custom labels to pods deployed by the operator
	Labels map[string]string `json:"labels,omitempty"`
}

func (a *ArgoCDServerSpec) IsEnabled() bool {
	return a.Enabled == nil || (a.Enabled != nil && *a.Enabled)
}

// ArgoCDServerServiceSpec defines the Service options for Argo CD Server component.
type ArgoCDServerServiceSpec struct {
	// Type is the ServiceType to use for the Service resource.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Service Type'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Server","urn:alm:descriptor:com.tectonic.ui:text"}
	Type corev1.ServiceType `json:"type"`
}

// Resource Customization for custom health check
type ResourceHealthCheck struct {
	Group string `json:"group,omitempty"`
	Kind  string `json:"kind,omitempty"`
	Check string `json:"check,omitempty"`
}

// Resource Customization for ignore difference
type ResourceIgnoreDifference struct {
	All                 *IgnoreDifferenceCustomization `json:"all,omitempty"`
	ResourceIdentifiers []ResourceIdentifiers          `json:"resourceIdentifiers,omitempty"`
}

// Resource Customization fields for ignore difference
type ResourceIdentifiers struct {
	Group         string                        `json:"group,omitempty"`
	Kind          string                        `json:"kind,omitempty"`
	Customization IgnoreDifferenceCustomization `json:"customization,omitempty"`
}

type IgnoreDifferenceCustomization struct {
	JqPathExpressions     []string `json:"jqPathExpressions,omitempty"`
	JsonPointers          []string `json:"jsonPointers,omitempty"`
	ManagedFieldsManagers []string `json:"managedFieldsManagers,omitempty"`
}

// Resource Customization for custom action
type ResourceAction struct {
	Group  string `json:"group,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Action string `json:"action,omitempty"`
}

// SSOProviderType string defines the type of SSO provider.
type SSOProviderType string

const (
	// SSOProviderTypeKeycloak means keycloak will be Installed and Integrated with Argo CD. A new realm with name argocd
	// will be created in this keycloak. This realm will have a client with name argocd that uses OpenShift v4 as Identity Provider.
	SSOProviderTypeKeycloak SSOProviderType = "keycloak"

	// SSOProviderTypeDex means dex will be Installed and Integrated with Argo CD.
	SSOProviderTypeDex SSOProviderType = "dex"
)

// ArgoCDSSOSpec defines SSO provider.
type ArgoCDSSOSpec struct {
	// Provider installs and configures the given SSO Provider with Argo CD.
	Provider SSOProviderType `json:"provider,omitempty"`

	// Dex contains the configuration for Argo CD dex authentication
	Dex *ArgoCDDexSpec `json:"dex,omitempty"`

	// Keycloak contains the configuration for Argo CD keycloak authentication
	// Removed: This field is no longer supported and the related functionality has been removed.
	Keycloak *ArgoCDKeycloakSpec `json:"keycloak,omitempty"`
}

func (a *ArgoCDSSOSpec) IsEnabled() bool {
	return a != nil && string(a.Provider) != ""
}

// KustomizeVersionSpec is used to specify information about a kustomize version to be used within ArgoCD.
type KustomizeVersionSpec struct {
	// Version is a configured kustomize version in the format of vX.Y.Z
	Version string `json:"version,omitempty"`
	// Path is the path to a configured kustomize version on the filesystem of your repo server.
	Path string `json:"path,omitempty"`
}

// LocalUserSpec is used to specify information about an ArgoCD local user to be created by the operator.
type LocalUserSpec struct {
	// Name of the local user
	Name string `json:"name"`

	// Enabled defines whether or not this local user is enabled. Default is
	// true
	Enabled *bool `json:"enabled,omitempty"`

	// ApiKey defines whether or not the user is configured to use an ArgoCD API
	// key. Default is true
	ApiKey *bool `json:"apiKey,omitempty"`

	// Login defines whether or not the user is configured to be able to login. Default is false
	Login bool `json:"login,omitempty"`

	// TokenLifetime defines the how long the token issued to this user is valid
	// for. An empty string or the value 0 indicates an infinite lifetime.
	// Examples: "30m", "8760h"
	TokenLifetime string `json:"tokenLifetime,omitempty"`

	// AutoRenewToken specifies if a new token is to be issued once the existing
	// one has expired. Default is true
	AutoRenewToken *bool `json:"autoRenewToken,omitempty"`
}

// ArgoCDMonitoringSpec is used to configure workload status monitoring for a given Argo CD instance.
// It triggers creation of serviceMonitor and PrometheusRules that alert users when a given workload
// status meets a certain criteria. For e.g, it can fire an alert if the application controller is
// pending for x mins consecutively.
type ArgoCDMonitoringSpec struct {
	// Enabled defines whether workload status monitoring is enabled for this instance or not
	Enabled bool `json:"enabled"`
	// DisableMetrics field can be used to enable or disable the collection of Metrics on Openshift
	DisableMetrics *bool `json:"disableMetrics,omitempty"`
}

// ArgoCDNodePlacementSpec is used to specify NodeSelector and Tolerations for Argo CD workloads
type ArgoCDNodePlacementSpec struct {
	// NodeSelector is a field of PodSpec, it is a map of key value pairs used for node selection
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations allow the pods to schedule onto nodes with matching taints
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// ArgoCDSpec defines the desired state of ArgoCD
// +k8s:openapi-gen=true
// +kubebuilder:validation:XValidation:rule="!(has(self.sso) && has(self.oidcConfig))",message="spec.sso and spec.oidcConfig cannot both be set"
type ArgoCDSpec struct {

	// ArgoCDApplicationSet defines whether the Argo CD ApplicationSet controller should be installed.
	ApplicationSet *ArgoCDApplicationSet `json:"applicationSet,omitempty"`

	// ApplicationInstanceLabelKey is the key name where Argo CD injects the app name as a tracking label.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Application Instance Label Key'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ApplicationInstanceLabelKey string `json:"applicationInstanceLabelKey,omitempty"`

	// InstallationID uniquely identifies an Argo CD instance in multi-instance clusters.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Installation ID",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	InstallationID string `json:"installationID,omitempty"`

	// Deprecated: ConfigManagementPlugins field is no longer supported. Argo CD now requires plugins to be defined as sidecar containers of repo server component. See '.spec.repo.sidecarContainers'. ConfigManagementPlugins was previously used to specify additional config management plugins.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Config Management Plugins'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ConfigManagementPlugins string `json:"configManagementPlugins,omitempty"`

	// Controller defines the Application Controller options for ArgoCD.
	Controller ArgoCDApplicationControllerSpec `json:"controller,omitempty"`

	// DisableAdmin will disable the admin user.
	DisableAdmin bool `json:"disableAdmin,omitempty"`

	// ExtraConfig can be used to add fields to Argo CD configmap that are not supported by Argo CD CRD.
	//
	// Note: ExtraConfig takes precedence over Argo CD CRD.
	// For example, A user sets `argocd.Spec.DisableAdmin` = true and also
	// `a.Spec.ExtraConfig["admin.enabled"]` = true. In this case, operator updates
	// Argo CD Configmap as follows -> argocd-cm.Data["admin.enabled"] = true.
	ExtraConfig map[string]string `json:"extraConfig,omitempty"`

	// GATrackingID is the google analytics tracking ID to use.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Google Analytics Tracking ID'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	GATrackingID string `json:"gaTrackingID,omitempty"`

	// GAAnonymizeUsers toggles user IDs being hashed before sending to google analytics.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Google Analytics Anonymize Users'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	GAAnonymizeUsers bool `json:"gaAnonymizeUsers,omitempty"`

	// Deprecated: Grafana defines the Grafana server options for ArgoCD.
	Grafana ArgoCDGrafanaSpec `json:"grafana,omitempty"`

	// HA options for High Availability support for the Redis component.
	HA ArgoCDHASpec `json:"ha,omitempty"`

	// HelpChatURL is the URL for getting chat help, this will typically be your Slack channel for support.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Help Chat URL'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	HelpChatURL string `json:"helpChatURL,omitempty"`

	// HelpChatText is the text for getting chat help, defaults to "Chat now!"
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Help Chat Text'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	HelpChatText string `json:"helpChatText,omitempty"`

	// Image is the ArgoCD container image for all ArgoCD components.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:ArgoCD","urn:alm:descriptor:com.tectonic.ui:text"}
	Image string `json:"image,omitempty"`

	// ImageUpdater defines whether the Argo CD ImageUpdater controller should be installed.
	ImageUpdater ArgoCDImageUpdaterSpec `json:"imageUpdater,omitempty"`

	// ImagePullPolicy is the image pull policy for all ArgoCD components.
	// Valid values are Always, IfNotPresent, Never. If not specified, defaults to the operator's global setting.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image Pull Policy",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:ArgoCD","urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:fieldDependency:image:enable"}
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Import is the import/restore options for ArgoCD.
	Import *ArgoCDImportSpec `json:"import,omitempty"`

	// Deprecated: InitialRepositories to configure Argo CD with upon creation of the cluster.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Initial Repositories'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	InitialRepositories string `json:"initialRepositories,omitempty"`

	// InitialSSHKnownHosts defines the SSH known hosts data upon creation of the cluster for connecting Git repositories via SSH.
	InitialSSHKnownHosts SSHHostsSpec `json:"initialSSHKnownHosts,omitempty"`

	// KustomizeBuildOptions is used to specify build options/parameters to use with `kustomize build`.
	KustomizeBuildOptions string `json:"kustomizeBuildOptions,omitempty"`

	// KustomizeVersions is a listing of configured versions of Kustomize to be made available within ArgoCD.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Kustomize Build Options'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	KustomizeVersions []KustomizeVersionSpec `json:"kustomizeVersions,omitempty"`

	// LocalUsers is a listing of local users to be created by the operator for the purpose of issuing ArgoCD API keys.
	LocalUsers []LocalUserSpec `json:"localUsers,omitempty"`

	// OIDCConfig is the OIDC configuration as an alternative to dex.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="OIDC Config'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	OIDCConfig string `json:"oidcConfig,omitempty"`

	// Monitoring defines whether workload status monitoring configuration for this instance.
	Monitoring ArgoCDMonitoringSpec `json:"monitoring,omitempty"`

	// NodePlacement defines NodeSelectors and Taints for Argo CD workloads
	NodePlacement *ArgoCDNodePlacementSpec `json:"nodePlacement,omitempty"`

	// Notifications defines whether the Argo CD Notifications controller should be installed.
	Notifications ArgoCDNotifications `json:"notifications,omitempty"`

	// Prometheus defines the Prometheus server options for ArgoCD.
	Prometheus ArgoCDPrometheusSpec `json:"prometheus,omitempty"`

	// RBAC defines the RBAC configuration for Argo CD.
	RBAC ArgoCDRBACSpec `json:"rbac,omitempty"`

	// Redis defines the Redis server options for ArgoCD.
	Redis ArgoCDRedisSpec `json:"redis,omitempty"`

	// Repo defines the repo server options for Argo CD.
	Repo ArgoCDRepoSpec `json:"repo,omitempty"`

	// Deprecated: RepositoryCredentials are the Git pull credentials to configure Argo CD with upon creation of the cluster.
	RepositoryCredentials string `json:"repositoryCredentials,omitempty"`

	// ResourceHealthChecks customizes resource health check behavior.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Health Check Customizations'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ResourceHealthChecks []ResourceHealthCheck `json:"resourceHealthChecks,omitempty"`

	// ResourceIgnoreDifferences customizes resource ignore difference behavior.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Ignore Difference Customizations'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ResourceIgnoreDifferences *ResourceIgnoreDifference `json:"resourceIgnoreDifferences,omitempty"`

	// ResourceActions customizes resource action behavior.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Action Customizations'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ResourceActions []ResourceAction `json:"resourceActions,omitempty"`

	// ResourceExclusions is used to completely ignore entire classes of resource group/kinds.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Exclusions'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ResourceExclusions string `json:"resourceExclusions,omitempty"`

	// ResourceInclusions is used to only include specific group/kinds in the
	// reconciliation process.
	ResourceInclusions string `json:"resourceInclusions,omitempty"`

	// ResourceTrackingMethod defines how Argo CD should track resources that it manages
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resource Tracking Method'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ResourceTrackingMethod string `json:"resourceTrackingMethod,omitempty"`

	// Server defines the options for the ArgoCD Server component.
	Server ArgoCDServerSpec `json:"server,omitempty"`

	// SourceNamespaces defines the namespaces application resources are allowed to be created in
	SourceNamespaces []string `json:"sourceNamespaces,omitempty"`

	// SSO defines the Single Sign-on configuration for Argo CD
	SSO *ArgoCDSSOSpec `json:"sso,omitempty"`

	// StatusBadgeEnabled toggles application status badge feature.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Status Badge Enabled'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	StatusBadgeEnabled bool `json:"statusBadgeEnabled,omitempty"`

	// TLS defines the TLS options for ArgoCD.
	TLS ArgoCDTLSSpec `json:"tls,omitempty"`

	// UsersAnonymousEnabled toggles anonymous user access.
	// The anonymous users get default role permissions specified argocd-rbac-cm.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Anonymous Users Enabled'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch","urn:alm:descriptor:com.tectonic.ui:advanced"}
	UsersAnonymousEnabled bool `json:"usersAnonymousEnabled,omitempty"`

	// Version is the tag to use with the ArgoCD container image for all ArgoCD components.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:ArgoCD","urn:alm:descriptor:com.tectonic.ui:text"}
	Version string `json:"version,omitempty"`

	// ClusterDomain is the cluster domain suffix used for constructing service FQDNs. Defaults to "cluster.local".
	// The full FQDN will be: <service>.<namespace>.svc.<clusterDomain>
	// This is useful for clusters that use a different DNS suffix (e.g., "CLUSTER_ID.cluster.local", "edge.local").
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cluster Domain'",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text","urn:alm:descriptor:com.tectonic.ui:advanced"}
	ClusterDomain string `json:"clusterDomain,omitempty"`

	// Banner defines an additional banner to be displayed in Argo CD UI
	Banner *Banner `json:"banner,omitempty"`

	// DefaultClusterScopedRoleDisabled will disable creation of default ClusterRoles for a cluster scoped instance.
	DefaultClusterScopedRoleDisabled bool `json:"defaultClusterScopedRoleDisabled,omitempty"`

	// AggregatedClusterRoles will allow users to have aggregated ClusterRoles for a cluster scoped instance.
	AggregatedClusterRoles bool `json:"aggregatedClusterRoles,omitempty"`

	// CmdParams specifies command-line parameters for the Argo CD components.
	CmdParams map[string]string `json:"cmdParams,omitempty"`

	// ArgoCDAgent defines configurations for the ArgoCD Agent component.
	ArgoCDAgent *ArgoCDAgentSpec `json:"argoCDAgent,omitempty"`

	// NamespaceManagement defines the list of namespaces that Argo CD is allowed to manage.
	NamespaceManagement []ManagedNamespaces `json:"namespaceManagement,omitempty"`
}

// NamespaceManagement defines the namespace management settings
type ManagedNamespaces struct {
	// Name of the namespace or pattern to be managed
	Name string `json:"name"`

	// Whether the namespace can be managed by ArgoCD
	AllowManagedBy bool `json:"allowManagedBy"`
}

const (
	ArgoCDConditionType = "Reconciled"
)

const (
	ArgoCDConditionReasonSuccess       = "Success"
	ArgoCDConditionReasonErrorOccurred = "ErrorOccurred"
)

// ArgoCDStatus defines the observed state of ArgoCD
// +k8s:openapi-gen=true
type ArgoCDStatus struct {
	// ApplicationController is a simple, high-level summary of where the Argo CD application controller component is in its lifecycle.
	// There are four possible ApplicationController values:
	// Pending: The Argo CD application controller component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Running: All of the required Pods for the Argo CD application controller component are in a Ready state.
	// Failed: At least one of the  Argo CD application controller component Pods had a failure.
	// Unknown: The state of the Argo CD application controller component could not be obtained.
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="ApplicationController",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	ApplicationController string `json:"applicationController,omitempty"`

	// ApplicationSetController is a simple, high-level summary of where the Argo CD applicationSet controller component is in its lifecycle.
	// There are four possible ApplicationSetController values:
	// Pending: The Argo CD applicationSet controller component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Running: All of the required Pods for the Argo CD applicationSet controller component are in a Ready state.
	// Failed: At least one of the  Argo CD applicationSet controller component Pods had a failure.
	// Unknown: The state of the Argo CD applicationSet controller component could not be obtained.
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="ApplicationSetController",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	ApplicationSetController string `json:"applicationSetController,omitempty"`

	// SSO is a simple, high-level summary of where the Argo CD SSO(Dex/Keycloak) component is in its lifecycle.
	// There are four possible sso values:
	// Pending: The Argo CD SSO component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Running: All of the required Pods for the Argo CD SSO component are in a Ready state.
	// Failed: At least one of the  Argo CD SSO component Pods had a failure.
	// Unknown: The state of the Argo CD SSO component could not be obtained.
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="SSO",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	SSO string `json:"sso,omitempty"`

	// NotificationsController is a simple, high-level summary of where the Argo CD notifications controller component is in its lifecycle.
	// There are four possible NotificationsController values:
	// Pending: The Argo CD notifications controller component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Running: All of the required Pods for the Argo CD notifications controller component are in a Ready state.
	// Failed: At least one of the  Argo CD notifications controller component Pods had a failure.
	// Unknown: The state of the Argo CD notifications controller component could not be obtained.
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="NotificationsController",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	NotificationsController string `json:"notificationsController,omitempty"`

	// Phase is a simple, high-level summary of where the ArgoCD is in its lifecycle.
	// There are four possible phase values:
	// Pending: The ArgoCD has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Available: All of the resources for the ArgoCD are ready.
	// Failed: At least one resource has experienced a failure.
	// Unknown: The state of the ArgoCD phase could not be obtained.
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="Phase",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Phase string `json:"phase,omitempty"`

	// Redis is a simple, high-level summary of where the Argo CD Redis component is in its lifecycle.
	// There are four possible redis values:
	// Pending: The Argo CD Redis component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Running: All of the required Pods for the Argo CD Redis component are in a Ready state.
	// Failed: At least one of the  Argo CD Redis component Pods had a failure.
	// Unknown: The state of the Argo CD Redis component could not be obtained.
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="Redis",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Redis string `json:"redis,omitempty"`

	// Repo is a simple, high-level summary of where the Argo CD Repo component is in its lifecycle.
	// There are four possible repo values:
	// Pending: The Argo CD Repo component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Running: All of the required Pods for the Argo CD Repo component are in a Ready state.
	// Failed: At least one of the  Argo CD Repo component Pods had a failure.
	// Unknown: The state of the Argo CD Repo component could not be obtained.
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="Repo",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Repo string `json:"repo,omitempty"`

	// Server is a simple, high-level summary of where the Argo CD server component is in its lifecycle.
	// There are four possible server values:
	// Pending: The Argo CD server component has been accepted by the Kubernetes system, but one or more of the required resources have not been created.
	// Running: All of the required Pods for the Argo CD server component are in a Ready state.
	// Failed: At least one of the  Argo CD server component Pods had a failure.
	// Unknown: The state of the Argo CD server component could not be obtained.
	//+operator-sdk:csv:customresourcedefinitions:type=status,displayName="Server",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Server string `json:"server,omitempty"`

	// RepoTLSChecksum contains the SHA256 checksum of the latest known state of tls.crt and tls.key in the argocd-repo-server-tls secret.
	RepoTLSChecksum string `json:"repoTLSChecksum,omitempty"`

	// RedisTLSChecksum contains the SHA256 checksum of the latest known state of tls.crt and tls.key in the argocd-operator-redis-tls secret.
	RedisTLSChecksum string `json:"redisTLSChecksum,omitempty"`

	// Host is the hostname of the Ingress.
	Host string `json:"host,omitempty"`

	// Conditions is an array of the ArgoCD's status conditions
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Banner defines an additional banner message to be displayed in Argo CD UI
// https://argo-cd.readthedocs.io/en/stable/operator-manual/custom-styles/#banners
type Banner struct {
	// Content defines the banner message content to display
	Content string `json:"content"`
	// URL defines an optional URL to be used as banner message link
	URL string `json:"url,omitempty"`
	// Permanent defines if the banner should be displayed permanently or only for a certain period of time
	Permanent bool `json:"permanent,omitempty"`
	// Position defines the position of the banner in the UI
	Position string `json:"position,omitempty"`
}

// ArgoCDTLSSpec defines the TLS options for ArgCD.
type ArgoCDTLSSpec struct {
	// CA defines the CA options.
	CA ArgoCDCASpec `json:"ca,omitempty"`

	// InitialCerts defines custom TLS certificates upon creation of the cluster for connecting Git repositories via HTTPS.
	InitialCerts map[string]string `json:"initialCerts,omitempty"`
}

type SSHHostsSpec struct {
	// ExcludeDefaultHosts describes whether you would like to include the default
	// list of SSH Known Hosts provided by ArgoCD.
	ExcludeDefaultHosts bool `json:"excludedefaulthosts,omitempty"`

	// Keys describes a custom set of SSH Known Hosts that you would like to
	// have included in your ArgoCD server.
	Keys string `json:"keys,omitempty"`
}

// WebhookServerSpec defines the options for the ApplicationSet Webhook Server component.
type WebhookServerSpec struct {

	// Host is the hostname to use for Ingress/Route resources.
	//+operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Host",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:Server","urn:alm:descriptor:com.tectonic.ui:text"}
	Host string `json:"host,omitempty"`

	// Ingress defines the desired state for an Ingress for the Application set webhook component.
	Ingress ArgoCDIngressSpec `json:"ingress,omitempty"`

	// Route defines the desired state for an OpenShift Route for the Application set webhook component.
	Route ArgoCDRouteSpec `json:"route,omitempty"`
}

// AgentMode is a type which represents possible agent modes
type AgentMode string

// Possible agent modes
const (
	// AgentModeManaged indicates that the agent is in managed mode
	AgentModeManaged AgentMode = "managed"
	// AgentModeAutonomous indicates that the agent is in autonomous mode
	AgentModeAutonomous AgentMode = "autonomous"
)

// AgentComponentType is a type which represents possible agent component types
type AgentComponentType string

// Possible agent component types
const (
	// AgentComponentTypePrincipal indicates the component type is principal
	AgentComponentTypePrincipal AgentComponentType = "principal"
	// AgentComponentTypeAgent indicates the component type is agent
	AgentComponentTypeAgent AgentComponentType = "agent"
)

type ArgoCDAgentSpec struct {

	// Principal defines configurations for the Principal component of Argo CD Agent.
	Principal *PrincipalSpec `json:"principal,omitempty"`

	// Agent defines configurations for the Agent component of Argo CD Agent.
	Agent *AgentSpec `json:"agent,omitempty"`
}

type PrincipalSpec struct {

	// Enabled is the flag to enable the Principal component during Argo CD installation. (optional, default `false`)
	Enabled *bool `json:"enabled,omitempty"`

	// Auth is the authentication method for the Principal component.
	Auth string `json:"auth,omitempty"`

	// LogLevel refers to the log level used by the Principal component.
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat refers to the log format used by the Principal component.
	LogFormat string `json:"logFormat,omitempty"`

	// Image is the name of Argo CD Agent image
	Image string `json:"image,omitempty"`

	// Env lets you specify environment for principal pods
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Server defines the server options for the Principal component.
	Server *PrincipalServerSpec `json:"server,omitempty"`

	// Namespace is the configuration for the Principal component namespace.
	Namespace *PrincipalNamespaceSpec `json:"namespace,omitempty"`

	// ResourceProxy defines the Resource Proxy options for the Principal component.
	ResourceProxy *PrincipalResourceProxySpec `json:"resourceProxy,omitempty"`

	// Redis defines the Redis options for the Principal component.
	Redis *PrincipalRedisSpec `json:"redis,omitempty"`

	// TLS defines the TLS options for the Principal component.
	TLS *PrincipalTLSSpec `json:"tls,omitempty"`

	// JWT defines the JWT options for the Principal component.
	JWT *PrincipalJWTSpec `json:"jwt,omitempty"`
}

type PrincipalServerSpec struct {
	// EnableWebSocket is the flag to enable the WebSocket on gRPC to stream events to the Agent.
	EnableWebSocket *bool `json:"enableWebSocket,omitempty"`

	// KeepAliveMinInterval is the minimum interval between keep-alive messages sent by the Agent to the Principal.
	KeepAliveMinInterval string `json:"keepAliveMinInterval,omitempty"`

	// Service defines the options for the Service backing the ArgoCD Agent component.
	// If not set, type ClusterIP will be used by default.
	Service ArgoCDAgentPrincipalServiceSpec `json:"service,omitempty"`

	// Route defines the options for the Route backing the ArgoCD Agent component.
	// Route is disabled only when explicitly configured with Enabled: false
	Route ArgoCDAgentPrincipalRouteSpec `json:"route,omitempty"`
}

type PrincipalRedisSpec struct {

	// ServerAddress is the address of the Redis server to be used by the Principal component.
	ServerAddress string `json:"serverAddress,omitempty"`

	// CompressionType is the compression type to be used by Redis.
	CompressionType string `json:"compressionType,omitempty"`
}

type PrincipalJWTSpec struct {

	// InsecureGenerate is the flag to allow the principal to generate its own private key for signing JWT tokens (insecure).
	InsecureGenerate *bool `json:"insecureGenerate,omitempty"`

	// SecretName is the name of the secret containing the JWT signing key.
	SecretName string `json:"secretName,omitempty"`
}

type PrincipalNamespaceSpec struct {

	// AllowedNamespaces is a list of namespaces the principal shall watch and process Argo CD resources in.
	AllowedNamespaces []string `json:"allowedNamespaces,omitempty"`

	// EnableNamespaceCreate is the flag to enable namespace creation for agents.
	EnableNamespaceCreate *bool `json:"enableNamespaceCreate,omitempty"`

	// NamespaceCreatePattern is a regexp pattern to restrict the names of namespaces to be created.
	NamespaceCreatePattern string `json:"namespaceCreatePattern,omitempty"`

	// NamespaceCreateLabels is the set of labels to apply to namespaces created for agents. Ex: "foo=bar,bar=baz"
	NamespaceCreateLabels []string `json:"namespaceCreateLabels,omitempty"`
}

type PrincipalResourceProxySpec struct {

	// SecretName is the name of the secret containing the TLS certificate and key for the resource proxy.
	SecretName string `json:"secretName,omitempty"`

	// CASecretName is the name of the secret containing the CA certificate for the resource proxy.
	CASecretName string `json:"caSecretName,omitempty"`
}

type PrincipalTLSSpec struct {

	// SecretName is The name of the secret containing the TLS certificate and key.
	SecretName string `json:"secretName,omitempty"`

	// RootCASecretName is the name of the secret containing the root CA TLS certificate
	RootCASecretName string `json:"rootCASecretName,omitempty"`

	// InsecureGenerate is the flag to allow the principal to generate its own set of TLS cert and key on startup when none are configured
	InsecureGenerate *bool `json:"insecureGenerate,omitempty"`
}

// ArgoCDAgentPrincipalServiceSpec defines the options for the Service backing the ArgoCD Agent Principalcomponent.
type ArgoCDAgentPrincipalServiceSpec struct {
	// Type is the ServiceType to use for the Service resource.
	// If not set, type ClusterIP will be used by default.
	Type corev1.ServiceType `json:"type"`
}

// ArgoCDAgentPrincipalRouteSpec defines the options for the Route backing the ArgoCD Agent Principal component.
type ArgoCDAgentPrincipalRouteSpec struct {
	// Enabled will toggle the creation of the OpenShift Route, ignored in case of non OpenShift cluster.
	// Route is disabled only when explicitly configured with false
	Enabled *bool `json:"enabled,omitempty"`
}

func (a *PrincipalSpec) IsEnabled() bool {
	return a.Enabled != nil && *a.Enabled
}

type AgentSpec struct {

	// Enabled is the flag to enable the Agent component during Argo CD installation. (optional, default `false`)
	Enabled *bool `json:"enabled,omitempty"`

	// Creds is the credential identifier for the agent authentication
	Creds string `json:"creds,omitempty"`

	// LogLevel refers to the log level used by the Agent component.
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat refers to the log format used by the Agent component.
	LogFormat string `json:"logFormat,omitempty"`

	// Image is the name of Argo CD Agent image
	Image string `json:"image,omitempty"`

	// Env lets you specify environment for agent pods
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Client defines the client options for the Agent component.
	Client *AgentClientSpec `json:"client,omitempty"`

	// Redis defines the Redis options for the Agent component.
	Redis *AgentRedisSpec `json:"redis,omitempty"`

	// TLS defines the TLS options for the Agent component.
	TLS *AgentTLSSpec `json:"tls,omitempty"`
}

type AgentClientSpec struct {

	// PrincipalServerAddress is the remote address of the principal server to connect to.
	PrincipalServerAddress string `json:"principalServerAddress,omitempty"`

	// PrincipalServerPort is the remote port of the principal server to connect to.
	PrincipalServerPort string `json:"principalServerPort,omitempty"`

	// Mode is the operational mode for the agent (managed or autonomous)
	Mode string `json:"mode,omitempty"`

	// EnableWebSocket is the flag to enable WebSocket for event streaming
	EnableWebSocket *bool `json:"enableWebSocket,omitempty"`

	// EnableCompression is the flag to enable compression while sending data between Principal and Agent using gRPC
	EnableCompression *bool `json:"enableCompression,omitempty"`

	// KeepAliveInterval is the interval for keep-alive pings to the principal
	KeepAliveInterval string `json:"keepAliveInterval,omitempty"`
}

type AgentRedisSpec struct {

	// ServerAddress is the address of the Redis server to be used by the PrincAgentipal component.
	ServerAddress string `json:"serverAddress,omitempty"`
}

type AgentTLSSpec struct {

	// SecretName is the name of the secret containing the agent client TLS certificate
	SecretName string `json:"secretName,omitempty"`

	// RootCASecretName is the name of the secret containing the root CA certificate
	RootCASecretName string `json:"rootCASecretName,omitempty"`

	// Insecure is the flag to skip TLS certificate validation when connecting to the principal (insecure, for development only)
	Insecure *bool `json:"insecure,omitempty"`
}

func (a *AgentSpec) IsEnabled() bool {
	return a.Enabled != nil && *a.Enabled
}

// IsDeletionFinalizerPresent checks if the instance has deletion finalizer
func (argocd *ArgoCD) IsDeletionFinalizerPresent() bool {
	for _, finalizer := range argocd.GetFinalizers() {
		if finalizer == common.ArgoCDDeletionFinalizer {
			return true
		}
	}
	return false
}

// WantsAutoTLS returns true if:
// 1. user has configured a route with reencrypt.
// 2. user has not configured TLS and we default to reencrypt.
func (s *ArgoCDServerSpec) WantsAutoTLS() bool {
	return s.Route.TLS == nil || s.Route.TLS.Termination == routev1.TLSTerminationReencrypt
}

// WantsAutoTLS returns true if the repository server configuration has set
// the autoTLS toggle to a supported provider.
func (r *ArgoCDRepoSpec) WantsAutoTLS() bool {
	return r.AutoTLS == "openshift"
}

// WantsAutoTLS returns true if the redis server configuration has set
// the autoTLS toggle to a supported provider.
func (r *ArgoCDRedisSpec) WantsAutoTLS() bool {
	return r.AutoTLS == "openshift"
}

// ApplicationInstanceLabelKey returns either the custom application instance
// label key if set, or the default value.
func (a *ArgoCD) ApplicationInstanceLabelKey() string {
	if a.Spec.ApplicationInstanceLabelKey != "" {
		return a.Spec.ApplicationInstanceLabelKey
	} else {
		return common.ArgoCDDefaultApplicationInstanceLabelKey
	}
}

// ResourceTrackingMethod represents the Argo CD resource tracking method to use
type ResourceTrackingMethod int

const (
	ResourceTrackingMethodInvalid            ResourceTrackingMethod = -1
	ResourceTrackingMethodLabel              ResourceTrackingMethod = 0
	ResourceTrackingMethodAnnotation         ResourceTrackingMethod = 1
	ResourceTrackingMethodAnnotationAndLabel ResourceTrackingMethod = 2
)

const (
	stringResourceTrackingMethodLabel              string = "label"
	stringResourceTrackingMethodAnnotation         string = "annotation"
	stringResourceTrackingMethodAnnotationAndLabel string = "annotation+label"
)

// String returns the string representation for a ResourceTrackingMethod
func (r ResourceTrackingMethod) String() string {
	switch r {
	case ResourceTrackingMethodLabel:
		return stringResourceTrackingMethodLabel
	case ResourceTrackingMethodAnnotation:
		return stringResourceTrackingMethodAnnotation
	case ResourceTrackingMethodAnnotationAndLabel:
		return stringResourceTrackingMethodAnnotationAndLabel
	}

	// Default is to use Annotation
	return stringResourceTrackingMethodAnnotation
}

// ParseResourceTrackingMethod parses a string into a resource tracking method
func ParseResourceTrackingMethod(name string) ResourceTrackingMethod {
	switch name {
	case stringResourceTrackingMethodLabel:
		return ResourceTrackingMethodLabel
	case stringResourceTrackingMethodAnnotation, "":
		return ResourceTrackingMethodAnnotation
	case stringResourceTrackingMethodAnnotationAndLabel:
		return ResourceTrackingMethodAnnotationAndLabel
	}

	return ResourceTrackingMethodInvalid
}

// ToLower returns the lower case representation for a SSOProviderType
func (p SSOProviderType) ToLower() SSOProviderType {
	str := string(p)
	return SSOProviderType(strings.ToLower(str))
}

// UseExternalCertificate return true if .route.tls.externalCertificate is set
func (r *ArgoCDRouteSpec) UseExternalCertificate() bool {
	if r != nil && r.TLS != nil && r.TLS.ExternalCertificate != nil {
		return true
	}
	return false
}
