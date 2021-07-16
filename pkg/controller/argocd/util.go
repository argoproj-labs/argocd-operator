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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/template"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	"gopkg.in/yaml.v2"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// DexConnector represents an authentication connector for Dex.
type DexConnector struct {
	Config map[string]interface{} `yaml:"config,omitempty"`
	ID     string                 `yaml:"id"`
	Name   string                 `yaml:"name"`
	Type   string                 `yaml:"type"`
}

// generateArgoAdminPassword will generate and return the admin password for Argo CD.
func generateArgoAdminPassword() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultAdminPasswordLength,
		common.ArgoCDDefaultAdminPasswordNumDigits,
		common.ArgoCDDefaultAdminPasswordNumSymbols,
		false, false)

	return []byte(pass), err
}

// generateArgoServerKey will generate and return the server signature key for session validation.
func generateArgoServerSessionKey() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultServerSessionKeyLength,
		common.ArgoCDDefaultServerSessionKeyNumDigits,
		common.ArgoCDDefaultServerSessionKeyNumSymbols,
		false, false)

	return []byte(pass), err
}

// getArgoApplicationControllerResources will return the ResourceRequirements for the Argo CD application controller container.
func getArgoApplicationControllerResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Controller.Resources != nil {
		resources = *cr.Spec.Controller.Resources
	}

	return resources
}

// getArgoApplicationControllerCommand will return the command for the ArgoCD Application Controller component.
func getArgoApplicationControllerCommand(cr *argoprojv1a1.ArgoCD) []string {
	cmd := []string{
		"argocd-application-controller",
		"--operation-processors", fmt.Sprint(getArgoServerOperationProcessors(cr)),
		"--redis", getRedisServerAddress(cr),
		"--repo-server", getRepoServerAddress(cr),
		"--status-processors", fmt.Sprint(getArgoServerStatusProcessors(cr)),
	}
	if cr.Spec.Controller.AppSync != nil {
		cmd = append(cmd, "--app-resync", strconv.FormatInt(int64(cr.Spec.Controller.AppSync.Seconds()), 10))
	}
	return cmd
}

// getArgoContainerImage will return the container image for ArgoCD.
func getArgoContainerImage(cr *argoprojv1a1.ArgoCD) string {
	defaultTag, defaultImg := false, false
	img := cr.Spec.Image
	if img == "" {
		img = common.ArgoCDDefaultArgoImage
		defaultImg = true
	}

	tag := cr.Spec.Version
	if tag == "" {
		tag = common.ArgoCDDefaultArgoVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}

	return argoutil.CombineImageTag(img, tag)
}

// getRepoServerContainerImage will return the container image for the Repo server.
//
// There are three possible options for configuring the image, and this is the
// order of preference.
//
// 1. from the Spec, the spec.repo field has an image and version to use for
// generating an image reference.
// 2. from the Environment, this looks for the `ARGOCD_REPOSERVER_IMAGE` field and uses
// that if the spec is not configured.
// 3. the default is configured in common.ArgoCDDefaultRepoServerVersion and
// common.ArgoCDDefaultRepoServerImage.
func getRepoServerContainerImage(cr *argoprojv1a1.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.Repo.Image
	if img == "" {
		img = common.ArgoCDDefaultArgoImage
		defaultImg = true
	}

	tag := cr.Spec.Repo.Version
	if tag == "" {
		tag = common.ArgoCDDefaultArgoVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getArgoRepoResources will return the ResourceRequirements for the Argo CD Repo server container.
func getArgoRepoResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Repo.Resources != nil {
		resources = *cr.Spec.Repo.Resources
	}

	return resources
}

// getArgoServerInsecure returns the insecure value for the ArgoCD Server component.
func getArgoServerInsecure(cr *argoprojv1a1.ArgoCD) bool {
	return cr.Spec.Server.Insecure
}

func isRepoServerTLSVerificationRequested(cr *argoprojv1a1.ArgoCD) bool {
	return cr.Spec.Repo.VerifyTLS
}

// getArgoServerGRPCHost will return the GRPC host for the given ArgoCD.
func getArgoServerGRPCHost(cr *argoprojv1a1.ArgoCD) string {
	host := nameWithSuffix("grpc", cr)
	if len(cr.Spec.Server.GRPC.Host) > 0 {
		host = cr.Spec.Server.GRPC.Host
	}
	return host
}

// getArgoServerHost will return the host for the given ArgoCD.
func getArgoServerHost(cr *argoprojv1a1.ArgoCD) string {
	host := cr.Name
	if len(cr.Spec.Server.Host) > 0 {
		host = cr.Spec.Server.Host
	}
	return host
}

// getArgoServerResources will return the ResourceRequirements for the Argo CD server container.
func getArgoServerResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	if cr.Spec.Server.Autoscale.Enabled {
		resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(common.ArgoCDDefaultServerResourceLimitCPU),
				corev1.ResourceMemory: resource.MustParse(common.ArgoCDDefaultServerResourceLimitMemory),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(common.ArgoCDDefaultServerResourceRequestCPU),
				corev1.ResourceMemory: resource.MustParse(common.ArgoCDDefaultServerResourceRequestMemory),
			},
		}
	}

	// Allow override of resource requirements from CR
	if cr.Spec.Server.Resources != nil {
		resources = *cr.Spec.Server.Resources
	}

	return resources
}

// getArgoServerURI will return the URI for the ArgoCD server.
// The hostname for argocd-server is from the route, ingress, an external hostname or service name in that order.
func (r *ReconcileArgoCD) getArgoServerURI(cr *argoprojv1a1.ArgoCD) string {
	host := nameWithSuffix("server", cr) // Default to service name

	// Use the external hostname provided by the user
	if cr.Spec.Server.Host != "" {
		host = cr.Spec.Server.Host
	}

	// Use Ingress host if enabled
	if cr.Spec.Server.Ingress.Enabled {
		ing := newIngressWithSuffix("server", cr)
		if argoutil.IsObjectFound(r.client, cr.Namespace, ing.Name, ing) {
			host = ing.Spec.Rules[0].Host
		}
	}

	// Use Route host if available, override Ingress if both exist
	if IsRouteAPIAvailable() {
		route := newRouteWithSuffix("server", cr)
		if argoutil.IsObjectFound(r.client, cr.Namespace, route.Name, route) {
			host = route.Spec.Host
		}
	}

	return fmt.Sprintf("https://%s", host) // TODO: Safe to assume HTTPS here?
}

// getArgoServerOperationProcessors will return the numeric Operation Processors value for the ArgoCD Server.
func getArgoServerOperationProcessors(cr *argoprojv1a1.ArgoCD) int32 {
	op := common.ArgoCDDefaultServerOperationProcessors
	if cr.Spec.Controller.Processors.Operation > op {
		op = cr.Spec.Controller.Processors.Operation
	}
	return op
}

// getArgoServerStatusProcessors will return the numeric Status Processors value for the ArgoCD Server.
func getArgoServerStatusProcessors(cr *argoprojv1a1.ArgoCD) int32 {
	sp := common.ArgoCDDefaultServerStatusProcessors
	if cr.Spec.Controller.Processors.Status > sp {
		sp = cr.Spec.Controller.Processors.Status
	}
	return sp
}

// getDexContainerImage will return the container image for the Dex server.
//
// There are three possible options for configuring the image, and this is the
// order of preference.
//
// 1. from the Spec, the spec.dex field has an image and version to use for
// generating an image reference.
// 2. from the Environment, this looks for the `ARGOCD_DEX_IMAGE` field and uses
// that if the spec is not configured.
// 3. the default is configured in common.ArgoCDDefaultDexVersion and
// common.ArgoCDDefaultDexImage.
func getDexContainerImage(cr *argoprojv1a1.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.Dex.Image
	if img == "" {
		img = common.ArgoCDDefaultDexImage
		defaultImg = true
	}

	tag := cr.Spec.Dex.Version
	if tag == "" {
		tag = common.ArgoCDDefaultDexVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDDexImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getDexOAuthClientID will return the OAuth client ID for the given ArgoCD.
func getDexOAuthClientID(cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDDefaultDexServiceAccountName))
}

// getDexOAuthClientSecret will return the OAuth client secret for the given ArgoCD.
func (r *ReconcileArgoCD) getDexOAuthClientSecret(cr *argoprojv1a1.ArgoCD) (*string, error) {
	sa := newServiceAccountWithName(common.ArgoCDDefaultDexServiceAccountName, cr)
	if err := argoutil.FetchObject(r.client, cr.Namespace, sa.Name, sa); err != nil {
		return nil, err
	}

	// Find the token secret
	var tokenSecret *corev1.ObjectReference
	for _, saSecret := range sa.Secrets {
		if strings.Contains(saSecret.Name, "token") {
			tokenSecret = &saSecret
			break
		}
	}

	if tokenSecret == nil {
		return nil, errors.New("unable to locate ServiceAccount token for OAuth client secret")
	}

	// Fetch the secret to obtain the token
	secret := argoutil.NewSecretWithName(cr.ObjectMeta, tokenSecret.Name)
	if err := argoutil.FetchObject(r.client, cr.Namespace, secret.Name, secret); err != nil {
		return nil, err
	}

	token := string(secret.Data["token"])
	return &token, nil
}

// getDexResources will return the ResourceRequirements for the Dex container.
func getDexResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Dex.Resources != nil {
		resources = *cr.Spec.Dex.Resources
	}

	return resources
}

// getGrafanaContainerImage will return the container image for the Grafana server.
func getGrafanaContainerImage(cr *argoprojv1a1.ArgoCD) string {
	defaultTag, defaultImg := false, false
	img := cr.Spec.Grafana.Image
	if img == "" {
		img = common.ArgoCDDefaultGrafanaImage
		defaultImg = true
	}

	tag := cr.Spec.Grafana.Version
	if tag == "" {
		tag = common.ArgoCDDefaultGrafanaVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDGrafanaImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getGrafanaResources will return the ResourceRequirements for the Grafana container.
func getGrafanaResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Grafana.Resources != nil {
		resources = *cr.Spec.Grafana.Resources
	}

	return resources
}

// getOpenShiftDexConfig will return the configuration for the Dex server running on OpenShift.
func (r *ReconcileArgoCD) getOpenShiftDexConfig(cr *argoprojv1a1.ArgoCD) (string, error) {
	clientSecret, err := r.getDexOAuthClientSecret(cr)
	if err != nil {
		return "", err
	}

	connector := DexConnector{
		Type: "openshift",
		ID:   "openshift",
		Name: "OpenShift",
		Config: map[string]interface{}{
			"issuer":       "https://kubernetes.default.svc", // TODO: Should this be hard-coded?
			"clientID":     getDexOAuthClientID(cr),
			"clientSecret": *clientSecret,
			"redirectURI":  r.getDexOAuthRedirectURI(cr),
			"insecureCA":   true, // TODO: Configure for openshift CA
		},
	}

	connectors := make([]DexConnector, 0)
	connectors = append(connectors, connector)

	dex := make(map[string]interface{})
	dex["connectors"] = connectors

	bytes, err := yaml.Marshal(dex)
	return string(bytes), err
}

// getRedisConfigPath will return the path for the Redis configuration templates.
func getRedisConfigPath() string {
	path := os.Getenv("REDIS_CONFIG_PATH")
	if len(path) > 0 {
		return path
	}
	return common.ArgoCDDefaultRedisConfigPath
}

// getRedisInitScript will load the redis configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisConf(cr *argoprojv1a1.ArgoCD) string {
	path := fmt.Sprintf("%s/redis.conf.tpl", getRedisConfigPath())
	conf, err := loadTemplateFile(path, map[string]string{})
	if err != nil {
		log.Error(err, "unable to load redis configuration")
		return ""
	}
	return conf
}

// getRedisContainerImage will return the container image for the Redis server.
func getRedisContainerImage(cr *argoprojv1a1.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.Redis.Image
	if img == "" {
		img = common.ArgoCDDefaultRedisImage
		defaultImg = true
	}
	tag := cr.Spec.Redis.Version
	if tag == "" {
		tag = common.ArgoCDDefaultRedisVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDRedisImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getRedisHAContainerImage will return the container image for the Redis server in HA mode.
func getRedisHAContainerImage(cr *argoprojv1a1.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.Redis.Image
	if img == "" {
		img = common.ArgoCDDefaultRedisImage
		defaultImg = true
	}
	tag := cr.Spec.Redis.Version
	if tag == "" {
		tag = common.ArgoCDDefaultRedisVersionHA
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDRedisHAImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getRedisHAProxyAddress will return the Redis HA Proxy service address for the given ArgoCD.
func getRedisHAProxyAddress(cr *argoprojv1a1.ArgoCD) string {
	return fqdnServiceRef("redis-ha-haproxy", common.ArgoCDDefaultRedisPort, cr)
}

// getRedisHAProxyContainerImage will return the container image for the Redis HA Proxy.
func getRedisHAProxyContainerImage(cr *argoprojv1a1.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.HA.RedisProxyImage
	if len(img) <= 0 {
		img = common.ArgoCDDefaultRedisHAProxyImage
		defaultImg = true
	}

	tag := cr.Spec.HA.RedisProxyVersion
	if len(tag) <= 0 {
		tag = common.ArgoCDDefaultRedisHAProxyVersion
		defaultTag = true
	}

	if e := os.Getenv(common.ArgoCDRedisHAProxyImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}

	return argoutil.CombineImageTag(img, tag)
}

// getRedisInitScript will load the redis init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisInitScript(cr *argoprojv1a1.ArgoCD) string {
	path := fmt.Sprintf("%s/init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis init-script")
		return ""
	}
	return script
}

// getRedisHAProxySConfig will load the Redis HA Proxy configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisHAProxyConfig(cr *argoprojv1a1.ArgoCD) string {
	path := fmt.Sprintf("%s/haproxy.cfg.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy configuration")
		return ""
	}
	return script
}

// getRedisHAProxyScript will load the Redis HA Proxy init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisHAProxyScript(cr *argoprojv1a1.ArgoCD) string {
	path := fmt.Sprintf("%s/haproxy_init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy init script")
		return ""
	}
	return script
}

// getRedisResources will return the ResourceRequirements for the Redis container.
func getRedisResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Redis.Resources != nil {
		resources = *cr.Spec.Redis.Resources
	}

	return resources
}

// getRedisHAProxyResources will return the ResourceRequirements for the Redis HA Proxy.
func getRedisHAProxyResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.HA.Resources != nil {
		resources = *cr.Spec.HA.Resources
	}

	return resources
}

// getRedisSentinelConf will load the redis sentinel configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisSentinelConf(cr *argoprojv1a1.ArgoCD) string {
	path := fmt.Sprintf("%s/sentinel.conf.tpl", getRedisConfigPath())
	conf, err := loadTemplateFile(path, map[string]string{})
	if err != nil {
		log.Error(err, "unable to load redis sentinel configuration")
		return ""
	}
	return conf
}

// getRedisServerAddress will return the Redis service address for the given ArgoCD.
func getRedisServerAddress(cr *argoprojv1a1.ArgoCD) string {
	if cr.Spec.HA.Enabled {
		return getRedisHAProxyAddress(cr)
	}
	return fqdnServiceRef(common.ArgoCDDefaultRedisSuffix, common.ArgoCDDefaultRedisPort, cr)
}

// loadTemplateFile will parse a template with the given path and execute it with the given params.
func loadTemplateFile(path string, params map[string]string) (string, error) {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		log.Error(err, "unable to parse template")
		return "", err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, params)
	if err != nil {
		log.Error(err, "unable to execute template")
		return "", err
	}
	return buf.String(), nil
}

// nameWithSuffix will return a name based on the given ArgoCD. The given suffix is appended to the generated name.
// Example: Given an ArgoCD with the name "example-argocd", providing the suffix "foo" would result in the value of
// "example-argocd-foo" being returned.
func nameWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("%s-%s", cr.Name, suffix)
}

// fqdnServiceRef will return the FQDN referencing a specific service name, as set up by the operator, with the
// given port.
func fqdnServiceRef(service string, port int, cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("%s.%s.svc.cluster.local:%d", nameWithSuffix(service, cr), cr.Namespace, port)
}

// InspectCluster will verify the availability of extra features available to the cluster, such as Prometheus and
// OpenShift Routes.
func InspectCluster() error {
	if err := verifyPrometheusAPI(); err != nil {
		return err
	}

	if err := verifyRouteAPI(); err != nil {
		return err
	}

	if err := verifyTemplateAPI(); err != nil {
		return err
	}
	return nil
}

// reconcileCertificateAuthority will reconcile all Certificate Authority resources.
func (r *ReconcileArgoCD) reconcileCertificateAuthority(cr *argoprojv1a1.ArgoCD) error {
	log.Info("reconciling CA secret")
	if err := r.reconcileClusterCASecret(cr); err != nil {
		return err
	}

	log.Info("reconciling CA config map")
	if err := r.reconcileCAConfigMap(cr); err != nil {
		return err
	}
	return nil
}

// reconcileResources will reconcile common ArgoCD resources.
func (r *ReconcileArgoCD) reconcileResources(cr *argoprojv1a1.ArgoCD) error {
	log.Info("reconciling status")
	if err := r.reconcileStatus(cr); err != nil {
		return err
	}

	log.Info("reconciling roles")
	if _, err := r.reconcileRoles(cr); err != nil {
		return err
	}

	log.Info("reconciling rolebindings")
	if err := r.reconcileRoleBindings(cr); err != nil {
		return err
	}

	log.Info("reconciling service accounts")
	if err := r.reconcileServiceAccounts(cr); err != nil {
		return err
	}

	log.Info("reconciling certificate authority")
	if err := r.reconcileCertificateAuthority(cr); err != nil {
		return err
	}

	log.Info("reconciling secrets")
	if err := r.reconcileSecrets(cr); err != nil {
		return err
	}

	log.Info("reconciling config maps")
	if err := r.reconcileConfigMaps(cr); err != nil {
		return err
	}

	log.Info("reconciling services")
	if err := r.reconcileServices(cr); err != nil {
		return err
	}

	log.Info("reconciling deployments")
	if err := r.reconcileDeployments(cr); err != nil {
		return err
	}

	log.Info("reconciling statefulsets")
	if err := r.reconcileStatefulSets(cr); err != nil {
		return err
	}

	log.Info("reconciling autoscalers")
	if err := r.reconcileAutoscalers(cr); err != nil {
		return err
	}

	log.Info("reconciling ingresses")
	if err := r.reconcileIngresses(cr); err != nil {
		return err
	}

	if IsRouteAPIAvailable() {
		log.Info("reconciling routes")
		if err := r.reconcileRoutes(cr); err != nil {
			return err
		}
	}

	if IsPrometheusAPIAvailable() {
		log.Info("reconciling prometheus")
		if err := r.reconcilePrometheus(cr); err != nil {
			return err
		}

		if err := r.reconcileMetricsServiceMonitor(cr); err != nil {
			return err
		}

		if err := r.reconcileRepoServerServiceMonitor(cr); err != nil {
			return err
		}

		if err := r.reconcileServerMetricsServiceMonitor(cr); err != nil {
			return err
		}
	}

	if cr.Spec.ApplicationSet != nil {
		log.Info("reconciling ApplicationSet controller")
		if err := r.reconcileApplicationSetController(cr); err != nil {
			return err
		}
	}

	if err := r.reconcileRepoServerTLSSecret(cr); err != nil {
		return err
	}

	if cr.Spec.SSO != nil {
		log.Info("reconciling SSO")
		if err := r.reconcileSSO(cr); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileArgoCD) deleteClusterResources(cr *argoprojv1a1.ArgoCD) error {
	selector, err := argocdInstanceSelector(cr.Name)
	if err != nil {
		return err
	}

	clusterRoleList := &v1.ClusterRoleList{}
	if err := filterObjectsBySelector(r.client, clusterRoleList, selector); err != nil {
		return fmt.Errorf("failed to filter ClusterRoles for %s: %w", cr.Name, err)
	}

	if err := deleteClusterRoles(r.client, clusterRoleList); err != nil {
		return err
	}

	clusterBindingsList := &v1.ClusterRoleBindingList{}
	if err := filterObjectsBySelector(r.client, clusterBindingsList, selector); err != nil {
		return fmt.Errorf("failed to filter ClusterRoleBindings for %s: %w", cr.Name, err)
	}

	if err := deleteClusterRoleBindings(r.client, clusterBindingsList); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileArgoCD) removeManagedByLabelFromNamespace(namespace string) error {
	ns := &corev1.Namespace{}
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: namespace}, ns); err != nil {
		return err
	}

	if ns.Labels == nil {
		return nil
	}
	delete(ns.Labels, common.ArgoCDManagedByLabel)
	return r.client.Update(context.TODO(), ns)
}

func filterObjectsBySelector(c client.Client, objectList runtime.Object, selector labels.Selector) error {
	return c.List(context.TODO(), objectList, client.MatchingLabelsSelector{Selector: selector})
}

func argocdInstanceSelector(name string) (labels.Selector, error) {
	selector := labels.NewSelector()
	requirement, err := labels.NewRequirement(common.ArgoCDKeyManagedBy, selection.Equals, []string{name})
	if err != nil {
		return nil, fmt.Errorf("failed to create a requirement for %w", err)
	}
	return selector.Add(*requirement), nil
}

func (r *ReconcileArgoCD) removeDeletionFinalizer(argocd *argoprojv1a1.ArgoCD) error {
	argocd.Finalizers = removeString(argocd.GetFinalizers(), common.ArgoCDDeletionFinalizer)
	if err := r.client.Update(context.TODO(), argocd); err != nil {
		return fmt.Errorf("failed to remove deletion finalizer from %s: %w", argocd.Name, err)
	}
	return nil
}

func (r *ReconcileArgoCD) addDeletionFinalizer(argocd *argoprojv1a1.ArgoCD) error {
	argocd.Finalizers = append(argocd.Finalizers, common.ArgoCDDeletionFinalizer)
	if err := r.client.Update(context.TODO(), argocd); err != nil {
		return fmt.Errorf("failed to add deletion finalizer for %s: %w", argocd.Name, err)
	}
	return nil
}

func removeString(slice []string, s string) []string {
	var result []string
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return result
}

// labelsForCluster returns the labels for all cluster resources.
func labelsForCluster(cr *argoprojv1a1.ArgoCD) map[string]string {
	labels := argoutil.DefaultLabels(cr.Name)
	for key, val := range cr.ObjectMeta.Labels {
		labels[key] = val
	}
	return labels
}

// annotationsForCluster returns the annotations for all cluster resources.
func annotationsForCluster(cr *argoprojv1a1.ArgoCD) map[string]string {
	annotations := argoutil.DefaultAnnotations(cr)
	for key, val := range cr.ObjectMeta.Annotations {
		annotations[key] = val
	}
	return annotations
}

// watchResources will register Watches for each of the supported Resources.
func watchResources(c controller.Controller, clusterResourceMapper, tlsSecretMapper, namespaceResourceMapper handler.ToRequestsFunc) error {

	deploymentConfigPred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			var count int32 = 1
			newDC, ok := e.ObjectNew.(*oappsv1.DeploymentConfig)
			if !ok {
				return false
			}
			oldDC, ok := e.ObjectOld.(*oappsv1.DeploymentConfig)
			if !ok {
				return false
			}
			if newDC.Name == defaultKeycloakIdentifier {
				if newDC.Status.AvailableReplicas == count {
					return true
				}
				if newDC.Status.AvailableReplicas == int32(0) &&
					!reflect.DeepEqual(oldDC.Status.AvailableReplicas, newDC.Status.AvailableReplicas) {
					// Handle the deletion of keycloak pod.
					log.Info(fmt.Sprintf("Handle the pod deletion event for keycloak deployment config %s in namespace %s",
						newDC.Name, newDC.Namespace))
					err := handleKeycloakPodDeletion(newDC)
					if err != nil {
						log.Error(err, fmt.Sprintf("Failed to update Deployment Config %s for keycloak pod deletion in namespace %s",
							newDC.Name, newDC.Namespace))
					}
				}
			}
			return false
		},
	}

	deleteSSOPred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			newCR, ok := e.ObjectNew.(*argoprojv1a1.ArgoCD)
			if !ok {
				return false
			}
			oldCR, ok := e.ObjectOld.(*argoprojv1a1.ArgoCD)
			if !ok {
				return false
			}
			if !reflect.DeepEqual(oldCR.Spec.SSO, newCR.Spec.SSO) && newCR.Spec.SSO == nil {
				err := deleteSSOConfiguration(newCR)
				if err != nil {
					log.Error(err, fmt.Sprintf("Failed to delete SSO Configuration for ArgoCD %s in namespace %s",
						newCR.Name, newCR.Namespace))
				}
			}
			return true
		},
	}

	// Watch for changes to primary resource ArgoCD
	if err := c.Watch(&source.Kind{Type: &argoprojv1a1.ArgoCD{}}, &handler.EnqueueRequestForObject{}, deleteSSOPred); err != nil {
		return err
	}

	// Watch for changes to ConfigMap sub-resources owned by ArgoCD instances.
	if err := watchOwnedResource(c, &corev1.ConfigMap{}); err != nil {
		return err
	}

	// Watch for changes to Secret sub-resources owned by ArgoCD instances.
	if err := watchOwnedResource(c, &corev1.Secret{}); err != nil {
		return err
	}

	// Watch for changes to Service sub-resources owned by ArgoCD instances.
	if err := watchOwnedResource(c, &corev1.Service{}); err != nil {
		return err
	}

	// Watch for changes to Deployment sub-resources owned by ArgoCD instances.
	if err := watchOwnedResource(c, &appsv1.Deployment{}); err != nil {
		return err
	}

	// Watch for changes to Ingress sub-resources owned by ArgoCD instances.
	if err := watchOwnedResource(c, &extv1beta1.Ingress{}); err != nil {
		return err
	}

	if err := watchOwnedResource(c, &v1.Role{}); err != nil {
		return err
	}

	if err := watchOwnedResource(c, &v1.RoleBinding{}); err != nil {
		return err
	}

	clusterResourceHandler := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: clusterResourceMapper,
	}

	tlsSecretHandler := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: tlsSecretMapper,
	}

	if err := c.Watch(&source.Kind{Type: &v1.ClusterRoleBinding{}}, clusterResourceHandler); err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &v1.ClusterRole{}}, clusterResourceHandler); err != nil {
		return err
	}

	// Watch for secrets of type TLS that might be created by external processes
	if err := c.Watch(&source.Kind{Type: &corev1.Secret{Type: corev1.SecretTypeTLS}}, tlsSecretHandler); err != nil {
		return err
	}

	// Watch for changes to Secret sub-resources owned by ArgoCD instances.
	if err := watchOwnedResource(c, &appsv1.StatefulSet{}); err != nil {
		return err
	}

	// Inspect cluster to verify availability of extra features
	// This sets the flags that are used in subsequent checks
	if err := InspectCluster(); err != nil {
		log.Info("unable to inspect cluster")
	}

	if IsRouteAPIAvailable() {
		// Watch OpenShift Route sub-resources owned by ArgoCD instances.
		if err := watchOwnedResource(c, &routev1.Route{}); err != nil {
			return err
		}
	}

	if IsPrometheusAPIAvailable() {
		// Watch Prometheus sub-resources owned by ArgoCD instances.
		if err := watchOwnedResource(c, &monitoringv1.Prometheus{}); err != nil {
			return err
		}

		// Watch Prometheus ServiceMonitor sub-resources owned by ArgoCD instances.
		if err := watchOwnedResource(c, &monitoringv1.ServiceMonitor{}); err != nil {
			return err
		}
	}

	if IsTemplateAPIAvailable() {
		// Watch for the changes to Deployment Config
		if err := c.Watch(&source.Kind{Type: &oappsv1.DeploymentConfig{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &argoprojv1a1.ArgoCD{},
		},
			deploymentConfigPred); err != nil {
			return err
		}
	}

	namespaceHandler := &handler.EnqueueRequestsFromMapFunc{
		ToRequests: namespaceResourceMapper,
	}

	if err := c.Watch(&source.Kind{Type: &corev1.Namespace{}}, namespaceHandler, namespaceFilterPredicate()); err != nil {
		return err
	}

	return nil
}

func watchOwnedResource(c controller.Controller, obj runtime.Object) error {
	return c.Watch(&source.Kind{Type: obj}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &argoprojv1a1.ArgoCD{},
	})
}

// withClusterLabels will add the given labels to the labels for the cluster and return the result.
func withClusterLabels(cr *argoprojv1a1.ArgoCD, addLabels map[string]string) map[string]string {
	labels := labelsForCluster(cr)
	for key, val := range addLabels {
		labels[key] = val
	}
	return labels
}

// boolPtr returns a pointer to val
func boolPtr(val bool) *bool {
	return &val
}

// triggerRollout will trigger a rollout of a Kubernetes resource specified as
// obj. It currently supports Deployment and StatefulSet resources.
func (r *ReconcileArgoCD) triggerRollout(obj interface{}, key string) error {
	switch res := obj.(type) {
	case *appsv1.Deployment:
		return r.triggerDeploymentRollout(res, key)
	case *appsv1.StatefulSet:
		return r.triggerStatefulSetRollout(res, key)
	default:
		return fmt.Errorf("resource of unknown type %T, cannot trigger rollout", res)
	}
}

func allowedNamespace(current string, namespaces string) bool {

	clusterConfigNamespaces := splitList(namespaces)
	if len(clusterConfigNamespaces) > 0 {
		if clusterConfigNamespaces[0] == "*" {
			return true
		}

		for _, n := range clusterConfigNamespaces {
			if n == current {
				return true
			}
		}
	}
	return false
}

func splitList(s string) []string {
	elems := strings.Split(s, ",")
	for i := range elems {
		elems[i] = strings.TrimSpace(elems[i])
	}
	return elems
}

func containsString(arr []string, s string) bool {
	for _, val := range arr {
		if strings.TrimSpace(val) == s {
			return true
		}
	}
	return false
}

func namespaceFilterPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// This checks if ArgoCDManagedByLabel exists in newMeta, if exists then -
			// 1. Check if oldMeta had the label or not? if no, return true
			// 2. if yes, check if the old and new values are different, if yes,
			// first deleteRBACs for the old value & return true.
			// Event is then handled by the reconciler, which would create appropriate RBACs.
			if valNew, ok := e.MetaNew.GetLabels()[common.ArgoCDManagedByLabel]; ok {
				if valOld, ok := e.MetaOld.GetLabels()[common.ArgoCDManagedByLabel]; ok && valOld != valNew {
					k8sClient, err := initK8sClient()
					if err != nil {
						return false
					}
					if err := deleteRBACsForNamespace(valOld, e.MetaOld.GetName(), k8sClient); err != nil {
						log.Error(err, fmt.Sprintf("failed to delete RBACs for namespace: %s", e.MetaOld.GetName()))
					} else {
						log.Info(fmt.Sprintf("Successfully removed the RBACs for namespace: %s", e.MetaOld.GetName()))
					}
				}
				return true
			}
			// This checks if the old meta had the label, if it did, delete the RBACs for the namespace
			// which were created when the label was added to the namespace.
			if ns, ok := e.MetaOld.GetLabels()[common.ArgoCDManagedByLabel]; ok && ns != "" {
				k8sClient, err := initK8sClient()
				if err != nil {
					return false
				}
				if err := deleteRBACsForNamespace(ns, e.MetaOld.GetName(), k8sClient); err != nil {
					log.Error(err, fmt.Sprintf("failed to delete RBACs for namespace: %s", e.MetaOld.GetName()))
				} else {
					log.Info(fmt.Sprintf("Successfully removed the RBACs for namespace: %s", e.MetaOld.GetName()))
				}
			}
			return false
		},
	}
}

// deleteRBACsForNamespace deletes the RBACs when the label from the namespace is removed.
func deleteRBACsForNamespace(ownerNS, sourceNS string, k8sClient kubernetes.Interface) error {
	log.Info(fmt.Sprintf("Removing the RBACs created for the namespace: %s", sourceNS))

	// List all the roles created for ArgoCD using the label selector
	labelSelector := metav1.LabelSelector{MatchLabels: map[string]string{common.ArgoCDKeyPartOf: common.ArgoCDAppName}}
	roles, err := k8sClient.RbacV1().Roles(sourceNS).List(context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to list roles for namespace: %s", sourceNS))
		return err
	}

	// Delete all the retrieved roles
	for _, role := range roles.Items {
		err = k8sClient.RbacV1().Roles(sourceNS).Delete(context.TODO(), role.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Error(err, fmt.Sprintf("failed to delete roles for namespace: %s", sourceNS))
		}
	}

	// List all the roles bindings created for ArgoCD using the label selector
	roleBindings, err := k8sClient.RbacV1().RoleBindings(sourceNS).List(context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to list role bindings for namespace: %s", sourceNS))
		return err
	}

	// Delete all the retrieved role bindings
	for _, roleBinding := range roleBindings.Items {
		err = k8sClient.RbacV1().RoleBindings(sourceNS).Delete(context.TODO(), roleBinding.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Error(err, fmt.Sprintf("failed to delete role binding for namespace: %s", sourceNS))
		}
	}

	// Get the cluster secret used for configuring ArgoCD
	labelSelector = metav1.LabelSelector{MatchLabels: map[string]string{common.ArgoCDSecretTypeLabel: "cluster"}}
	secrets, err := k8sClient.CoreV1().Secrets(ownerNS).List(context.TODO(), metav1.ListOptions{LabelSelector: labels.Set(labelSelector.MatchLabels).String()})
	if err != nil {
		log.Error(err, fmt.Sprintf("failed to retrieve secrets for namespace: %s", ownerNS))
		return err
	}
	for _, secret := range secrets.Items {
		if string(secret.Data["server"]) != common.ArgoCDDefaultServer {
			continue
		}
		if namespaces, ok := secret.Data["namespaces"]; ok {
			namespaceList := strings.Split(string(namespaces), ",")
			var result []string

			for _, n := range namespaceList {
				// remove the namespace from the list of namespaces
				if strings.TrimSpace(n) == sourceNS {
					continue
				}
				result = append(result, strings.TrimSpace(n))
				sort.Strings(result)
				secret.Data["namespaces"] = []byte(strings.Join(result, ","))
			}
			// Update the secret with the updated list of namespaces
			if _, err = k8sClient.CoreV1().Secrets(ownerNS).Update(context.TODO(), &secret, metav1.UpdateOptions{}); err != nil {
				log.Error(err, fmt.Sprintf("failed to update cluster permission secret for namespace: %s", ownerNS))
				return err
			}
		}
	}
	return nil
}

func initK8sClient() (*kubernetes.Clientset, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get k8s config")
		return nil, err
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "unable to create k8s client")
		return nil, err
	}

	return k8sClient, nil
}
