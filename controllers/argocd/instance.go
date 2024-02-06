package argocd

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

// getValueOrDefault returns the value if it's non-empty, otherwise returns the default value.
func (r *ArgoCDReconciler) getValueOrDefault(value interface{}, defaultValue interface{}) interface{} {
	if util.IsPtr(value) {
		if reflect.ValueOf(value).IsNil() {
			return defaultValue
		}
		return reflect.ValueOf(value).String()
	}

	switch v := value.(type) {
	case string:
		if len(v) > 0 {
			return v
		}
		return defaultValue
	case map[string]string:
		if len(v) > 0 {
			return v
		}
		return defaultValue
	}

	return defaultValue
}

// getApplicationInstanceLabelKey returns the application instance label key for the given ArgoCD.
func (r *ArgoCDReconciler) getApplicationInstanceLabelKey() string {
	return r.getValueOrDefault(r.Instance.Spec.ApplicationInstanceLabelKey, common.ArgoCDDefaultApplicationInstanceLabelKey).(string)
}

// getCAConfigMapName returns the CA ConfigMap name for the given ArgoCD.
func (r *ArgoCDReconciler) getCAConfigMapName() string {
	return r.getValueOrDefault(r.Instance.Spec.TLS.CA.ConfigMapName, nameWithSuffix(common.ArgoCDCASuffix, r.Instance)).(string)
}

// getSCMRootCAConfigMapName returns the SCMRootCA ConfigMap name for the given ArgoCD ApplicationSet Controller.
func (r *ArgoCDReconciler) getSCMRootCAConfigMapName() string {
	return r.getValueOrDefault(r.Instance.Spec.ApplicationSet.SCMRootCAConfigMap, "").(string)
}

// getConfigManagementPlugins returns the config management plugins for the given ArgoCD.
func (r *ArgoCDReconciler) getConfigManagementPlugins() string {
	return r.getValueOrDefault(r.Instance.Spec.ConfigManagementPlugins, common.ArgoCDDefaultConfigManagementPlugins).(string)
}

// getGATrackingID returns the google analytics tracking ID for the given Argo CD.
func (r *ArgoCDReconciler) getGATrackingID() string {
	return r.getValueOrDefault(r.Instance.Spec.GATrackingID, common.ArgoCDDefaultGATrackingID).(string)
}

// getHelpChatURL returns the help chat URL for the given Argo CD.
func (r *ArgoCDReconciler) getHelpChatURL() string {
	return r.getValueOrDefault(r.Instance.Spec.HelpChatURL, common.ArgoCDDefaultHelpChatURL).(string)
}

// getHelpChatText returns the help chat text for the given Argo CD.
func (r *ArgoCDReconciler) getHelpChatText() string {
	return r.getValueOrDefault(r.Instance.Spec.HelpChatText, common.ArgoCDDefaultHelpChatText).(string)
}

// getKustomizeBuildOptions returns the kuztomize build options for the given ArgoCD.
func (r *ArgoCDReconciler) getKustomizeBuildOptions() string {
	return r.getValueOrDefault(r.Instance.Spec.KustomizeBuildOptions, common.ArgoCDDefaultKustomizeBuildOptions).(string)
}

// getOIDCConfig returns the OIDC configuration for the given ArgoCD.
func (r *ArgoCDReconciler) getOIDCConfig() string {
	return r.getValueOrDefault(r.Instance.Spec.OIDCConfig, common.ArgoCDDefaultOIDCConfig).(string)
}

// getRBACPolicy will return the RBAC policy for the given ArgoCD.
func (r *ArgoCDReconciler) getRBACPolicy() string {
	return r.getValueOrDefault(r.Instance.Spec.RBAC.Policy, common.ArgoCDDefaultRBACPolicy).(string)
}

// getRBACPolicyMatcherMode will return the RBAC policy matcher mode for the given ArgoCD.
func (r *ArgoCDReconciler) getRBACPolicyMatcherMode() string {
	return r.getValueOrDefault(r.Instance.Spec.RBAC.PolicyMatcherMode, common.ArgoCDPolicyMatcherMode).(string)
}

// getRBACDefaultPolicy will return the RBAC default policy for the given ArgoCD.
func (r *ArgoCDReconciler) getRBACDefaultPolicy() string {
	return r.getValueOrDefault(r.Instance.Spec.RBAC.DefaultPolicy, common.ArgoCDDefaultRBACPolicy).(string)
}

// getRBACScopes will return the RBAC scopes for the given ArgoCD.
func (r *ArgoCDReconciler) getRBACScopes() string {
	return r.getValueOrDefault(r.Instance.Spec.RBAC.Scopes, common.ArgoCDDefaultRBACScopes).(string)
}

// getResourceExclusions will return the resource exclusions for the given ArgoCD.
func (r *ArgoCDReconciler) getResourceExclusions() string {
	return r.getValueOrDefault(r.Instance.Spec.ResourceExclusions, common.ArgoCDDefaultResourceExclusions).(string)
}

// getResourceInclusions will return the resource inclusions for the given ArgoCD.
func (r *ArgoCDReconciler) getResourceInclusions() string {
	return r.getValueOrDefault(r.Instance.Spec.ResourceInclusions, common.ArgoCDDefaultResourceInclusions).(string)
}

// getInitialRepositories will return the initial repositories for the given ArgoCD.
func (r *ArgoCDReconciler) getInitialRepositories() string {
	return r.getValueOrDefault(r.Instance.Spec.InitialRepositories, common.ArgoCDDefaultRepositories).(string)
}

// getRepositoryCredentials will return the repository credentials for the given ArgoCD.
func (r *ArgoCDReconciler) getRepositoryCredentials() string {
	return r.getValueOrDefault(r.Instance.Spec.RepositoryCredentials, common.ArgoCDDefaultRepositoryCredentials).(string)
}

// getInitialTLSCerts will return the TLS certs for the given ArgoCD.
func (r *ArgoCDReconciler) getInitialTLSCerts() map[string]string {
	return r.getValueOrDefault(r.Instance.Spec.RepositoryCredentials, make(map[string]string)).(map[string]string)
}

// getSSHKnownHosts will return the SSH Known Hosts data for the given ArgoCD.
func (r *ArgoCDReconciler) getInitialSSHKnownHosts() string {
	skh := common.ArgoCDDefaultSSHKnownHosts
	if r.Instance.Spec.InitialSSHKnownHosts.ExcludeDefaultHosts {
		skh = ""
	}
	if len(r.Instance.Spec.InitialSSHKnownHosts.Keys) > 0 {
		skh += r.Instance.Spec.InitialSSHKnownHosts.Keys
	}
	return skh
}
