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
	"errors"
	"fmt"
	"strings"

	argoproj "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj"
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"gopkg.in/yaml.v2"

	routev1 "github.com/openshift/api/route/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
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

// getArgoContainerImage will return the container image for ArgoCD.
func getArgoContainerImage(cr *argoprojv1a1.ArgoCD) string {
	img := cr.Spec.Image
	if len(img) <= 0 {
		img = argoproj.ArgoCDDefaultArgoImage
	}

	tag := cr.Spec.Version
	if len(tag) <= 0 {
		tag = argoproj.ArgoCDDefaultArgoVersion
	}
	return fmt.Sprintf("%s:%s", img, tag)
}

// getArgoDexConfiguration will return the configuration for the Dex server.
// The configuration will be returned as a YAML string
func (r *ReconcileArgoCD) getArgoDexConfiguration(cr *argoprojv1a1.ArgoCD) (string, error) {
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

// getArgoServerInsecure returns the insecure value for the ArgoCD Server component.
func getArgoServerInsecure(cr *argoprojv1a1.ArgoCD) bool {
	return cr.Spec.Server.Insecure
}

// getArgoServerGRPCHost will retun the GRPC host for the given ArgoCD.
func getArgoServerGRPCHost(cr *argoprojv1a1.ArgoCD) string {
	host := nameWithSuffix("grpc", cr)
	if len(cr.Spec.Server.GRPC.Host) > 0 {
		host = cr.Spec.Server.GRPC.Host
	}
	return host
}

// getArgoServerHost will retun the host for the given ArgoCD.
func getArgoServerHost(cr *argoprojv1a1.ArgoCD) string {
	host := cr.Name
	if len(cr.Spec.Server.Host) > 0 {
		host = cr.Spec.Server.Host
	}
	return host
}

// getArgoServerURI will return the URI for the ArgoCD server.
// The hostname for argocd-server is from the route, ingress or service name in that order.
func (r *ReconcileArgoCD) getArgoServerURI(cr *argoprojv1a1.ArgoCD) string {
	host := nameWithSuffix("server", cr) // Default to service name

	// Use Ingress host if enabled
	if cr.Spec.Ingress.Enabled {
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
	op := argoproj.ArgoCDDefaultArgoServerOperationProcessors
	if cr.Spec.Controller.Processors.Operation > op {
		op = cr.Spec.Controller.Processors.Operation
	}
	return op
}

// getArgoServerStatusProcessors will return the numeric Status Processors value for the ArgoCD Server.
func getArgoServerStatusProcessors(cr *argoprojv1a1.ArgoCD) int32 {
	sp := argoproj.ArgoCDDefaultArgoServerStatusProcessors
	if cr.Spec.Controller.Processors.Status > sp {
		sp = cr.Spec.Controller.Processors.Status
	}
	return sp
}

// getDexContainerImage will return the container image for the Dex server.
func getDexContainerImage(cr *argoprojv1a1.ArgoCD) string {
	img := cr.Spec.Dex.Image
	if len(img) <= 0 {
		img = argoproj.ArgoCDDefaultDexImage
	}

	tag := cr.Spec.Dex.Version
	if len(tag) <= 0 {
		tag = argoproj.ArgoCDDefaultDexVersion
	}
	return fmt.Sprintf("%s:%s", img, tag)
}

// getDexInitContainers will return the init-containers for the Dex server.
func getDexInitContainers(cr *argoprojv1a1.ArgoCD) []corev1.Container {
	ics := []corev1.Container{{
		Command: []string{
			"cp",
			"/usr/local/bin/argocd-util",
			"/shared",
		},
		Image:           getArgoContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "copyutil",
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "static-files",
			MountPath: "/shared",
		}},
	}}

	// Add Oauth configuration if enabled.
	if cr.Spec.Dex.OAuth != nil && cr.Spec.Dex.OAuth.Enabled {
		ic := corev1.Container{
			Command: []string{
				"sleep",
				"3",
			},
			Image:           getDexContainerImage(cr),
			ImagePullPolicy: corev1.PullAlways,
			Name:            "openshift-oauth-config",
		}

		ics = append(ics, ic)
	}

	return ics
}

// getDexOAuthClientID will return the OAuth client ID for the given ArgoCD.
func getDexOAuthClientID(cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", cr.Namespace, argoproj.ArgoCDDefaultDexServiceAccountName)
}

// getDexOAuthClientID will return the OAuth client secret for the given ArgoCD.
func (r *ReconcileArgoCD) getDexOAuthClientSecret(cr *argoprojv1a1.ArgoCD) (*string, error) {
	sa := newServiceAccountWithName(argoproj.ArgoCDDefaultDexServiceAccountName, cr)
	log.Info(fmt.Sprintf("Fetching Service Account: %s", sa.Name))
	if err := argoutil.FetchObject(r.client, cr.Namespace, sa.Name, sa); err != nil {
		return nil, err
	}

	// Find the token secret
	log.Info("Locating Service Account Token Secret")
	var tokenSecret *corev1.ObjectReference
	for _, saSecret := range sa.Secrets {
		log.Info(fmt.Sprintf("Found Secret: %s", saSecret.Name))
		if strings.Contains(saSecret.Name, "token") {
			log.Info(fmt.Sprintf("Using Secret: %s", saSecret.Name))
			tokenSecret = &saSecret
			break
		}
	}

	if tokenSecret == nil {
		return nil, errors.New("unable to locate ServiceAccount token for OAuth client secret")
	}

	// Fetch the secret to obtain the token
	secret := newSecretWithName(tokenSecret.Name, cr)
	log.Info(fmt.Sprintf("Fetching Service Account Token Secret: %s", secret.Name))
	if err := argoutil.FetchObject(r.client, cr.Namespace, secret.Name, secret); err != nil {
		return nil, err
	}

	token := string(secret.Data["token"])
	log.Info(fmt.Sprintf("Service Account Token: %s", token))
	return &token, nil
}

// getGrafanaContainerImage will return the container image for the Grafana server.
func getGrafanaContainerImage(cr *argoprojv1a1.ArgoCD) string {
	img := cr.Spec.Grafana.Image
	if len(img) <= 0 {
		img = argoproj.ArgoCDDefaultGrafanaImage
	}

	tag := cr.Spec.Grafana.Version
	if len(tag) <= 0 {
		tag = argoproj.ArgoCDDefaultGrafanaVersion
	}
	return fmt.Sprintf("%s:%s", img, tag)
}

// getRedisContainerImage will return the container image for the Redis server.
func getRedisContainerImage(cr *argoprojv1a1.ArgoCD) string {
	img := cr.Spec.Redis.Image
	if len(img) <= 0 {
		img = argoproj.ArgoCDDefaultRedisImage
	}

	tag := cr.Spec.Redis.Version
	if len(tag) <= 0 {
		tag = argoproj.ArgoCDDefaultRedisVersion
	}
	return fmt.Sprintf("%s:%s", img, tag)
}

func nameWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) string {
	return fmt.Sprintf("%s-%s", cr.Name, suffix)
}

// InspectCluster will verify the availability of extra features.
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
	if err := r.reconcileCASecret(cr); err != nil {
		return err
	}

	log.Info("reconciling CA config map")
	if err := r.reconcileCAConfigMap(cr); err != nil {
		return err
	}
	return nil
}

// reconcileOpenShiftResources will reconcile OpenShift specific ArgoCD resources.
func (r *ReconcileArgoCD) reconcileOpenShiftResources(cr *argoprojv1a1.ArgoCD) error {
	if err := r.reconcileRoutes(cr); err != nil {
		return err
	}

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
	return nil
}

// reconcileResources will reconcile common ArgoCD resources.
func (r *ReconcileArgoCD) reconcileResources(cr *argoprojv1a1.ArgoCD) error {
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

// setDefaults sets the default vaules for the spec and returns true if the spec was changed.
func setDefaults(cr *argoprojv1a1.ArgoCD) bool {
	changed := false
	return changed
}

// watchResources will register Watches for each of the supported Resources.
func watchResources(c controller.Controller) error {
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
