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
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// createRBACConfigMap will create the Argo CD RBAC ConfigMap resource.
func (r *ReconcileArgoCD) createRBACConfigMap(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	data := make(map[string]string)
	data[common.ArgoCDKeyRBACPolicyCSV] = getRBACPolicy(cr)
	data[common.ArgoCDKeyRBACPolicyDefault] = getRBACDefaultPolicy(cr)
	data[common.ArgoCDKeyRBACScopes] = getRBACScopes(cr)
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// getApplicationInstanceLabelKey will return the application instance label key  for the given ArgoCD.
func getApplicationInstanceLabelKey(cr *argoprojv1a1.ArgoCD) string {
	key := common.ArgoCDDefaultApplicationInstanceLabelKey
	if len(cr.Spec.ApplicationInstanceLabelKey) > 0 {
		key = cr.Spec.ApplicationInstanceLabelKey
	}
	return key
}

// getCAConfigMapName will return the CA ConfigMap name for the given ArgoCD.
func getCAConfigMapName(cr *argoprojv1a1.ArgoCD) string {
	if len(cr.Spec.TLS.CA.ConfigMapName) > 0 {
		return cr.Spec.TLS.CA.ConfigMapName
	}
	return nameWithSuffix(common.ArgoCDCASuffix, cr)
}

// getConfigManagementPlugins will return the config management plugins for the given ArgoCD.
func getConfigManagementPlugins(cr *argoprojv1a1.ArgoCD) string {
	plugins := common.ArgoCDDefaultConfigManagementPlugins
	if len(cr.Spec.ConfigManagementPlugins) > 0 {
		plugins = cr.Spec.ConfigManagementPlugins
	}
	return plugins
}

func getDexConfig(cr *argoprojv1a1.ArgoCD) string {
	config := common.ArgoCDDefaultDexConfig
	if len(cr.Spec.Dex.Config) > 0 {
		config = cr.Spec.Dex.Config
	}
	return config
}

// getGATrackingID will return the google analytics tracking ID for the given Argo CD.
func getGATrackingID(cr *argoprojv1a1.ArgoCD) string {
	id := common.ArgoCDDefaultGATrackingID
	if len(cr.Spec.GATrackingID) > 0 {
		id = cr.Spec.GATrackingID
	}
	return id
}

// getHelpChatURL will return the help chat URL for the given Argo CD.
func getHelpChatURL(cr *argoprojv1a1.ArgoCD) string {
	url := common.ArgoCDDefaultHelpChatURL
	if len(cr.Spec.HelpChatURL) > 0 {
		url = cr.Spec.HelpChatURL
	}
	return url
}

// getHelpChatText will return the help chat text for the given Argo CD.
func getHelpChatText(cr *argoprojv1a1.ArgoCD) string {
	text := common.ArgoCDDefaultHelpChatText
	if len(cr.Spec.HelpChatText) > 0 {
		text = cr.Spec.HelpChatText
	}
	return text
}

// getKustomizeBuildOptions will return the kuztomize build options for the given ArgoCD.
func getKustomizeBuildOptions(cr *argoprojv1a1.ArgoCD) string {
	kbo := common.ArgoCDDefaultKustomizeBuildOptions
	if len(cr.Spec.KustomizeBuildOptions) > 0 {
		kbo = cr.Spec.KustomizeBuildOptions
	}
	return kbo
}

// getOIDCConfig will return the OIDC configuration for the given ArgoCD.
func getOIDCConfig(cr *argoprojv1a1.ArgoCD) string {
	config := common.ArgoCDDefaultOIDCConfig
	if len(cr.Spec.OIDCConfig) > 0 {
		config = cr.Spec.OIDCConfig
	}
	return config
}

// getRBACPolicy will return the RBAC policy for the given ArgoCD.
func getRBACPolicy(cr *argoprojv1a1.ArgoCD) string {
	policy := common.ArgoCDDefaultRBACPolicy
	if cr.Spec.RBAC.Policy != nil {
		policy = *cr.Spec.RBAC.Policy
	}
	return policy
}

// getRBACDefaultPolicy will retun the RBAC default policy for the given ArgoCD.
func getRBACDefaultPolicy(cr *argoprojv1a1.ArgoCD) string {
	dp := common.ArgoCDDefaultRBACDefaultPolicy
	if cr.Spec.RBAC.DefaultPolicy != nil {
		dp = *cr.Spec.RBAC.DefaultPolicy
	}
	return dp
}

// getRBACScopes will return the RBAC scopes for the given ArgoCD.
func getRBACScopes(cr *argoprojv1a1.ArgoCD) string {
	scopes := common.ArgoCDDefaultRBACScopes
	if cr.Spec.RBAC.Scopes != nil {
		scopes = *cr.Spec.RBAC.Scopes
	}
	return scopes
}

// getResourceCustomizations will return the resource customizations for the given ArgoCD.
func getResourceCustomizations(cr *argoprojv1a1.ArgoCD) string {
	rc := common.ArgoCDDefaultResourceCustomizations
	if cr.Spec.ResourceCustomizations != "" {
		rc = cr.Spec.ResourceCustomizations
	}
	return rc
}

// getResourceExclusions will return the resource exclusions for the given ArgoCD.
func getResourceExclusions(cr *argoprojv1a1.ArgoCD) string {
	re := common.ArgoCDDefaultResourceExclusions
	if cr.Spec.ResourceExclusions != "" {
		re = cr.Spec.ResourceExclusions
	}
	return re
}

// getResourceInclusions will return the resource inclusions for the given ArgoCD.
func getResourceInclusions(cr *argoprojv1a1.ArgoCD) string {
	re := common.ArgoCDDefaultResourceInclusions
	if cr.Spec.ResourceInclusions != "" {
		re = cr.Spec.ResourceInclusions
	}
	return re
}

// getInitialRepositories will return the initial repositories for the given ArgoCD.
func getInitialRepositories(cr *argoprojv1a1.ArgoCD) string {
	repos := common.ArgoCDDefaultRepositories
	if len(cr.Spec.InitialRepositories) > 0 {
		repos = cr.Spec.InitialRepositories
	}
	return repos
}

// getRepositoryCredentials will return the repository credentials for the given ArgoCD.
func getRepositoryCredentials(cr *argoprojv1a1.ArgoCD) string {
	repos := common.ArgoCDDefaultRepositoryCredentials
	if len(cr.Spec.RepositoryCredentials) > 0 {
		repos = cr.Spec.RepositoryCredentials
	}
	return repos
}

// getSSHKnownHosts will return the SSH Known Hosts data for the given ArgoCD.
func getInitialSSHKnownHosts(cr *argoprojv1a1.ArgoCD) string {
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
func getInitialTLSCerts(cr *argoprojv1a1.ArgoCD) map[string]string {
	certs := make(map[string]string)
	if len(cr.Spec.TLS.InitialCerts) > 0 {
		certs = cr.Spec.TLS.InitialCerts
	}
	return certs
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

// newConfigMapWithName creates a new ConfigMap with the given name for the given ArgCD.
func newConfigMapWithName(name string, cr *argoprojv1a1.ArgoCD) *corev1.ConfigMap {
	cm := newConfigMap(cr)
	cm.ObjectMeta.Name = name

	lbls := cm.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	cm.ObjectMeta.Labels = lbls

	return cm
}

// newConfigMapWithName creates a new ConfigMap with the given suffix appended to the name.
// The name for the CongifMap is based on the name of the given ArgCD.
func newConfigMapWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) *corev1.ConfigMap {
	return newConfigMapWithName(fmt.Sprintf("%s-%s", cr.ObjectMeta.Name, suffix), cr)
}

// reconcileConfigMaps will ensure that all ArgoCD ConfigMaps are present.
func (r *ReconcileArgoCD) reconcileConfigMaps(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileArgoConfigMap(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisConfiguration(cr); err != nil {
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

	return r.reconcileGPGKeysConfigMap(cr)
}

// reconcileCAConfigMap will ensure that the Certificate Authority ConfigMap is present.
// This ConfigMap holds the CA Certificate data for client use.
func (r *ReconcileArgoCD) reconcileCAConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(getCAConfigMapName(cr), cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	caSecret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, common.ArgoCDCASuffix)
	if !argoutil.IsObjectFound(r.client, cr.Namespace, caSecret.Name, caSecret) {
		log.Info(fmt.Sprintf("ca secret [%s] not found, waiting to reconcile ca configmap [%s]", caSecret.Name, cm.Name))
		return nil
	}

	cm.Data = map[string]string{
		common.ArgoCDKeyTLSCert: string(caSecret.Data[common.ArgoCDKeyTLSCert]),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileConfiguration will ensure that the main ConfigMap for ArgoCD is present.
func (r *ReconcileArgoCD) reconcileArgoConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDConfigMapName, cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		if err := r.reconcileDexConfiguration(cm, cr); err != nil {
			return err
		}
		return r.reconcileExistingArgoConfigMap(cm, cr)
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] = getApplicationInstanceLabelKey(cr)
	cm.Data[common.ArgoCDKeyConfigManagementPlugins] = getConfigManagementPlugins(cr)
	cm.Data[common.ArgoCDKeyAdminEnabled] = fmt.Sprintf("%t", !cr.Spec.DisableAdmin)
	cm.Data[common.ArgoCDKeyDexConfig] = getDexConfig(cr)
	cm.Data[common.ArgoCDKeyGATrackingID] = getGATrackingID(cr)
	cm.Data[common.ArgoCDKeyGAAnonymizeUsers] = fmt.Sprint(cr.Spec.GAAnonymizeUsers)
	cm.Data[common.ArgoCDKeyHelpChatURL] = getHelpChatURL(cr)
	cm.Data[common.ArgoCDKeyHelpChatText] = getHelpChatText(cr)
	cm.Data[common.ArgoCDKeyKustomizeBuildOptions] = getKustomizeBuildOptions(cr)
	cm.Data[common.ArgoCDKeyOIDCConfig] = getOIDCConfig(cr)
	if c := getResourceCustomizations(cr); c != "" {
		cm.Data[common.ArgoCDKeyResourceCustomizations] = c
	}
	cm.Data[common.ArgoCDKeyResourceExclusions] = getResourceExclusions(cr)
	cm.Data[common.ArgoCDKeyResourceInclusions] = getResourceInclusions(cr)
	cm.Data[common.ArgoCDKeyRepositories] = getInitialRepositories(cr)
	cm.Data[common.ArgoCDKeyRepositoryCredentials] = getRepositoryCredentials(cr)
	cm.Data[common.ArgoCDKeyStatusBadgeEnabled] = fmt.Sprint(cr.Spec.StatusBadgeEnabled)
	cm.Data[common.ArgoCDKeyServerURL] = r.getArgoServerURI(cr)
	cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] = fmt.Sprint(cr.Spec.UsersAnonymousEnabled)

	dexConfig := getDexConfig(cr)
	if len(dexConfig) <= 0 && cr.Spec.Dex.OpenShiftOAuth {
		cfg, err := r.getOpenShiftDexConfig(cr)
		if err != nil {
			return err
		}
		dexConfig = cfg
	}
	cm.Data[common.ArgoCDKeyDexConfig] = dexConfig

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileDexConfiguration will ensure that Dex is configured properly.
func (r *ReconcileArgoCD) reconcileDexConfiguration(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	actual := cm.Data[common.ArgoCDKeyDexConfig]
	desired := getDexConfig(cr)
	if len(desired) <= 0 && cr.Spec.Dex.OpenShiftOAuth {
		cfg, err := r.getOpenShiftDexConfig(cr)
		if err != nil {
			return err
		}
		desired = cfg
	}

	if actual != desired {
		// Update ConfigMap with desired configuration.
		cm.Data[common.ArgoCDKeyDexConfig] = desired
		if err := r.client.Update(context.TODO(), cm); err != nil {
			return err
		}

		// Trigger rollout of Dex Deployment to pick up changes.
		deploy := newDeploymentWithSuffix("dex-server", "dex-server", cr)
		if !argoutil.IsObjectFound(r.client, deploy.Namespace, deploy.Name, deploy) {
			log.Info("unable to locate dex deployment")
			return nil
		}

		deploy.Spec.Template.ObjectMeta.Labels["dex.config.changed"] = time.Now().UTC().Format("01022006-150406-MST")
		return r.client.Update(context.TODO(), deploy)
	}
	return nil
}

func (r *ReconcileArgoCD) reconcileExistingArgoConfigMap(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
	changed := false

	if cm.Data[common.ArgoCDKeyAdminEnabled] == fmt.Sprintf("%t", cr.Spec.DisableAdmin) {
		cm.Data[common.ArgoCDKeyAdminEnabled] = fmt.Sprintf("%t", !cr.Spec.DisableAdmin)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] != cr.Spec.ApplicationInstanceLabelKey {
		cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] = cr.Spec.ApplicationInstanceLabelKey
		changed = true
	}

	if cm.Data[common.ArgoCDKeyConfigManagementPlugins] != cr.Spec.ConfigManagementPlugins {
		cm.Data[common.ArgoCDKeyConfigManagementPlugins] = cr.Spec.ConfigManagementPlugins
		changed = true
	}

	if cm.Data[common.ArgoCDKeyGATrackingID] != cr.Spec.GATrackingID {
		cm.Data[common.ArgoCDKeyGATrackingID] = cr.Spec.GATrackingID
		changed = true
	}

	if cm.Data[common.ArgoCDKeyGAAnonymizeUsers] != fmt.Sprint(cr.Spec.GAAnonymizeUsers) {
		cm.Data[common.ArgoCDKeyGAAnonymizeUsers] = fmt.Sprint(cr.Spec.GAAnonymizeUsers)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyHelpChatURL] != cr.Spec.HelpChatURL {
		cm.Data[common.ArgoCDKeyHelpChatURL] = cr.Spec.HelpChatURL
		changed = true
	}

	if cm.Data[common.ArgoCDKeyHelpChatText] != cr.Spec.HelpChatText {
		cm.Data[common.ArgoCDKeyHelpChatText] = cr.Spec.HelpChatText
		changed = true
	}

	if cm.Data[common.ArgoCDKeyKustomizeBuildOptions] != cr.Spec.KustomizeBuildOptions {
		cm.Data[common.ArgoCDKeyKustomizeBuildOptions] = cr.Spec.KustomizeBuildOptions
		changed = true
	}

	if cm.Data[common.ArgoCDKeyOIDCConfig] != cr.Spec.OIDCConfig {
		cm.Data[common.ArgoCDKeyOIDCConfig] = cr.Spec.OIDCConfig
		changed = true
	}

	if cm.Data[common.ArgoCDKeyResourceCustomizations] != cr.Spec.ResourceCustomizations {
		cm.Data[common.ArgoCDKeyResourceCustomizations] = cr.Spec.ResourceCustomizations
		changed = true
	}

	if cm.Data[common.ArgoCDKeyResourceExclusions] != cr.Spec.ResourceExclusions {
		cm.Data[common.ArgoCDKeyResourceExclusions] = cr.Spec.ResourceExclusions
		changed = true
	}

	uri := r.getArgoServerURI(cr)
	if cm.Data[common.ArgoCDKeyServerURL] != uri {
		cm.Data[common.ArgoCDKeyServerURL] = uri
		changed = true
	}

	if cm.Data[common.ArgoCDKeyStatusBadgeEnabled] != fmt.Sprint(cr.Spec.StatusBadgeEnabled) {
		cm.Data[common.ArgoCDKeyStatusBadgeEnabled] = fmt.Sprint(cr.Spec.StatusBadgeEnabled)
		changed = true
	}

	if cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] != fmt.Sprint(cr.Spec.UsersAnonymousEnabled) {
		cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] = fmt.Sprint(cr.Spec.UsersAnonymousEnabled)
		changed = true
	}

	if changed {
		return r.client.Update(context.TODO(), cm) // TODO: Reload Argo CD server after ConfigMap change (which properties)?
	}

	return nil // Nothing changed, no update needed...
}

// reconcileGrafanaConfiguration will ensure that the Grafana configuration ConfigMap is present.
func (r *ReconcileArgoCD) reconcileGrafanaConfiguration(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := newConfigMapWithSuffix(common.ArgoCDGrafanaConfigMapSuffix, cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	secret := argoutil.NewSecretWithSuffix(cr.ObjectMeta, "grafana")
	secret, err := argoutil.FetchSecret(r.client, cr.ObjectMeta, secret.Name)
	if err != nil {
		return err
	}

	grafanaConfig := GrafanaConfig{
		Security: GrafanaSecurityConfig{
			AdminUser:     string(secret.Data[common.ArgoCDKeyGrafanaAdminUsername]),
			AdminPassword: string(secret.Data[common.ArgoCDKeyGrafanaAdminPassword]),
			SecretKey:     string(secret.Data[common.ArgoCDKeyGrafanaSecretKey]),
		},
	}

	data, err := loadGrafanaConfigs()
	if err != nil {
		return err
	}

	tmpls, err := loadGrafanaTemplates(&grafanaConfig)
	if err != nil {
		return err
	}

	for key, val := range tmpls {
		data[key] = val
	}
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileGrafanaDashboards will ensure that the Grafana dashboards ConfigMap is present.
func (r *ReconcileArgoCD) reconcileGrafanaDashboards(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := newConfigMapWithSuffix(common.ArgoCDGrafanaDashboardConfigMapSuffix, cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	pattern := filepath.Join(getGrafanaConfigPath(), "dashboards/*.json")
	dashboards, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	data := make(map[string]string)
	for _, f := range dashboards {
		dashboard, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}

		parts := strings.Split(f, "/")
		filename := parts[len(parts)-1]
		data[filename] = string(dashboard)
	}
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileRBAC will ensure that the ArgoCD RBAC ConfigMap is present.
func (r *ReconcileArgoCD) reconcileRBAC(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDRBACConfigMapName, cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		return r.reconcileRBACConfigMap(cm, cr)
	}
	return r.createRBACConfigMap(cm, cr)
}

// reconcileRBACConfigMap will ensure that the RBAC ConfigMap is syncronized with the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRBACConfigMap(cm *corev1.ConfigMap, cr *argoprojv1a1.ArgoCD) error {
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

	// Scopes
	if cr.Spec.RBAC.Scopes != nil && cm.Data[common.ArgoCDKeyRBACScopes] != *cr.Spec.RBAC.Scopes {
		cm.Data[common.ArgoCDKeyRBACScopes] = *cr.Spec.RBAC.Scopes
		changed = true
	}

	if changed {
		// TODO: Reload server (and dex?) if RBAC settings change?
		return r.client.Update(context.TODO(), cm)
	}
	return nil // ConfigMap exists and nothing to do, move along...
}

// reconcileRedisConfiguration will ensure that all of the Redis ConfigMaps are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisConfiguration(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRedisHAConfigMap(cr); err != nil {
		return err
	}
	return nil
}

// reconcileRedisHAConfigMap will ensure that the Redis HA ConfigMap is present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisHAConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		if !cr.Spec.HA.Enabled {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			return r.client.Delete(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	cm.Data = map[string]string{
		"haproxy.cfg":     getRedisHAProxyConfig(cr),
		"haproxy_init.sh": getRedisHAProxyScript(cr),
		"init.sh":         getRedisInitScript(cr),
		"redis.conf":      getRedisConf(cr),
		"sentinel.conf":   getRedisSentinelConf(cr),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileSSHKnownHosts will ensure that the ArgoCD SSH Known Hosts ConfigMap is present.
func (r *ReconcileArgoCD) reconcileSSHKnownHosts(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDKnownHostsConfigMapName, cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, move along...
	}

	cm.Data = map[string]string{
		common.ArgoCDKeySSHKnownHosts: getInitialSSHKnownHosts(cr),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileTLSCerts will ensure that the ArgoCD TLS Certs ConfigMap is present.
func (r *ReconcileArgoCD) reconcileTLSCerts(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDTLSCertsConfigMapName, cr)
	cm.Data = getInitialTLSCerts(cr)
	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}

	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		return r.client.Update(context.TODO(), cm)
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileGPGKeysConfigMap creates a gpg-keys config map
func (r *ReconcileArgoCD) reconcileGPGKeysConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDGPGKeysConfigMapName, cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		return nil
	}
	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}
