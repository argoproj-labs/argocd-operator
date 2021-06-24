// Copyright 2021 ArgoCD Operator Developers
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

package argocd

import (
	"fmt"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewConfigMapWithName creates a new ConfigMap with the given name for the given ArgCD.
func NewConfigMapWithName(name string, cr *argoprojv1a1.ArgoCD) *corev1.ConfigMap {
	cm := newConfigMap(cr)
	cm.ObjectMeta.Name = name

	lbls := cm.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	cm.ObjectMeta.Labels = lbls

	return cm
}

// newConfigMap returns a new ConfigMap instance for the given ArgoCD.
func newConfigMap(cr *argoprojv1a1.ArgoCD) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// GetCAConfigMapName will return the CA ConfigMap name for the given ArgoCD.
func GetCAConfigMapName(cr *argoprojv1a1.ArgoCD) string {
	if len(cr.Spec.TLS.CA.ConfigMapName) > 0 {
		return cr.Spec.TLS.CA.ConfigMapName
	}
	return nameWithSuffix(common.ArgoCDCASuffix, cr)
}

// nameWithSuffix will return a name based on the given ArgoCD. The given suffix is appended to the generated name.
// Example: Given an ArgoCD with the name "example-argocd", providing the suffix "foo" would result in the value of
// "example-argocd-foo" being returned.
func nameWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("%s-%s", cr.Name, suffix)
}

// NewConfigMapWithSuffix creates a new ConfigMap with the given suffix appended to the name.
// The name for the CongifMap is based on the name of the given ArgCD.
func NewConfigMapWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) *corev1.ConfigMap {
	return NewConfigMapWithName(fmt.Sprintf("%s-%s", cr.ObjectMeta.Name, suffix), cr)
}

// GetDexConfig returns default dex config
func GetDexConfig(cr *argoprojv1a1.ArgoCD) string {
	config := common.ArgoCDDefaultDexConfig
	if len(cr.Spec.Dex.Config) > 0 {
		config = cr.Spec.Dex.Config
	}
	return config
}

// GetApplicationInstanceLabelKey will return the application instance label key  for the given ArgoCD.
func GetApplicationInstanceLabelKey(cr *argoprojv1a1.ArgoCD) string {
	key := common.ArgoCDDefaultApplicationInstanceLabelKey
	if len(cr.Spec.ApplicationInstanceLabelKey) > 0 {
		key = cr.Spec.ApplicationInstanceLabelKey
	}
	return key
}

// GetConfigManagementPlugins will return the config management plugins for the given ArgoCD.
func GetConfigManagementPlugins(cr *argoprojv1a1.ArgoCD) string {
	plugins := common.ArgoCDDefaultConfigManagementPlugins
	if len(cr.Spec.ConfigManagementPlugins) > 0 {
		plugins = cr.Spec.ConfigManagementPlugins
	}
	return plugins
}

// GetGATrackingID will return the google analytics tracking ID for the given Argo CD.
func GetGATrackingID(cr *argoprojv1a1.ArgoCD) string {
	id := common.ArgoCDDefaultGATrackingID
	if len(cr.Spec.GATrackingID) > 0 {
		id = cr.Spec.GATrackingID
	}
	return id
}

// GetHelpChatURL will return the help chat URL for the given Argo CD.
func GetHelpChatURL(cr *argoprojv1a1.ArgoCD) string {
	url := common.ArgoCDDefaultHelpChatURL
	if len(cr.Spec.HelpChatURL) > 0 {
		url = cr.Spec.HelpChatURL
	}
	return url
}

// GetHelpChatText will return the help chat text for the given Argo CD.
func GetHelpChatText(cr *argoprojv1a1.ArgoCD) string {
	text := common.ArgoCDDefaultHelpChatText
	if len(cr.Spec.HelpChatText) > 0 {
		text = cr.Spec.HelpChatText
	}
	return text
}

// GetKustomizeBuildOptions will return the kuztomize build options for the given ArgoCD.
func GetKustomizeBuildOptions(cr *argoprojv1a1.ArgoCD) string {
	kbo := common.ArgoCDDefaultKustomizeBuildOptions
	if len(cr.Spec.KustomizeBuildOptions) > 0 {
		kbo = cr.Spec.KustomizeBuildOptions
	}
	return kbo
}

// GetOIDCConfig will return the OIDC configuration for the given ArgoCD.
func GetOIDCConfig(cr *argoprojv1a1.ArgoCD) string {
	config := common.ArgoCDDefaultOIDCConfig
	if len(cr.Spec.OIDCConfig) > 0 {
		config = cr.Spec.OIDCConfig
	}
	return config
}

// GetResourceCustomizations will return the resource customizations for the given ArgoCD.
func GetResourceCustomizations(cr *argoprojv1a1.ArgoCD) string {
	rc := common.ArgoCDDefaultResourceCustomizations
	if cr.Spec.ResourceCustomizations != "" {
		rc = cr.Spec.ResourceCustomizations
	}
	return rc
}

// GetResourceExclusions will return the resource exclusions for the given ArgoCD.
func GetResourceExclusions(cr *argoprojv1a1.ArgoCD) string {
	re := common.ArgoCDDefaultResourceExclusions
	if cr.Spec.ResourceExclusions != "" {
		re = cr.Spec.ResourceExclusions
	}
	return re
}

// GetResourceInclusions will return the resource inclusions for the given ArgoCD.
func GetResourceInclusions(cr *argoprojv1a1.ArgoCD) string {
	re := common.ArgoCDDefaultResourceInclusions
	if cr.Spec.ResourceInclusions != "" {
		re = cr.Spec.ResourceInclusions
	}
	return re
}

// GetInitialRepositories will return the initial repositories for the given ArgoCD.
func GetInitialRepositories(cr *argoprojv1a1.ArgoCD) string {
	repos := common.ArgoCDDefaultRepositories
	if len(cr.Spec.InitialRepositories) > 0 {
		repos = cr.Spec.InitialRepositories
	}
	return repos
}

// GetRepositoryCredentials will return the repository credentials for the given ArgoCD.
func GetRepositoryCredentials(cr *argoprojv1a1.ArgoCD) string {
	repos := common.ArgoCDDefaultRepositoryCredentials
	if len(cr.Spec.RepositoryCredentials) > 0 {
		repos = cr.Spec.RepositoryCredentials
	}
	return repos
}

// GetInitialSSHKnownHosts will return the SSH Known Hosts data for the given ArgoCD.
func GetInitialSSHKnownHosts(cr *argoprojv1a1.ArgoCD) string {
	skh := common.ArgoCDDefaultSSHKnownHosts
	if cr.Spec.InitialSSHKnownHosts.ExcludeDefaultHosts {
		skh = ""
	}
	if len(cr.Spec.InitialSSHKnownHosts.Keys) > 0 {
		skh += cr.Spec.InitialSSHKnownHosts.Keys
	}
	return skh
}

// GetInitialTLSCerts will return the TLS certs for the given ArgoCD.
func GetInitialTLSCerts(cr *argoprojv1a1.ArgoCD) map[string]string {
	certs := make(map[string]string)
	if len(cr.Spec.TLS.InitialCerts) > 0 {
		certs = cr.Spec.TLS.InitialCerts
	}
	return certs
}
