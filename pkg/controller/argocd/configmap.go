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
	"reflect"
	"strings"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	defaultKnownHosts = `bitbucket.org ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAubiN81eDcafrgMeLzaFPsw2kNvEcqTKl/VqLat/MaB33pZy0y3rJZtnqwR2qOOvbwKZYKiEO1O6VqNEBxKvJJelCq0dTXWT5pbO2gDXC6h6QDXCaHo6pOHGPUy+YBaGQRGuSusMEASYiWunYN0vCAI8QaXnWMXNMdFP3jHAJH0eDsoiGnLPBlBp4TNm6rYI74nMzgz3B9IikW4WVK+dc8KZJZWYjAuORU3jc1c/NPskD2ASinf8v3xnfXeukU0sJ5N6m5E8VLjObPEO+mN2t/FZTMZLiFqPWc/ALSqnMnnhwrNi2rbfg/rd/IpL8Le3pSBne8+seeFVBoGqzHM9yXw==
github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf
gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
ssh.dev.azure.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H
vs-ssh.visualstudio.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7Hr1oTWqNqOlzGJOfGJ4NakVyIzf1rXYd4d7wo6jBlkLvCA4odBlL0mDUyZ0/QUfTTqeu+tm22gOsv+VrVTMk6vwRU75gY/y9ut5Mb3bR5BV58dKXyq9A9UeB5Cakehn5Zgm6x1mKoVyf+FFn26iYqXJRgzIZZcZ5V6hrE0Qg39kZm4az48o0AUbf6Sp4SLdvnuMa2sVNwHBboS7EJkm57XQPVU3/QpyNLHbWDdzwtrlS+ez30S3AdYhLKEOxAG8weOnyrtLJAUen9mTkol8oII1edf7mWWbWVf0nBmly21+nZcmCTISQBtdcyPaEno7fFQMDD26/s0lfKob4Kw8H	
`
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
	if len(cr.Spec.ResourceCustomizations) > 0 {
		rc = cr.Spec.ResourceCustomizations
	}
	return rc
}

// getResourceExclusions will return the resource exclusions for the given ArgoCD.
func getResourceExclusions(cr *argoprojv1a1.ArgoCD) string {
	re := common.ArgoCDDefaultResourceExclusions
	if len(cr.Spec.ResourceExclusions) > 0 {
		re = cr.Spec.ResourceExclusions
	}
	return re
}

// getRepositories will return the repositories for the given ArgoCD.
func getRepositories(cr *argoprojv1a1.ArgoCD) string {
	repos := common.ArgoCDDefaultRepositories
	if len(cr.Spec.Repositories) > 0 {
		repos = cr.Spec.Repositories
	}
	return repos
}

// getSSHKnownHosts will return the SSH Known Hosts data for the given ArgoCD.
func getSSHKnownHosts(cr *argoprojv1a1.ArgoCD) string {
	skh := common.ArgoCDDefaultSSHKnownHosts
	if len(cr.Spec.SSHKnownHosts) > 0 {
		skh = cr.Spec.SSHKnownHosts
	}
	return skh
}

// getTLSCerts will return the TLS certs for the given ArgoCD.
func getTLSCerts(cr *argoprojv1a1.ArgoCD) map[string]string {
	certs := make(map[string]string)
	if len(cr.Spec.TLS.Certs) > 0 {
		certs = cr.Spec.TLS.Certs
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

	return nil
}

// reconcileCAConfigMap will ensure that the Certificate Authority ConfigMap is present.
// This ConfigMap holds the CA Certificate data for client use.
func (r *ReconcileArgoCD) reconcileCAConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(getCAConfigMapName(cr), cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	caSecret := newSecretWithSuffix(common.ArgoCDCASuffix, cr)
	caSecret, err := r.getSecret(caSecret.Name, cr)
	if err != nil {
		return err
	}

	cm.Data = map[string]string{
		TLSCACertKey: string(caSecret.Data[TLSCertKey]),
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
		uri := r.getArgoServerURI(cr)
		if cm.Data[common.ArgoCDKeyServerURL] != uri {
			cm.Data[common.ArgoCDKeyServerURL] = uri
			return r.client.Update(context.TODO(), cm)
		}

		if err := r.reconcileDexConfiguration(cm, cr); err != nil {
			return err
		}
		return nil // ConfigMap found and configured, do nothing further...
	}

	if len(cm.Data) <= 0 {
		cm.Data = make(map[string]string)
	}

	cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] = getApplicationInstanceLabelKey(cr)
	cm.Data[common.ArgoCDKeyConfigManagementPlugins] = getConfigManagementPlugins(cr)
	cm.Data[common.ArgoCDKeyDexConfig] = getDexConfig(cr)
	cm.Data[common.ArgoCDKeyGATrackingID] = getGATrackingID(cr)
	cm.Data[common.ArgoCDKeyGAAnonymizeUsers] = fmt.Sprint(cr.Spec.GAAnonymizeUsers)
	cm.Data[common.ArgoCDKeyHelpChatURL] = getHelpChatURL(cr)
	cm.Data[common.ArgoCDKeyHelpChatText] = getHelpChatText(cr)
	cm.Data[common.ArgoCDKeyKustomizeBuildOptions] = getKustomizeBuildOptions(cr)
	cm.Data[common.ArgoCDKeyOIDCConfig] = getOIDCConfig(cr)
	cm.Data[common.ArgoCDKeyResourceCustomizations] = getResourceCustomizations(cr)
	cm.Data[common.ArgoCDKeyResourceExclusions] = getResourceExclusions(cr)
	cm.Data[common.ArgoCDKeyRepositories] = getRepositories(cr)
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
		cm.Data[common.ArgoCDKeyDexConfig] = desired
		return r.client.Update(context.TODO(), cm)
	}
	return nil
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

	secret := newSecretWithSuffix("grafana", cr)
	secret, err := r.getSecret(secret.Name, cr)
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
	if err := r.reconcileRedisProbesConfigMap(cr); err != nil {
		return err
	}
	return nil
}

// reconcileRedisHAConfigMap will ensure that the Redis HA ConfigMap is present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisHAConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found with nothing changed, move along...
	}

	cm.Data = map[string]string{
		"init.sh":       getRedisInitScript(cr),
		"redis.conf":    getRedisConf(cr),
		"sentinel.conf": getRedisSentinelConf(cr),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileRedisProbesConfigMap will ensure that the Redis Probes ConfigMap is present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisProbesConfigMap(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDRedisProbesConfigMapName, cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found with nothing changed, move along...
	}

	cm.Data = map[string]string{
		"check-quorum.sh": getRedisQuorumScript(cr),
		"readiness.sh":    getRedisReadinessScript(cr),
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
		skh := getSSHKnownHosts(cr)
		if cm.Data[common.ArgoCDKeySSHKnownHosts] != skh {
			cm.Data[common.ArgoCDKeySSHKnownHosts] = skh
			return r.client.Update(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	cm.Data = map[string]string{
		common.ArgoCDKeySSHKnownHosts: getSSHKnownHosts(cr),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}

// reconcileTLSCerts will ensure that the ArgoCD TLS Certs ConfigMap is present.
func (r *ReconcileArgoCD) reconcileTLSCerts(cr *argoprojv1a1.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDTLSCertsConfigMapName, cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cm.Name, cm) {
		if !reflect.DeepEqual(cm.Data, cr.Spec.TLS.Certs) {
			cm.Data = cr.Spec.TLS.Certs
			return r.client.Update(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	cm.Data = getTLSCerts(cr)

	if err := controllerutil.SetControllerReference(cr, cm, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cm)
}
