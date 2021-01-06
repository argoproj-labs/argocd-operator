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
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sethvargo/go-password/password"
	"gopkg.in/yaml.v2"

	routev1 "github.com/openshift/api/route/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
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
// The hostname for argocd-server is from the route, ingress or service name in that order.
func (r *ReconcileArgoCD) getArgoServerURI(cr *argoprojv1a1.ArgoCD) string {
	host := nameWithSuffix("server", cr) // Default to service name

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
	return fmt.Sprintf("system:serviceaccount:%s:%s", cr.Namespace, common.ArgoCDDefaultDexServiceAccountName)
}

// getDexOAuthClientID will return the OAuth client secret for the given ArgoCD.
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
	suffix := fmt.Sprintf("redis-ha-haproxy:%v", common.ArgoCDDefaultRedisPort)
	return nameWithSuffix(suffix, cr)
}

// getRedisHAProxyContainerImage will return the container image for the Redis HA Proxy.
func getRedisHAProxyContainerImage(cr *argoprojv1a1.ArgoCD) string {
	img := cr.Spec.HA.RedisProxyImage
	if len(img) <= 0 {
		img = common.ArgoCDDefaultRedisHAProxyImage
	}

	tag := cr.Spec.HA.RedisProxyVersion
	if len(tag) <= 0 {
		tag = common.ArgoCDDefaultRedisHAProxyVersion
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

	suffix := fmt.Sprintf("%s:%v", common.ArgoCDDefaultRedisSuffix, common.ArgoCDDefaultRedisPort)
	return nameWithSuffix(suffix, cr)
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

// InspectCluster will verify the availability of extra features available to the cluster, such as Prometheus and
// OpenShift Routes.
func InspectCluster() error {
	if err := verifyPrometheusAPI(); err != nil {
		return err
	}

	if err := verifyRouteAPI(); err != nil {
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

	return nil
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
func watchResources(c controller.Controller, clusterResourceMapper handler.ToRequestsFunc) error {
	// Watch for changes to primary resource ArgoCD
	if err := c.Watch(&source.Kind{Type: &argoprojv1a1.ArgoCD{}}, &handler.EnqueueRequestForObject{}); err != nil {
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

	if err := c.Watch(&source.Kind{Type: &v1.ClusterRoleBinding{}}, clusterResourceHandler); err != nil {
		return err
	}

	if err := c.Watch(&source.Kind{Type: &v1.ClusterRole{}}, clusterResourceHandler); err != nil {
		return err
	}

	// Watch for changes to Secret sub-resources owned by ArgoCD instances.
	if err := watchOwnedResource(c, &appsv1.StatefulSet{}); err != nil {
		return err
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
