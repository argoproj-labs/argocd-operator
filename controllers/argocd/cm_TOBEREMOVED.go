package argocd

import (
	"context"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// getRBACPolicy will return the RBAC policy for the given ArgoCD.
func getRBACPolicy(cr *argoproj.ArgoCD) string {
	policy := common.ArgoCDDefaultRBACPolicy
	if cr.Spec.RBAC.Policy != nil {
		policy = *cr.Spec.RBAC.Policy
	}
	return policy
}

// getRBACDefaultPolicy will retun the RBAC default policy for the given ArgoCD.
func getRBACDefaultPolicy(cr *argoproj.ArgoCD) string {
	dp := common.ArgoCDDefaultRBACDefaultPolicy
	if cr.Spec.RBAC.DefaultPolicy != nil {
		dp = *cr.Spec.RBAC.DefaultPolicy
	}
	return dp
}

// getRBACScopes will return the RBAC scopes for the given ArgoCD.
func getRBACScopes(cr *argoproj.ArgoCD) string {
	scopes := common.ArgoCDDefaultRBACScopes
	if cr.Spec.RBAC.Scopes != nil {
		scopes = *cr.Spec.RBAC.Scopes
	}
	return scopes
}

// createRBACConfigMap will create the Argo CD RBAC ConfigMap resource.
func (r *ReconcileArgoCD) createRBACConfigMap(cm *corev1.ConfigMap, cr *argoproj.ArgoCD) error {
	data := make(map[string]string)
	data[common.ArgoCDKeyRBACPolicyCSV] = getRBACPolicy(cr)
	data[common.ArgoCDKeyRBACPolicyDefault] = getRBACDefaultPolicy(cr)
	data[common.ArgoCDKeyRBACScopes] = getRBACScopes(cr)
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRBAC will ensure that the ArgoCD RBAC ConfigMap is present.
func (r *ReconcileArgoCD) reconcileRBAC(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDRBACConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return r.reconcileRBACConfigMap(cm, cr)
	}
	return r.createRBACConfigMap(cm, cr)
}

// reconcileRBACConfigMap will ensure that the RBAC ConfigMap is syncronized with the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRBACConfigMap(cm *corev1.ConfigMap, cr *argoproj.ArgoCD) error {
	changed := false
	// Policy CSV
	if cr.Spec.RBAC.Policy != nil && cm.Data[common.ArgoCDKeyRBACPolicyCSV] != *cr.Spec.RBAC.Policy {
		cm.Data[common.ArgoCDKeyRBACPolicyCSV] = *cr.Spec.RBAC.Policy
		changed = true
	}

	// Default Policy
	if cr.Spec.RBAC.DefaultPolicy != nil && cm.Data[common.ArgoCDKeyRBACPolicyDefault] != *cr.Spec.RBAC.DefaultPolicy {
		cm.Data[common.ArgoCDKeyRBACPolicyDefault] = *cr.Spec.RBAC.DefaultPolicy
		changed = true
	}

	// Default Policy Matcher Mode
	if cr.Spec.RBAC.PolicyMatcherMode != nil && cm.Data[common.ArgoCDPolicyMatcherMode] != *cr.Spec.RBAC.PolicyMatcherMode {
		cm.Data[common.ArgoCDPolicyMatcherMode] = *cr.Spec.RBAC.PolicyMatcherMode
		changed = true
	}

	// Scopes
	if cr.Spec.RBAC.Scopes != nil && cm.Data[common.ArgoCDKeyRBACScopes] != *cr.Spec.RBAC.Scopes {
		cm.Data[common.ArgoCDKeyRBACScopes] = *cr.Spec.RBAC.Scopes
		changed = true
	}

	if changed {
		// TODO: Reload server (and dex?) if RBAC settings change?
		return r.Client.Update(context.TODO(), cm)
	}
	return nil // ConfigMap exists and nothing to do, move along...
}

// getApplicationInstanceLabelKey will return the application instance label key  for the given ArgoCD.
func getApplicationInstanceLabelKey(cr *argoproj.ArgoCD) string {
	key := common.ArgoCDDefaultApplicationInstanceLabelKey
	if len(cr.Spec.ApplicationInstanceLabelKey) > 0 {
		key = cr.Spec.ApplicationInstanceLabelKey
	}
	return key
}

// getCAConfigMapName will return the CA ConfigMap name for the given ArgoCD.
func getCAConfigMapName(cr *argoproj.ArgoCD) string {
	if len(cr.Spec.TLS.CA.ConfigMapName) > 0 {
		return cr.Spec.TLS.CA.ConfigMapName
	}
	return nameWithSuffix(common.ArgoCDCASuffix, cr)
}

// getSCMRootCAConfigMapName will return the SCMRootCA ConfigMap name for the given ArgoCD ApplicationSet Controller.
func getSCMRootCAConfigMapName(cr *argoproj.ArgoCD) string {
	if cr.Spec.ApplicationSet.SCMRootCAConfigMap != "" && len(cr.Spec.ApplicationSet.SCMRootCAConfigMap) > 0 {
		return cr.Spec.ApplicationSet.SCMRootCAConfigMap
	}
	return ""
}

// getConfigManagementPlugins will return the config management plugins for the given ArgoCD.
func getConfigManagementPlugins(cr *argoproj.ArgoCD) string {
	plugins := common.ArgoCDDefaultConfigManagementPlugins
	if len(cr.Spec.ConfigManagementPlugins) > 0 {
		plugins = cr.Spec.ConfigManagementPlugins
	}
	return plugins
}

// getGATrackingID will return the google analytics tracking ID for the given Argo CD.
func getGATrackingID(cr *argoproj.ArgoCD) string {
	id := common.ArgoCDDefaultGATrackingID
	if len(cr.Spec.GATrackingID) > 0 {
		id = cr.Spec.GATrackingID
	}
	return id
}

// getHelpChatURL will return the help chat URL for the given Argo CD.
func getHelpChatURL(cr *argoproj.ArgoCD) string {
	url := common.ArgoCDDefaultHelpChatURL
	if len(cr.Spec.HelpChatURL) > 0 {
		url = cr.Spec.HelpChatURL
	}
	return url
}

// getHelpChatText will return the help chat text for the given Argo CD.
func getHelpChatText(cr *argoproj.ArgoCD) string {
	text := common.ArgoCDDefaultHelpChatText
	if len(cr.Spec.HelpChatText) > 0 {
		text = cr.Spec.HelpChatText
	}
	return text
}

// getKustomizeBuildOptions will return the kuztomize build options for the given ArgoCD.
func getKustomizeBuildOptions(cr *argoproj.ArgoCD) string {
	kbo := common.ArgoCDDefaultKustomizeBuildOptions
	if len(cr.Spec.KustomizeBuildOptions) > 0 {
		kbo = cr.Spec.KustomizeBuildOptions
	}
	return kbo
}

// getOIDCConfig will return the OIDC configuration for the given ArgoCD.
func getOIDCConfig(cr *argoproj.ArgoCD) string {
	config := common.ArgoCDDefaultOIDCConfig
	if len(cr.Spec.OIDCConfig) > 0 {
		config = cr.Spec.OIDCConfig
	}
	return config
}

// getResourceExclusions will return the resource exclusions for the given ArgoCD.
func getResourceExclusions(cr *argoproj.ArgoCD) string {
	re := common.ArgoCDDefaultResourceExclusions
	if cr.Spec.ResourceExclusions != "" {
		re = cr.Spec.ResourceExclusions
	}
	return re
}

// getResourceInclusions will return the resource inclusions for the given ArgoCD.
func getResourceInclusions(cr *argoproj.ArgoCD) string {
	re := common.ArgoCDDefaultResourceInclusions
	if cr.Spec.ResourceInclusions != "" {
		re = cr.Spec.ResourceInclusions
	}
	return re
}

// getInitialRepositories will return the initial repositories for the given ArgoCD.
func getInitialRepositories(cr *argoproj.ArgoCD) string {
	repos := common.ArgoCDDefaultRepositories
	if len(cr.Spec.InitialRepositories) > 0 {
		repos = cr.Spec.InitialRepositories
	}
	return repos
}

// getRepositoryCredentials will return the repository credentials for the given ArgoCD.
func getRepositoryCredentials(cr *argoproj.ArgoCD) string {
	repos := common.ArgoCDDefaultRepositoryCredentials
	if len(cr.Spec.RepositoryCredentials) > 0 {
		repos = cr.Spec.RepositoryCredentials
	}
	return repos
}

// getSSHKnownHosts will return the SSH Known Hosts data for the given ArgoCD.
func getInitialSSHKnownHosts(cr *argoproj.ArgoCD) string {
	skh := common.ArgoCDDefaultSSHKnownHosts
	if cr.Spec.InitialSSHKnownHosts.ExcludeDefaultHosts {
		skh = ""
	}
	if len(cr.Spec.InitialSSHKnownHosts.Keys) > 0 {
		skh += cr.Spec.InitialSSHKnownHosts.Keys
	}
	return skh
}

// reconcileSSHKnownHosts will ensure that the ArgoCD SSH Known Hosts ConfigMap is present.
func (r *ReconcileArgoCD) reconcileSSHKnownHosts(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDKnownHostsConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, move along...
	}

	cm.Data = map[string]string{
		common.ArgoCDKeySSHKnownHosts: getInitialSSHKnownHosts(cr),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// getTLSCerts will return the TLS certs for the given ArgoCD.
func getInitialTLSCerts(cr *argoproj.ArgoCD) map[string]string {
	certs := make(map[string]string)
	if len(cr.Spec.TLS.InitialCerts) > 0 {
		certs = cr.Spec.TLS.InitialCerts
	}
	return certs
}

// reconcileTLSCerts will ensure that the ArgoCD TLS Certs ConfigMap is present.
func (r *ReconcileArgoCD) reconcileTLSCerts(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDTLSCertsConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, move along...
	}

	cm.Data = getInitialTLSCerts(cr)

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileGPGKeysConfigMap creates a gpg-keys config map
func (r *ReconcileArgoCD) reconcileGPGKeysConfigMap(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDGPGKeysConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil
	}
	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}
