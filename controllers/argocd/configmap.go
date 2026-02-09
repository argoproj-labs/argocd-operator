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

package argocd

import (
	"context"
	"fmt"
	"reflect"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

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
	argoutil.LogResourceCreation(log, cm)
	return r.Create(context.TODO(), cm)
}

// getApplicationInstanceLabelKey will return the application instance label key  for the given ArgoCD.
func getApplicationInstanceLabelKey(cr *argoproj.ArgoCD) string {
	key := common.ArgoCDDefaultApplicationInstanceLabelKey
	if len(cr.Spec.ApplicationInstanceLabelKey) > 0 {
		key = cr.Spec.ApplicationInstanceLabelKey
	}
	return key
}

// setRespectRBAC configures RespectRBAC key and value for ConfigMap.
func setRespectRBAC(cr *argoproj.ArgoCD, data map[string]string) map[string]string {
	if cr.Spec.Controller.RespectRBAC != "" &&
		(cr.Spec.Controller.RespectRBAC == common.ArgoCDValueRespectRBACStrict || cr.Spec.Controller.RespectRBAC == common.ArgoCDValueRespectRBACNormal) {
		data[common.ArgoCDKeyRespectRBAC] = cr.Spec.Controller.RespectRBAC
	}
	return data
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

// getResourceHealthChecks loads health customizations to `resource.customizations.health` from argocd-cm ConfigMap
func getResourceHealthChecks(cr *argoproj.ArgoCD) map[string]string {
	healthCheck := make(map[string]string)
	if cr.Spec.ResourceHealthChecks != nil {
		resourceHealthChecks := cr.Spec.ResourceHealthChecks
		for _, healthCustomization := range resourceHealthChecks {
			if healthCustomization.Group != "" {
				healthCustomization.Group += "_"
			}
			subkey := "resource.customizations.health." + healthCustomization.Group + healthCustomization.Kind
			subvalue := healthCustomization.Check
			healthCheck[subkey] = subvalue
		}
	}
	return healthCheck
}

// getResourceIgnoreDifferences loads ignore differences customizations to `resource.customizations.ignoreDifferences` from argocd-cm ConfigMap
func getResourceIgnoreDifferences(cr *argoproj.ArgoCD) (map[string]string, error) {
	ignoreDiff := make(map[string]string)
	if cr.Spec.ResourceIgnoreDifferences != nil {
		resourceIgnoreDiff := cr.Spec.ResourceIgnoreDifferences
		if !reflect.DeepEqual(resourceIgnoreDiff.All, &argoproj.IgnoreDifferenceCustomization{}) {
			subkey := "resource.customizations.ignoreDifferences.all"
			bytes, err := yaml.Marshal(resourceIgnoreDiff.All)
			if err != nil {
				return ignoreDiff, err
			}
			subvalue := string(bytes)
			ignoreDiff[subkey] = subvalue
		}
		for _, ignoreDiffCustomization := range resourceIgnoreDiff.ResourceIdentifiers {
			if ignoreDiffCustomization.Group != "" {
				ignoreDiffCustomization.Group += "_"
			}
			subkey := "resource.customizations.ignoreDifferences." + ignoreDiffCustomization.Group + ignoreDiffCustomization.Kind
			bytes, err := yaml.Marshal(ignoreDiffCustomization.Customization)
			if err != nil {
				return ignoreDiff, err
			}
			subvalue := string(bytes)
			ignoreDiff[subkey] = subvalue
		}
	}
	return ignoreDiff, nil
}

// getResourceActions loads custom actions to `resource.customizations.actions` from argocd-cm ConfigMap
func getResourceActions(cr *argoproj.ArgoCD) map[string]string {
	action := make(map[string]string)
	if cr.Spec.ResourceActions != nil {
		resourceAction := cr.Spec.ResourceActions
		for _, actionCustomization := range resourceAction {
			if actionCustomization.Group != "" {
				actionCustomization.Group += "_"
			}
			subkey := "resource.customizations.actions." + actionCustomization.Group + actionCustomization.Kind
			subvalue := actionCustomization.Action
			action[subkey] = subvalue
		}
	}
	return action
}

// getResourceExclusions returns resource exclusions from the CR or defaults if not set.
func getResourceExclusions(cr *argoproj.ArgoCD) (string, error) {

	// Use CR value if provided
	if cr.Spec.ResourceExclusions != "" {
		return cr.Spec.ResourceExclusions, nil
	}

	// Use defaults
	defaultExclusions := getDefaultResourceExclusions()
	yamlData, err := yaml.Marshal(defaultExclusions)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource exclusions: %v", err)
	}
	return string(yamlData), nil

}

// getResourceInclusions will return the resource inclusions for the given ArgoCD.
func getResourceInclusions(cr *argoproj.ArgoCD) string {
	re := common.ArgoCDDefaultResourceInclusions
	if cr.Spec.ResourceInclusions != "" {
		re = cr.Spec.ResourceInclusions
	}
	return re
}

// getResourceTrackingMethod will return the resource tracking method for the given ArgoCD.
func getResourceTrackingMethod(cr *argoproj.ArgoCD) string {
	rtm := argoproj.ParseResourceTrackingMethod(cr.Spec.ResourceTrackingMethod)
	if rtm == argoproj.ResourceTrackingMethodInvalid {
		log.Info(fmt.Sprintf("Found '%s' as resource tracking method, which is invalid. Using default 'Annotation' method.", cr.Spec.ResourceTrackingMethod))
	} else if cr.Spec.ResourceTrackingMethod != "" {
		log.Info(fmt.Sprintf("Found '%s' as tracking method", cr.Spec.ResourceTrackingMethod))
	} else {
		log.Info("Using default resource tracking method 'Annotation'")
	}
	return rtm.String()
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

// getTLSCerts will return the TLS certs for the given ArgoCD.
func getInitialTLSCerts(cr *argoproj.ArgoCD) map[string]string {
	certs := make(map[string]string)
	if len(cr.Spec.TLS.InitialCerts) > 0 {
		certs = cr.Spec.TLS.InitialCerts
	}
	return certs
}

// newConfigMap returns a new ConfigMap instance for the given ArgoCD.
func newConfigMap(cr *argoproj.ArgoCD) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
	argoutil.AddTrackedByOperatorLabel(&cm.ObjectMeta)
	return cm
}

// newConfigMapWithName creates a new ConfigMap with the given name for the given ArgCD.
func newConfigMapWithName(name string, cr *argoproj.ArgoCD) *corev1.ConfigMap {
	cm := newConfigMap(cr)
	cm.Name = name

	lbls := cm.Labels
	lbls[common.ArgoCDKeyName] = name
	cm.Labels = lbls

	return cm
}

// reconcileConfigMaps will ensure that all ArgoCD ConfigMaps are present.
func (r *ReconcileArgoCD) reconcileConfigMaps(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	if err := r.reconcileArgoConfigMap(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisConfiguration(cr, useTLSForRedis); err != nil {
		return err
	}

	if err := r.reconcileRBAC(cr); err != nil {
		return err
	}

	if err := r.reconcileSSHKnownHosts(cr); err != nil {
		return err
	}

	if err := r.reconcileTLSCerts(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaConfiguration(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaDashboards(cr); err != nil {
		return err
	}

	if err := r.reconcileArgoCmdParamsConfigMap(cr); err != nil {
		return err
	}

	return r.reconcileGPGKeysConfigMap(cr)
}

// reconcileCAConfigMap will ensure that the Certificate Authority ConfigMap is present.
// This ConfigMap holds the CA Certificate data for client use.
func (r *ReconcileArgoCD) reconcileCAConfigMap(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(getCAConfigMapName(cr), cr)

	configMapExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm)
	if err != nil {
		return err
	}
	if configMapExists {
		return nil // ConfigMap found, do nothing
	}

	caSecret := argoutil.NewSecretWithSuffix(cr, common.ArgoCDCASuffix)
	caSecretExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, caSecret.Name, caSecret)
	if err != nil {
		return err
	}
	if !caSecretExists {
		log.Info(fmt.Sprintf("ca secret [%s] not found, waiting to reconcile ca configmap [%s]", caSecret.Name, cm.Name))
		return nil
	}

	cm.Data = map[string]string{
		common.ArgoCDKeyTLSCert: string(caSecret.Data[common.ArgoCDKeyTLSCert]),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, cm)
	return r.Create(context.TODO(), cm)
}

// reconcileConfiguration will ensure that the main ConfigMap for ArgoCD is present.
func (r *ReconcileArgoCD) reconcileArgoConfigMap(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDConfigMapName, cr)
	cm.Data = make(map[string]string)
	cm.Data = setRespectRBAC(cr, cm.Data)
	cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] = getApplicationInstanceLabelKey(cr)
	cm.Data[common.ArgoCDKeyAdminEnabled] = fmt.Sprintf("%t", !cr.Spec.DisableAdmin)
	cm.Data[common.ArgoCDKeyGATrackingID] = getGATrackingID(cr)
	cm.Data[common.ArgoCDKeyGAAnonymizeUsers] = fmt.Sprint(cr.Spec.GAAnonymizeUsers)
	cm.Data[common.ArgoCDKeyHelpChatURL] = getHelpChatURL(cr)
	cm.Data[common.ArgoCDKeyHelpChatText] = getHelpChatText(cr)
	cm.Data[common.ArgoCDKeyKustomizeBuildOptions] = getKustomizeBuildOptions(cr)

	// Set installationID as a top-level key
	if cr.Spec.InstallationID != "" {
		cm.Data[common.ArgoCDKeyInstallationID] = cr.Spec.InstallationID
	}

	if len(cr.Spec.KustomizeVersions) > 0 {
		for _, kv := range cr.Spec.KustomizeVersions {
			cm.Data["kustomize.version."+kv.Version] = kv.Path
		}
	}

	cm.Data[common.ArgoCDKeyOIDCConfig] = getOIDCConfig(cr)

	if c := getResourceHealthChecks(cr); c != nil {
		for k, v := range c {
			cm.Data[k] = v
		}
	}

	if c, err := getResourceIgnoreDifferences(cr); c != nil && err == nil {
		for k, v := range c {
			cm.Data[k] = v
		}
	} else {
		return err
	}

	if c := getResourceActions(cr); c != nil {
		for k, v := range c {
			cm.Data[k] = v
		}
	}

	resourceExclusions, err := getResourceExclusions(cr)
	if err != nil {
		return err
	}
	cm.Data[common.ArgoCDKeyResourceExclusions] = resourceExclusions
	cm.Data[common.ArgoCDKeyResourceInclusions] = getResourceInclusions(cr)
	cm.Data[common.ArgoCDKeyResourceTrackingMethod] = getResourceTrackingMethod(cr)
	cm.Data[common.ArgoCDKeyStatusBadgeEnabled] = fmt.Sprint(cr.Spec.StatusBadgeEnabled)

	serverURI, err := r.getArgoServerURI(cr)
	if err != nil {
		return err
	}
	cm.Data[common.ArgoCDKeyServerURL] = serverURI
	cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] = fmt.Sprint(cr.Spec.UsersAnonymousEnabled)

	// deprecated: log warning for deprecated field InitialRepositories
	//lint:ignore SA1019 known to be deprecated
	if cr.Spec.InitialRepositories != "" { //nolint:staticcheck // SA1019: We must test deprecated fields.
		log.Info(initialRepositoriesWarning)
	}
	// deprecated: log warning for deprecated field RepositoryCredential
	//lint:ignore SA1019 known to be deprecated
	if cr.Spec.RepositoryCredentials != "" { //nolint:staticcheck // SA1019: We must test deprecated fields.
		log.Info(repositoryCredentialsWarning)
	}

	// create dex config if dex is enabled through `.spec.sso`
	if UseDex(cr) {
		dexConfig := getDexConfig(cr)

		// Append the default OpenShift dex config if the openShiftOAuth is requested through `.spec.sso.dex`.
		if cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.OpenShiftOAuth && !r.IsExternalAuthenticationEnabledForOpenShiftCluster {
			cfg, err := r.getOpenShiftDexConfig(cr)
			if err != nil {
				return err
			}
			dexConfig = cfg
		}
		cm.Data[common.ArgoCDKeyDexConfig] = dexConfig
	}

	if cr.Spec.Banner != nil {
		if cr.Spec.Banner.Content != "" {
			cm.Data[common.ArgoCDKeyBannerContent] = cr.Spec.Banner.Content
			if cr.Spec.Banner.URL != "" {
				cm.Data[common.ArgoCDKeyBannerURL] = cr.Spec.Banner.URL
			}
			if cr.Spec.Banner.Permanent {
				cm.Data[common.ArgoCDKeyBannerPermanent] = "true"
			}
			if cr.Spec.Banner.Position != "" {
				cm.Data[common.ArgoCDKeyBannerPosition] = cr.Spec.Banner.Position
			}
		}
	}

	// Find all users explicitly defined via extraConfig
	legacyUsers := localUsersInExtraConfig(cr)

	// Create local users
	for _, user := range cr.Spec.LocalUsers {
		// Ignore any user defined via extraConfig
		if legacyUsers[user.Name] {
			continue
		}
		key := "accounts." + user.Name
		if (user.ApiKey == nil || *user.ApiKey) && user.Login {
			cm.Data[key] = "apiKey, login"
		} else if user.ApiKey == nil || *user.ApiKey {
			cm.Data[key] = "apiKey"
		} else if user.Login {
			cm.Data[key] = "login"
		}
		if user.Enabled == nil || *user.Enabled {
			cm.Data[key+".enabled"] = "true"
		} else {
			cm.Data[key+".enabled"] = "false"
		}
	}

	if len(cr.Spec.ExtraConfig) > 0 {
		for k, v := range cr.Spec.ExtraConfig {
			cm.Data[k] = v
		}
	}

	// Check and set default value for server.rbac.disableApplicationFineGrainedRBACInheritance if not present
	if _, exists := cm.Data[common.ArgoCDServerRBACDisableFineGrainedInheritance]; !exists {
		cm.Data[common.ArgoCDServerRBACDisableFineGrainedInheritance] = "false"
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}

	existingCM := &corev1.ConfigMap{}
	found, err := argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, existingCM)
	if err != nil {
		return err
	}
	if found {

		// reconcile dex configuration if dex is enabled `.spec.sso.dex.provider` or there is
		// existing dex configuration
		if UseDex(cr) {
			if err := r.reconcileDexConfiguration(existingCM, cr); err != nil {
				return err
			}
			cm.Data[common.ArgoCDKeyDexConfig] = existingCM.Data[common.ArgoCDKeyDexConfig]
		} else if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeKeycloak {
			log.Info("Keycloak SSO provider is no longer supported. Existing configuration will be ignored and not reconciled.")
			// Keycloak functionality has been removed, skipping reconciliation
		}

		changed := false
		if !reflect.DeepEqual(cm.Data, existingCM.Data) {
			existingCM.Data = cm.Data
			changed = true
		}

		// Check OwnerReferences
		var refChanged bool
		var err error
		if refChanged, err = modifyOwnerReferenceIfNeeded(cr, existingCM, r.Scheme); err != nil {
			return err
		}

		if refChanged {
			changed = true
		}

		if changed {
			explanation := "updating data"
			if refChanged {
				explanation += ", owner reference"
			}
			argoutil.LogResourceUpdate(log, existingCM, explanation)
			return r.Update(context.TODO(), existingCM)
		}
		return nil // Do nothing as there is no change in the configmap.
	}
	argoutil.LogResourceCreation(log, cm)
	return r.Create(context.TODO(), cm)

}

// reconcileGrafanaConfiguration will ensure that the Grafana configuration ConfigMap is present.
func (r *ReconcileArgoCD) reconcileGrafanaConfiguration(cr *argoproj.ArgoCD) error {
	//lint:ignore SA1019 known to be deprecated
	if !cr.Spec.Grafana.Enabled { //nolint:staticcheck // SA1019: We must test deprecated fields.
		return nil // Grafana not enabled, do nothing.
	}

	log.Info(grafanaDeprecatedWarning)

	return nil
}

// reconcileGrafanaDashboards will ensure that the Grafana dashboards ConfigMap is present.
func (r *ReconcileArgoCD) reconcileGrafanaDashboards(cr *argoproj.ArgoCD) error {
	//lint:ignore SA1019 known to be deprecated
	if !cr.Spec.Grafana.Enabled { //nolint:staticcheck // SA1019: We must test deprecated fields.
		return nil // Grafana not enabled, do nothing.
	}

	log.Info(grafanaDeprecatedWarning)

	return nil
}

// reconcileRBAC will ensure that the ArgoCD RBAC ConfigMap is present.
func (r *ReconcileArgoCD) reconcileRBAC(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDRBACConfigMapName, cr)

	found, err := argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm)
	if err != nil {
		return err
	}
	if found {
		return r.reconcileRBACConfigMap(cm, cr)
	}
	return r.createRBACConfigMap(cm, cr)
}

// reconcileRBACConfigMap will ensure that the RBAC ConfigMap is syncronized with the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRBACConfigMap(cm *corev1.ConfigMap, cr *argoproj.ArgoCD) error {
	changed := false
	explanation := ""
	// Policy CSV
	if cr.Spec.RBAC.Policy != nil && cm.Data[common.ArgoCDKeyRBACPolicyCSV] != *cr.Spec.RBAC.Policy {
		cm.Data[common.ArgoCDKeyRBACPolicyCSV] = *cr.Spec.RBAC.Policy
		explanation = "rbac policy"
		changed = true
	}

	// Default Policy
	if cr.Spec.RBAC.DefaultPolicy != nil && cm.Data[common.ArgoCDKeyRBACPolicyDefault] != *cr.Spec.RBAC.DefaultPolicy {
		cm.Data[common.ArgoCDKeyRBACPolicyDefault] = *cr.Spec.RBAC.DefaultPolicy
		if changed {
			explanation += ", "
		}
		explanation += " rbac default policy"
		changed = true
	}

	// Default Policy Matcher Mode
	if cr.Spec.RBAC.PolicyMatcherMode != nil && cm.Data[common.ArgoCDPolicyMatcherMode] != *cr.Spec.RBAC.PolicyMatcherMode {
		cm.Data[common.ArgoCDPolicyMatcherMode] = *cr.Spec.RBAC.PolicyMatcherMode
		if changed {
			explanation += ", "
		}
		explanation += "rbac policy matcher mode"
		changed = true
	}

	// Scopes
	if cr.Spec.RBAC.Scopes != nil && cm.Data[common.ArgoCDKeyRBACScopes] != *cr.Spec.RBAC.Scopes {
		if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeKeycloak {
			log.Info("Keycloak SSO provider is no longer supported. RBAC scopes configuration is ignored.")
		} else {
			cm.Data[common.ArgoCDKeyRBACScopes] = *cr.Spec.RBAC.Scopes
			if changed {
				explanation += ", "
			}
			explanation += "rbac scopes"
			changed = true
		}
	}

	// Check OwnerReferences
	var ownerRefChanged bool
	var err error
	if ownerRefChanged, err = modifyOwnerReferenceIfNeeded(cr, cm, r.Scheme); err != nil {
		return err
	}

	if ownerRefChanged {
		explanation += ", owner reference"
		changed = true
	}

	if changed {
		argoutil.LogResourceUpdate(log, cm, "updating", explanation)
		// TODO: Reload server (and dex?) if RBAC settings change?
		return r.Update(context.TODO(), cm)
	}
	return nil // ConfigMap exists and nothing to do, move along...
}

// modifyOwnerReferenceIfNeeded reverts any changes to the OwnerReference of the
// given config map. Returns true if the owner reference was modified, false if
// not.
func modifyOwnerReferenceIfNeeded(cr *argoproj.ArgoCD, cm *corev1.ConfigMap, scheme *runtime.Scheme) (bool, error) {
	gvk, err := apiutil.GVKForObject(cr, scheme)
	if err != nil {
		return false, err
	}
	changed := false
	// Look for an existing ArgoCD owner reference
	for i := range cm.OwnerReferences {
		ref := &cm.OwnerReferences[i]

		if ref.Kind != gvk.Kind {
			continue
		}
		if ref.APIVersion != gvk.GroupVersion().String() {
			ref.APIVersion = gvk.GroupVersion().String()
			changed = true
		}
		if ref.UID != cr.GetUID() {
			ref.UID = cr.GetUID()
			changed = true
		}
		if ref.Name != cr.GetName() {
			ref.Name = cr.GetName()
			changed = true
		}
		return changed, nil
	}
	// No ArgoCD owner reference found — add one
	if err := controllerutil.SetControllerReference(cr, cm, scheme); err != nil {
		return false, err
	}
	return true, nil
}

// reconcileRedisConfiguration will ensure that all of the Redis ConfigMaps are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisConfiguration(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	if err := r.reconcileRedisHAConfigMap(cr, useTLSForRedis); err != nil {
		return err
	}
	if err := r.reconcileRedisHAHealthConfigMap(cr, useTLSForRedis); err != nil {
		return err
	}
	return nil
}

// reconcileRedisHAConfigMap will ensure that the Redis HA Health ConfigMap is present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisHAHealthConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	ctx := context.TODO()
	cm := newConfigMapWithName(common.ArgoCDRedisHAHealthConfigMapName, cr)
	cm.Data = map[string]string{
		"redis_liveness.sh":    getRedisLivenessScript(useTLSForRedis),
		"redis_readiness.sh":   getRedisReadinessScript(useTLSForRedis),
		"sentinel_liveness.sh": getSentinelLivenessScript(useTLSForRedis),
	}
	existingCM := &corev1.ConfigMap{}
	exists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, existingCM)
	if err != nil {
		return err
	}
	// If HA is disabled, we need to ensure the ConfigMap is deleted
	if !cr.Spec.HA.Enabled {
		if exists {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			argoutil.LogResourceDeletion(log, cm, "redis ha is disabled")
			return r.Delete(ctx, existingCM)
		}
		return nil // Nothing to do since HA is not enabled and ConfigMap does not exist
	}

	//HA: enabled, set owner reference
	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	if !exists {
		// ConfigMap does not exist, create it
		argoutil.LogResourceCreation(log, cm)
		return r.Create(ctx, cm)
	}

	// Update path
	refChanged, err := modifyOwnerReferenceIfNeeded(cr, existingCM, r.Scheme)
	if err != nil {
		return err
	}
	// Check if the data has changed
	dataChanged := !reflect.DeepEqual(cm.Data, existingCM.Data)
	if !refChanged && !dataChanged {
		return nil
	}
	if dataChanged {
		existingCM.Data = cm.Data
	}
	explanation := "updating data"
	if refChanged {
		explanation += ", owner reference"
	}
	argoutil.LogResourceUpdate(log, existingCM, explanation)
	return r.Update(ctx, existingCM)
}

// reconcileRedisHAConfigMap ensures the Redis HA ConfigMap is correctly reconciled.
func (r *ReconcileArgoCD) reconcileRedisHAConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	ctx := context.TODO()
	desired := newConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	desired.Data = map[string]string{
		"haproxy.cfg":     getRedisHAProxyConfig(cr, useTLSForRedis),
		"haproxy_init.sh": getRedisHAProxyScript(cr),
		"init.sh":         getRedisInitScript(cr, useTLSForRedis),
		"redis.conf":      getRedisConf(useTLSForRedis),
		"sentinel.conf":   getRedisSentinelConf(useTLSForRedis),
	}
	existing := &corev1.ConfigMap{}
	exists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, desired.Name, existing)
	if err != nil {
		return err
	}
	// HA disabled: delete ConfigMap if it exists
	if !cr.Spec.HA.Enabled {
		if exists {
			argoutil.LogResourceDeletion(log, existing, "redis ha is disabled")
			return r.Delete(ctx, existing)
		}
		return nil
	}
	// HA enabled: ensure owner reference
	if err := controllerutil.SetControllerReference(cr, desired, r.Scheme); err != nil {
		return err
	}
	// Create if missing
	if !exists {
		argoutil.LogResourceCreation(log, desired)
		return r.Create(ctx, desired)
	}
	// Update path
	refChanged, err := modifyOwnerReferenceIfNeeded(cr, existing, r.Scheme)
	if err != nil {
		return err
	}
	dataChanged := !reflect.DeepEqual(desired.Data, existing.Data)
	if !refChanged && !dataChanged {
		return nil
	}
	if dataChanged {
		existing.Data = desired.Data
	}
	explanation := "updating data"
	if refChanged {
		explanation += ", owner reference"
	}
	argoutil.LogResourceUpdate(log, existing, explanation)
	return r.Update(ctx, existing)
}

func (r *ReconcileArgoCD) recreateRedisHAConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)

	exists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm)
	if err != nil {
		return err
	}
	if exists {
		argoutil.LogResourceDeletion(log, cm, "deleting config map in order to recreate it")
		if err := r.Delete(context.TODO(), cm); err != nil {
			return err
		}
	}
	return r.reconcileRedisHAConfigMap(cr, useTLSForRedis)
}

func (r *ReconcileArgoCD) recreateRedisHAHealthConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAHealthConfigMapName, cr)

	exists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm)
	if err != nil {
		return err
	}
	if exists {
		argoutil.LogResourceDeletion(log, cm, "deleting config map in order to recreate it")
		if err := r.Delete(context.TODO(), cm); err != nil {
			return err
		}
	}
	return r.reconcileRedisHAHealthConfigMap(cr, useTLSForRedis)
}

// reconcileSSHKnownHosts will ensure that the ArgoCD SSH Known Hosts ConfigMap is present.
func (r *ReconcileArgoCD) reconcileSSHKnownHosts(cr *argoproj.ArgoCD) error {
	ctx := context.TODO()
	cm := newConfigMapWithName(common.ArgoCDKnownHostsConfigMapName, cr)
	exists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm)
	if err != nil {
		return err
	}
	if !exists {

		cm.Data = map[string]string{
			common.ArgoCDKeySSHKnownHosts: getInitialSSHKnownHosts(cr),
		}

		if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
			return err
		}
		argoutil.LogResourceCreation(log, cm)
		return r.Create(ctx, cm)
	}
	// update path
	refChanged, err := modifyOwnerReferenceIfNeeded(cr, cm, r.Scheme)
	if err != nil {
		return err
	}
	if refChanged {
		explanation := "updating owner reference"
		argoutil.LogResourceUpdate(log, cm, explanation)
		return r.Update(ctx, cm)
	}
	// No changes required
	return nil
}

// reconcileTLSCerts will ensure that the ArgoCD TLS Certs ConfigMap is present.
func (r *ReconcileArgoCD) reconcileTLSCerts(cr *argoproj.ArgoCD) error {
	ctx := context.TODO()
	cm := newConfigMapWithName(common.ArgoCDTLSCertsConfigMapName, cr)
	exists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm)
	if err != nil {
		return err
	}
	if !exists {
		cm.Data = getInitialTLSCerts(cr)
		if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
			return err
		}
		argoutil.LogResourceCreation(log, cm)
		return r.Create(ctx, cm)
	}
	// update path
	refChanged, err := modifyOwnerReferenceIfNeeded(cr, cm, r.Scheme)
	if err != nil {
		return err
	}
	if refChanged {
		explanation := "updating owner reference"
		argoutil.LogResourceUpdate(log, cm, explanation)
		return r.Update(ctx, cm)
	}
	// No changes required
	return nil
}

// reconcileGPGKeysConfigMap ensures the gpg-keys ConfigMap exists and has the correct owner reference.
func (r *ReconcileArgoCD) reconcileGPGKeysConfigMap(cr *argoproj.ArgoCD) error {
	ctx := context.TODO()
	desired := newConfigMapWithName(common.ArgoCDGPGKeysConfigMapName, cr)
	existing := &corev1.ConfigMap{}
	exists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, desired.Name, existing)
	if err != nil {
		return err
	}
	// Always ensure owner reference is set on the desired object
	if err := controllerutil.SetControllerReference(cr, desired, r.Scheme); err != nil {
		return err
	}
	// Create if missing
	if !exists {
		argoutil.LogResourceCreation(log, desired)
		return r.Create(ctx, desired)
	}
	// Update owner reference if needed
	refChanged, err := modifyOwnerReferenceIfNeeded(cr, existing, r.Scheme)
	if err != nil {
		return err
	}
	if refChanged {
		argoutil.LogResourceUpdate(log, existing, "updating owner reference")
		return r.Update(ctx, existing)
	}
	// No changes required
	return nil
}

// reconcileArgoCmdParamsConfigMap will ensure that the ConfigMap containing command line parameters for ArgoCD is present.
func (r *ReconcileArgoCD) reconcileArgoCmdParamsConfigMap(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDCmdParamsConfigMapName, cr)
	cm.Data = make(map[string]string)

	// Set default for controller.resource.health.persist to "true"
	const healthPersistKey = "controller.resource.health.persist"
	cm.Data[healthPersistKey] = "true"

	// Copy user-specified command parameters if any
	if len(cr.Spec.CmdParams) > 0 {
		for k, v := range cr.Spec.CmdParams {
			cm.Data[k] = v
		}
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}

	existingCM := &corev1.ConfigMap{}
	isFound, err := argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, existingCM)
	if err != nil {
		return err
	}
	if isFound {

		changed := false

		// Compare only if data is being managed
		if len(cm.Data) > 0 && !reflect.DeepEqual(cm.Data, existingCM.Data) {
			existingCM.Data = cm.Data
			changed = true
		}
		// Check OwnerReferences
		var refChanged bool
		var err error
		if refChanged, err = modifyOwnerReferenceIfNeeded(cr, existingCM, r.Scheme); err != nil {
			return err
		}
		if refChanged {
			changed = true
		}
		if changed {
			explanation := "updating data"
			if refChanged {
				explanation += ", owner reference"
			}
			argoutil.LogResourceUpdate(log, existingCM, explanation)
			return r.Update(context.TODO(), existingCM)
		}
		return nil // Do nothing as there is no change in the configmap.
	}
	argoutil.LogResourceCreation(log, cm)
	return r.Create(context.TODO(), cm)
}

type filteredResource struct {
	APIGroups []string `yaml:"apiGroups,omitempty"`
	Kinds     []string `yaml:"kinds,omitempty"`
	Clusters  []string `yaml:"clusters,omitempty"`
}

func getDefaultResourceExclusions() []filteredResource {
	// See this URL for a description of why these resources are used:
	// - https://argo-cd.readthedocs.io/en/stable/operator-manual/upgrading/2.14-3.0/#default-resourceexclusions-configurations

	// See this URL for a current list of rules: https://github.com/argoproj/argo-cd/blob/master/manifests/base/config/argocd-cm.yaml

	return []filteredResource{
		{APIGroups: []string{"", "discovery.k8s.io"}, Kinds: []string{"Endpoints", "EndpointSlice"}},
		{APIGroups: []string{"apiregistration.k8s.io"}, Kinds: []string{"APIService"}},
		{APIGroups: []string{"coordination.k8s.io"}, Kinds: []string{"Lease"}},
		{APIGroups: []string{"authentication.k8s.io", "authorization.k8s.io"},
			Kinds: []string{
				"SelfSubjectReview", "TokenReview", "LocalSubjectAccessReview",
				"SelfSubjectAccessReview", "SelfSubjectRulesReview", "SubjectAccessReview"}},
		{APIGroups: []string{"certificates.k8s.io"}, Kinds: []string{"CertificateSigningRequest"}},
		{APIGroups: []string{"cert-manager.io"}, Kinds: []string{"CertificateRequest"}},
		{APIGroups: []string{"cilium.io"}, Kinds: []string{"CiliumIdentity", "CiliumEndpoint", "CiliumEndpointSlice"}},
		{APIGroups: []string{"kyverno.io", "reports.kyverno.io", "wgpolicyk8s.io"},
			Kinds: []string{
				"PolicyReport", "ClusterPolicyReport", "EphemeralReport", "ClusterEphemeralReport",
				"AdmissionReport", "ClusterAdmissionReport", "BackgroundScanReport",
				"ClusterBackgroundScanReport", "UpdateRequest"}},
	}
}
