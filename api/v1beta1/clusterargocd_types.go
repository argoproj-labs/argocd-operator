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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj-labs/argocd-operator/common"
)

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
type ClusterArgoCD struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterArgoCDSpec `json:"spec,omitempty"`
	Status ArgoCDStatus      `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterArgoCDList contains a list of ClusterArgoCD
type ClusterArgoCDList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterArgoCD `json:"items"`
}

// ClusterArgoCDSpec defines the desired state of ArgoCD
// +k8s:openapi-gen=true
type ClusterArgoCDSpec struct {

	// ControlPlaneNamespace is the target namespace where Argo CD resources would get deployed.
	ControlPlaneNamespace string `json:"controlPlaneNamespace,omitempty"`

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

// IsDeletionFinalizerPresent checks if the instance has deletion finalizer
func (clusterArgoCD *ClusterArgoCD) IsDeletionFinalizerPresent() bool {
	for _, finalizer := range clusterArgoCD.GetFinalizers() {
		if finalizer == common.ArgoCDDeletionFinalizer {
			return true
		}
	}
	return false
}
