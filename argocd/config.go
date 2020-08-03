// Copyright 2019 Argo CD Operator Developers
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

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	"github.com/sethvargo/go-password/password"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// createRBACConfigMap will create the Argo CD RBAC ConfigMap resource.
func (r *ArgoClusterReconciler) createRBACConfigMap(cm *corev1.ConfigMap, cr *v1alpha1.ArgoCD) error {
	data := make(map[string]string)
	data[common.ArgoCDKeyRBACPolicyCSV] = getRBACPolicy(cr)
	data[common.ArgoCDKeyRBACPolicyDefault] = getRBACDefaultPolicy(cr)
	data[common.ArgoCDKeyRBACScopes] = getRBACScopes(cr)
	cm.Data = data

	ctrl.SetControllerReference(cr, cm, r.Scheme)
	return r.Client.Create(context.TODO(), cm)
}

// getGrafanaAdminPassword will generate and return the admin password for Argo CD.
func generateArgoAdminPassword() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultAdminPasswordLength,
		common.ArgoCDDefaultAdminPasswordNumDigits,
		common.ArgoCDDefaultAdminPasswordNumSymbols,
		false, false)

	return []byte(pass), err
}

// getApplicationInstanceLabelKey will return the application instance label key  for the given ArgoCD.
func getApplicationInstanceLabelKey(cr *v1alpha1.ArgoCD) string {
	key := common.ArgoCDDefaultApplicationInstanceLabelKey
	if len(cr.Spec.ApplicationInstanceLabelKey) > 0 {
		key = cr.Spec.ApplicationInstanceLabelKey
	}
	return key
}

// getCAConfigMapName will return the CA ConfigMap name for the given ArgoCD.
func getCAConfigMapName(cr *v1alpha1.ArgoCD) string {
	if len(cr.Spec.TLS.CA.ConfigMapName) > 0 {
		return cr.Spec.TLS.CA.ConfigMapName
	}
	return common.NameWithSuffix(cr.ObjectMeta, common.ArgoCDCASuffix)
}

// getConfigManagementPlugins will return the config management plugins for the given ArgoCD.
func getConfigManagementPlugins(cr *v1alpha1.ArgoCD) string {
	plugins := common.ArgoCDDefaultConfigManagementPlugins
	if len(cr.Spec.ConfigManagementPlugins) > 0 {
		plugins = cr.Spec.ConfigManagementPlugins
	}
	return plugins
}

func getDexConfig(cr *v1alpha1.ArgoCD) string {
	config := common.ArgoCDDefaultDexConfig
	if len(cr.Spec.Dex.Config) > 0 {
		config = cr.Spec.Dex.Config
	}
	return config
}

// getGATrackingID will return the google analytics tracking ID for the given Argo CD.
func getGATrackingID(cr *v1alpha1.ArgoCD) string {
	id := common.ArgoCDDefaultGATrackingID
	if len(cr.Spec.GATrackingID) > 0 {
		id = cr.Spec.GATrackingID
	}
	return id
}

// getHelpChatURL will return the help chat URL for the given Argo CD.
func getHelpChatURL(cr *v1alpha1.ArgoCD) string {
	url := common.ArgoCDDefaultHelpChatURL
	if len(cr.Spec.HelpChatURL) > 0 {
		url = cr.Spec.HelpChatURL
	}
	return url
}

// getHelpChatText will return the help chat text for the given Argo CD.
func getHelpChatText(cr *v1alpha1.ArgoCD) string {
	text := common.ArgoCDDefaultHelpChatText
	if len(cr.Spec.HelpChatText) > 0 {
		text = cr.Spec.HelpChatText
	}
	return text
}

// getInitialRepositories will return the initial repositories for the given ArgoCD.
func getInitialRepositories(cr *v1alpha1.ArgoCD) string {
	repos := common.ArgoCDDefaultRepositories
	if len(cr.Spec.InitialRepositories) > 0 {
		repos = cr.Spec.InitialRepositories
	}
	return repos
}

// getSSHKnownHosts will return the SSH Known Hosts data for the given ArgoCD.
func getInitialSSHKnownHosts(cr *v1alpha1.ArgoCD) string {
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
func getInitialTLSCerts(cr *v1alpha1.ArgoCD) map[string]string {
	certs := make(map[string]string)
	if len(cr.Spec.TLS.InitialCerts) > 0 {
		certs = cr.Spec.TLS.InitialCerts
	}
	return certs
}

// getKustomizeBuildOptions will return the kuztomize build options for the given ArgoCD.
func getKustomizeBuildOptions(cr *v1alpha1.ArgoCD) string {
	kbo := common.ArgoCDDefaultKustomizeBuildOptions
	if len(cr.Spec.KustomizeBuildOptions) > 0 {
		kbo = cr.Spec.KustomizeBuildOptions
	}
	return kbo
}

// getOIDCConfig will return the OIDC configuration for the given ArgoCD.
func getOIDCConfig(cr *v1alpha1.ArgoCD) string {
	config := common.ArgoCDDefaultOIDCConfig
	if len(cr.Spec.OIDCConfig) > 0 {
		config = cr.Spec.OIDCConfig
	}
	return config
}

// getRBACPolicy will return the RBAC policy for the given ArgoCD.
func getRBACPolicy(cr *v1alpha1.ArgoCD) string {
	policy := common.ArgoCDDefaultRBACPolicy
	if cr.Spec.RBAC.Policy != nil {
		policy = *cr.Spec.RBAC.Policy
	}
	return policy
}

// getRBACDefaultPolicy will retun the RBAC default policy for the given ArgoCD.
func getRBACDefaultPolicy(cr *v1alpha1.ArgoCD) string {
	dp := common.ArgoCDDefaultRBACDefaultPolicy
	if cr.Spec.RBAC.DefaultPolicy != nil {
		dp = *cr.Spec.RBAC.DefaultPolicy
	}
	return dp
}

// getRBACScopes will return the RBAC scopes for the given ArgoCD.
func getRBACScopes(cr *v1alpha1.ArgoCD) string {
	scopes := common.ArgoCDDefaultRBACScopes
	if cr.Spec.RBAC.Scopes != nil {
		scopes = *cr.Spec.RBAC.Scopes
	}
	return scopes
}

// getRepositoryCredentials will return the repository credentials for the given ArgoCD.
func getRepositoryCredentials(cr *v1alpha1.ArgoCD) string {
	repos := common.ArgoCDDefaultRepositoryCredentials
	if len(cr.Spec.RepositoryCredentials) > 0 {
		repos = cr.Spec.RepositoryCredentials
	}
	return repos
}

// getResourceCustomizations will return the resource customizations for the given ArgoCD.
func getResourceCustomizations(cr *v1alpha1.ArgoCD) string {
	rc := common.ArgoCDDefaultResourceCustomizations
	if len(cr.Spec.ResourceCustomizations) > 0 {
		rc = cr.Spec.ResourceCustomizations
	}
	return rc
}

// getResourceExclusions will return the resource exclusions for the given ArgoCD.
func getResourceExclusions(cr *v1alpha1.ArgoCD) string {
	re := common.ArgoCDDefaultResourceExclusions
	if len(cr.Spec.ResourceExclusions) > 0 {
		re = cr.Spec.ResourceExclusions
	}
	return re
}

// hasArgoAdminPasswordChanged will return true if the Argo admin password has changed.
func hasArgoAdminPasswordChanged(actual *corev1.Secret, expected *corev1.Secret) bool {
	actualPwd := string(actual.Data[common.ArgoCDKeyAdminPassword])
	expectedPwd := string(expected.Data[common.ArgoCDKeyAdminPassword])

	validPwd, _ := common.VerifyPassword(expectedPwd, actualPwd)
	if !validPwd {
		log.Info("admin password has changed")
		return true
	}
	return false
}

// reconcileConfiguration will ensure that the main ConfigMap for ArgoCD is present.
func (r *ArgoClusterReconciler) reconcileArgoConfigMap(cr *v1alpha1.ArgoCD) error {
	cm := resources.NewConfigMapWithName(cr.ObjectMeta, common.ArgoCDConfigMapName)
	if resources.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.reconcileDexConfiguration(cm, cr); err != nil {
			return err
		}
		return r.reconcileExistingArgoConfigMap(cm, cr)
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

	ctrl.SetControllerReference(cr, cm, r.Scheme)
	return r.Client.Create(context.TODO(), cm)
}

// reconcileExistingArgoConfigMap will ensure that the main ConfigMap for ArgoCD is up to date.
func (r *ArgoClusterReconciler) reconcileExistingArgoConfigMap(cm *corev1.ConfigMap, cr *v1alpha1.ArgoCD) error {
	changed := false

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
		return r.Client.Update(context.TODO(), cm) // TODO: Reload Argo CD server after ConfigMap change (which properties)?
	}

	return nil // Nothing changed, no update needed...
}

// reconcileRBAC will ensure that the ArgoCD RBAC ConfigMap is present.
func (r *ArgoClusterReconciler) reconcileRBAC(cr *v1alpha1.ArgoCD) error {
	cm := resources.NewConfigMapWithName(cr.ObjectMeta, common.ArgoCDRBACConfigMapName)
	if resources.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return r.reconcileRBACConfigMap(cm, cr)
	}
	return r.createRBACConfigMap(cm, cr)
}

// reconcileRBACConfigMap will ensure that the RBAC ConfigMap is syncronized with the given ArgoCD.
func (r *ArgoClusterReconciler) reconcileRBACConfigMap(cm *corev1.ConfigMap, cr *v1alpha1.ArgoCD) error {
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
		return r.Client.Update(context.TODO(), cm)
	}
	return nil // ConfigMap exists and nothing to do, move along...
}

// reconcileSSHKnownHosts will ensure that the ArgoCD SSH Known Hosts ConfigMap is present.
func (r *ArgoClusterReconciler) reconcileSSHKnownHosts(cr *v1alpha1.ArgoCD) error {
	cm := resources.NewConfigMapWithName(cr.ObjectMeta, common.ArgoCDKnownHostsConfigMapName)
	if resources.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, move along...
	}

	cm.Data = map[string]string{
		common.ArgoCDKeySSHKnownHosts: getInitialSSHKnownHosts(cr),
	}

	ctrl.SetControllerReference(cr, cm, r.Scheme)
	return r.Client.Create(context.TODO(), cm)
}

// reconcileExistingArgoSecret will ensure that the Argo CD Secret is up to date.
func (r *ArgoClusterReconciler) reconcileExistingArgoSecret(cr *v1alpha1.ArgoCD, secret *corev1.Secret, clusterSecret *corev1.Secret, tlsSecret *corev1.Secret) error {
	changed := false

	if hasArgoAdminPasswordChanged(secret, clusterSecret) {
		hashedPassword, err := common.HashPassword(string(clusterSecret.Data[common.ArgoCDKeyAdminPassword]))
		if err != nil {
			return err
		}

		secret.Data[common.ArgoCDKeyAdminPassword] = []byte(hashedPassword)
		secret.Data[common.ArgoCDKeyAdminPasswordMTime] = common.NowBytes()
		changed = true
	}

	if hasArgoTLSChanged(secret, tlsSecret) {
		secret.Data[common.ArgoCDKeyTLSCert] = tlsSecret.Data[common.ArgoCDKeyTLSCert]
		secret.Data[common.ArgoCDKeyTLSPrivateKey] = tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey]
		changed = true
	}

	if changed {
		log.Info("updating argo secret")
		if err := r.Client.Update(context.TODO(), secret); err != nil {
			return err
		}

		// Trigger rollout of Argo Server Deployment
		deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "server", "server")
		return r.triggerRollout(deploy, "secret.changed")
	}

	return nil
}

// reconcileClusterCASecret ensures the CA Secret is created for the ArgoCD cluster.
func (r *ArgoClusterReconciler) reconcileClusterCASecret(cr *v1alpha1.ArgoCD) error {
	secret := resources.NewSecretWithSuffix(cr.ObjectMeta, "ca")
	if resources.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	secret, err := newCASecret(cr.ObjectMeta)
	if err != nil {
		return err
	}

	ctrl.SetControllerReference(cr, secret, r.Scheme)
	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterTLSSecret ensures the TLS Secret is created for the ArgoCD cluster.
func (r *ArgoClusterReconciler) reconcileClusterTLSSecret(cr *v1alpha1.ArgoCD) error {
	secret := resources.NewTLSSecret(cr.ObjectMeta, "tls")
	if resources.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	caSecret := resources.NewSecretWithSuffix(cr.ObjectMeta, "ca")
	caSecret, err := resources.FetchSecret(r.Client, cr.ObjectMeta, caSecret.Name)
	if err != nil {
		return err
	}

	caCert, err := common.ParsePEMEncodedCert(caSecret.Data[corev1.TLSCertKey])
	if err != nil {
		return err
	}

	caKey, err := common.ParsePEMEncodedPrivateKey(caSecret.Data[corev1.TLSPrivateKeyKey])
	if err != nil {
		return err
	}

	secret, err = newCertificateSecret("tls", caCert, caKey, cr)
	if err != nil {
		return err
	}

	ctrl.SetControllerReference(cr, secret, r.Scheme)
	return r.Client.Create(context.TODO(), secret)
}

// reconcileArgoSecret will ensure that the Argo CD Secret is present.
func (r *ArgoClusterReconciler) reconcileArgoSecret(cr *v1alpha1.ArgoCD) error {
	clusterSecret := resources.NewSecretWithSuffix(cr.ObjectMeta, "cluster")
	secret := resources.NewSecretWithName(cr.ObjectMeta, common.ArgoCDSecretName)

	if !resources.IsObjectFound(r.Client, cr.Namespace, clusterSecret.Name, clusterSecret) {
		log.Info(fmt.Sprintf("cluster secret [%s] not found, waiting to reconcile argo secret [%s]", clusterSecret.Name, secret.Name))
		return nil
	}

	tlsSecret := resources.NewSecretWithSuffix(cr.ObjectMeta, "tls")
	if !resources.IsObjectFound(r.Client, cr.Namespace, tlsSecret.Name, tlsSecret) {
		log.Info(fmt.Sprintf("tls secret [%s] not found, waiting to reconcile argo secret [%s]", tlsSecret.Name, secret.Name))
		return nil
	}

	if resources.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return r.reconcileExistingArgoSecret(cr, secret, clusterSecret, tlsSecret)
	}

	// Secret not found, create it...
	hashedPassword, err := common.HashPassword(string(clusterSecret.Data[common.ArgoCDKeyAdminPassword]))
	if err != nil {
		return err
	}

	sessionKey, err := generateArgoServerSessionKey()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword:      []byte(hashedPassword),
		common.ArgoCDKeyAdminPasswordMTime: common.NowBytes(),
		common.ArgoCDKeyServerSecretKey:    sessionKey,
		common.ArgoCDKeyTLSCert:            tlsSecret.Data[common.ArgoCDKeyTLSCert],
		common.ArgoCDKeyTLSPrivateKey:      tlsSecret.Data[common.ArgoCDKeyTLSPrivateKey],
	}

	ctrl.SetControllerReference(cr, secret, r.Scheme)
	return r.Client.Create(context.TODO(), secret)
}

// reconcileClusterMainSecret will ensure that the main Secret is present for the Argo CD cluster.
func (r *ArgoClusterReconciler) reconcileClusterMainSecret(cr *v1alpha1.ArgoCD) error {
	secret := resources.NewSecretWithSuffix(cr.ObjectMeta, "cluster")
	if resources.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // Secret found, do nothing
	}

	adminPassword, err := generateArgoAdminPassword()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyAdminPassword: adminPassword,
	}

	ctrl.SetControllerReference(cr, secret, r.Scheme)
	return r.Client.Create(context.TODO(), secret)
}
