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
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

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

// getResourceTrackingMethod will return the resource tracking method for the given ArgoCD.
func getResourceTrackingMethod(cr *argoproj.ArgoCD) string {
	rtm := argoproj.ParseResourceTrackingMethod(cr.Spec.ResourceTrackingMethod)
	if rtm == argoproj.ResourceTrackingMethodInvalid {
		log.Info(fmt.Sprintf("Found '%s' as resource tracking method, which is invalid. Using default 'label' method.", cr.Spec.ResourceTrackingMethod))
	} else if cr.Spec.ResourceTrackingMethod != "" {
		log.Info(fmt.Sprintf("Found '%s' as tracking method", cr.Spec.ResourceTrackingMethod))
	} else {
		log.Info("Using default resource tracking method 'label'")
	}
	return rtm.String()
}

// newConfigMap returns a new ConfigMap instance for the given ArgoCD.
func newConfigMap(cr *argoproj.ArgoCD) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newConfigMapWithName creates a new ConfigMap with the given name for the given ArgCD.
func newConfigMapWithName(name string, cr *argoproj.ArgoCD) *corev1.ConfigMap {
	cm := newConfigMap(cr)
	cm.ObjectMeta.Name = name

	lbls := cm.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	cm.ObjectMeta.Labels = lbls

	return cm
}

// newConfigMapWithName creates a new ConfigMap with the given suffix appended to the name.
// The name for the CongifMap is based on the name of the given ArgCD.
func newConfigMapWithSuffix(suffix string, cr *argoproj.ArgoCD) *corev1.ConfigMap {
	return newConfigMapWithName(fmt.Sprintf("%s-%s", cr.ObjectMeta.Name, suffix), cr)
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

	return r.reconcileGPGKeysConfigMap(cr)
}

// reconcileCAConfigMap will ensure that the Certificate Authority ConfigMap is present.
// This ConfigMap holds the CA Certificate data for client use.
func (r *ReconcileArgoCD) reconcileCAConfigMap(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(getCAConfigMapName(cr), cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	caSecret := argoutil.NewSecretWithSuffix(cr, common.ArgoCDCASuffix)
	if !argoutil.IsObjectFound(r.Client, cr.Namespace, caSecret.Name, caSecret) {
		log.Info(fmt.Sprintf("ca secret [%s] not found, waiting to reconcile ca configmap [%s]", caSecret.Name, cm.Name))
		return nil
	}

	cm.Data = map[string]string{
		common.ArgoCDKeyTLSCert: string(caSecret.Data[common.ArgoCDKeyTLSCert]),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileConfiguration will ensure that the main ConfigMap for ArgoCD is present.
func (r *ReconcileArgoCD) reconcileArgoConfigMap(cr *argoproj.ArgoCD) error {
	cm := newConfigMapWithName(common.ArgoCDConfigMapName, cr)

	cm.Data = make(map[string]string)

	cm.Data[common.ArgoCDKeyApplicationInstanceLabelKey] = getApplicationInstanceLabelKey(cr)
	cm.Data[common.ArgoCDKeyConfigManagementPlugins] = getConfigManagementPlugins(cr)
	cm.Data[common.ArgoCDKeyAdminEnabled] = fmt.Sprintf("%t", !cr.Spec.DisableAdmin)
	cm.Data[common.ArgoCDKeyGATrackingID] = getGATrackingID(cr)
	cm.Data[common.ArgoCDKeyGAAnonymizeUsers] = fmt.Sprint(cr.Spec.GAAnonymizeUsers)
	cm.Data[common.ArgoCDKeyHelpChatURL] = getHelpChatURL(cr)
	cm.Data[common.ArgoCDKeyHelpChatText] = getHelpChatText(cr)
	cm.Data[common.ArgoCDKeyKustomizeBuildOptions] = getKustomizeBuildOptions(cr)

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

	cm.Data[common.ArgoCDKeyResourceExclusions] = getResourceExclusions(cr)
	cm.Data[common.ArgoCDKeyResourceInclusions] = getResourceInclusions(cr)
	cm.Data[common.ArgoCDKeyResourceTrackingMethod] = getResourceTrackingMethod(cr)
	cm.Data[common.ArgoCDKeyRepositories] = getInitialRepositories(cr)
	cm.Data[common.ArgoCDKeyRepositoryCredentials] = getRepositoryCredentials(cr)
	cm.Data[common.ArgoCDKeyStatusBadgeEnabled] = fmt.Sprint(cr.Spec.StatusBadgeEnabled)
	cm.Data[common.ArgoCDKeyServerURL] = r.getArgoServerURI(cr)
	cm.Data[common.ArgoCDKeyUsersAnonymousEnabled] = fmt.Sprint(cr.Spec.UsersAnonymousEnabled)

	// create dex config if dex is enabled through `.spec.sso`
	if UseDex(cr) {
		dexConfig := getDexConfig(cr)

		// If no dexConfig expressed but openShiftOAuth is requested through `.spec.sso.dex`, use default
		// openshift dex config
		if dexConfig == "" && (cr.Spec.SSO != nil && cr.Spec.SSO.Dex != nil && cr.Spec.SSO.Dex.OpenShiftOAuth) {
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
		}
	}

	if len(cr.Spec.ExtraConfig) > 0 {
		for k, v := range cr.Spec.ExtraConfig {
			cm.Data[k] = v
		}
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}

	existingCM := &corev1.ConfigMap{}
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, existingCM) {

		// reconcile dex configuration if dex is enabled `.spec.sso.dex.provider` or there is
		// existing dex configuration
		if UseDex(cr) {
			if err := r.reconcileDexConfiguration(existingCM, cr); err != nil {
				return err
			}
		} else if cr.Spec.SSO != nil && cr.Spec.SSO.Provider.ToLower() == argoproj.SSOProviderTypeKeycloak {
			// retain oidc.config during reconcilliation when keycloak is configured
			cm.Data[common.ArgoCDKeyOIDCConfig] = existingCM.Data[common.ArgoCDKeyOIDCConfig]
		}

		if !reflect.DeepEqual(cm.Data, existingCM.Data) {
			existingCM.Data = cm.Data
			return r.Client.Update(context.TODO(), existingCM)
		}
		return nil // Do nothing as there is no change in the configmap.
	}
	return r.Client.Create(context.TODO(), cm)

}

// reconcileGrafanaConfiguration will ensure that the Grafana configuration ConfigMap is present.
func (r *ReconcileArgoCD) reconcileGrafanaConfiguration(cr *argoproj.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := newConfigMapWithSuffix(common.ArgoCDGrafanaConfigMapSuffix, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	secret := argoutil.NewSecretWithSuffix(cr, "grafana")
	secret, err := argoutil.FetchSecret(r.Client, cr.ObjectMeta, secret.Name)
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

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileGrafanaDashboards will ensure that the Grafana dashboards ConfigMap is present.
func (r *ReconcileArgoCD) reconcileGrafanaDashboards(cr *argoproj.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := newConfigMapWithSuffix(common.ArgoCDGrafanaDashboardConfigMapSuffix, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	pattern := filepath.Join(getGrafanaConfigPath(), "dashboards/*.json")
	dashboards, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	data := make(map[string]string)
	for _, f := range dashboards {
		dashboard, err := os.ReadFile(f)
		if err != nil {
			return err
		}

		parts := strings.Split(f, "/")
		filename := parts[len(parts)-1]
		data[filename] = string(dashboard)
	}
	cm.Data = data

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
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
	cm := newConfigMapWithName(common.ArgoCDRedisHAHealthConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if !cr.Spec.HA.Enabled {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			return r.Client.Delete(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	cm.Data = map[string]string{
		"redis_liveness.sh":    getRedisLivenessScript(useTLSForRedis),
		"redis_readiness.sh":   getRedisReadinessScript(useTLSForRedis),
		"sentinel_liveness.sh": getSentinelLivenessScript(useTLSForRedis),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRedisHAConfigMap will ensure that the Redis HA ConfigMap is present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisHAConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if !cr.Spec.HA.Enabled {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			return r.Client.Delete(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	cm.Data = map[string]string{
		"haproxy.cfg":     getRedisHAProxyConfig(cr, useTLSForRedis),
		"haproxy_init.sh": getRedisHAProxyScript(cr),
		"init.sh":         getRedisInitScript(cr, useTLSForRedis),
		"redis.conf":      getRedisConf(useTLSForRedis),
		"sentinel.conf":   getRedisSentinelConf(useTLSForRedis),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

func (r *ReconcileArgoCD) recreateRedisHAConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.Client.Delete(context.TODO(), cm); err != nil {
			return err
		}
	}
	return r.reconcileRedisHAConfigMap(cr, useTLSForRedis)
}

func (r *ReconcileArgoCD) recreateRedisHAHealthConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAHealthConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.Client.Delete(context.TODO(), cm); err != nil {
			return err
		}
	}
	return r.reconcileRedisHAHealthConfigMap(cr, useTLSForRedis)
}
