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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/sethvargo/go-password/password"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// GrafanaConfig represents the Grafana configuration options.
type GrafanaConfig struct {
	// Security options
	Security GrafanaSecurityConfig
}

// GrafanaSecurityConfig represents the Grafana security options.
type GrafanaSecurityConfig struct {
	// AdminUser is the default admin user.
	AdminUser string

	// AdminPassword is the default admin password
	AdminPassword string

	// SecretKey is used for signing
	SecretKey string
}

// generateGrafanaSecretKey will generate and return the secret key for Grafana.
func generateGrafanaSecretKey() ([]byte, error) {
	key, err := password.Generate(
		common.ArgoCDDefaultGrafanaSecretKeyLength,
		common.ArgoCDDefaultGrafanaSecretKeyNumDigits,
		common.ArgoCDDefaultGrafanaSecretKeyNumSymbols,
		false, false)

	return []byte(key), err
}

// getGrafanaConfigPath will return the path for the Grafana configuration templates
func getGrafanaConfigPath() string {
	path := os.Getenv("GRAFANA_CONFIG_PATH")
	if len(path) > 0 {
		return path
	}
	return common.ArgoCDDefaultGrafanaConfigPath
}

// getGrafanaContainerImage will return the container image for the Grafana server.
func getGrafanaContainerImage(cr *v1alpha1.ArgoCD) string {
	img := cr.Spec.Grafana.Image
	if len(img) <= 0 {
		img = common.ArgoCDDefaultGrafanaImage
	}

	tag := cr.Spec.Grafana.Version
	if len(tag) <= 0 {
		tag = common.ArgoCDDefaultGrafanaVersion
	}
	return common.CombineImageTag(img, tag)
}

// getGrafanaHost will return the hostname value for Grafana.
func getGrafanaHost(cr *v1alpha1.ArgoCD) string {
	host := common.NameWithSuffix(cr.ObjectMeta, "grafana")
	if len(cr.Spec.Grafana.Host) > 0 {
		host = cr.Spec.Grafana.Host
	}
	return host
}

// getGrafanaReplicas will return the size value for the Grafana replica count.
func getGrafanaReplicas(cr *v1alpha1.ArgoCD) *int32 {
	replicas := common.ArgoCDDefaultGrafanaReplicas
	if cr.Spec.Grafana.Size != nil {
		if *cr.Spec.Grafana.Size >= 0 && *cr.Spec.Grafana.Size != replicas {
			replicas = *cr.Spec.Grafana.Size
		}
	}
	return &replicas
}

// getGrafanaResources will return the ResourceRequirements for the Grafana container.
func getGrafanaResources(cr *v1alpha1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Grafana.Resources != nil {
		resources = *cr.Spec.Grafana.Resources
	}

	return resources
}

// loadGrafanaConfigs will scan the config directory and read any files ending with '.yaml'
func loadGrafanaConfigs() (map[string]string, error) {
	data := make(map[string]string)

	pattern := filepath.Join(getGrafanaConfigPath(), "*.yaml")
	configs, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	for _, f := range configs {
		config, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}

		parts := strings.Split(f, "/")
		filename := parts[len(parts)-1]
		data[filename] = string(config)
	}

	return data, nil
}

// hasGrafanaSpecChanged will return true if the supported properties differs in the actual versus the desired state.
func hasGrafanaSpecChanged(actual *appsv1.Deployment, desired *v1alpha1.ArgoCD) bool {
	// Replica count
	if desired.Spec.Grafana.Size != nil { // Replica count specified in desired state
		if *desired.Spec.Grafana.Size >= 0 && *actual.Spec.Replicas != *desired.Spec.Grafana.Size {
			return true
		}
	} else { // Replica count NOT specified in desired state
		if *actual.Spec.Replicas != common.ArgoCDDefaultGrafanaReplicas {
			return true
		}
	}
	return false
}

// loadGrafanaTemplates will scan the template directory and parse/execute any files ending with '.tmpl'
func loadGrafanaTemplates(c *GrafanaConfig) (map[string]string, error) {
	data := make(map[string]string)

	templateDir := filepath.Join(getGrafanaConfigPath(), "templates")
	entries, err := ioutil.ReadDir(templateDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue // Ignore directories and anything that doesn't end with '.tmpl'
		}

		filename := entry.Name()
		path := filepath.Join(templateDir, filename)
		tmpl, err := template.ParseFiles(path)
		if err != nil {
			return nil, err
		}

		buf := new(bytes.Buffer)
		err = tmpl.Execute(buf, c)
		if err != nil {
			return nil, err
		}

		parts := strings.Split(filename, ".tmpl")
		if len(parts) <= 1 {
			return nil, fmt.Errorf("invalid template name: %s", filename)
		}

		key := parts[0]
		data[key] = buf.String()
	}

	return data, nil
}

// reconcileGrafanaConfiguration will ensure that the Grafana configuration ConfigMap is present.
func (r *ArgoClusterReconciler) reconcileGrafanaConfiguration(cr *v1alpha1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := resources.NewConfigMapWithSuffix(cr.ObjectMeta, common.ArgoCDGrafanaConfigMapSuffix)
	if resources.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		return nil // ConfigMap found, do nothing
	}

	secret := resources.NewSecretWithSuffix(cr.ObjectMeta, "grafana")
	secret, err := resources.FetchSecret(r.Client, cr.ObjectMeta, secret.Name)
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

	ctrl.SetControllerReference(cr, cm, r.Scheme)
	return r.Client.Create(context.TODO(), cm)
}

// reconcileGrafanaDashboards will ensure that the Grafana dashboards ConfigMap is present.
func (r *ArgoClusterReconciler) reconcileGrafanaDashboards(cr *v1alpha1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	cm := resources.NewConfigMapWithSuffix(cr.ObjectMeta, common.ArgoCDGrafanaDashboardConfigMapSuffix)
	if resources.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
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

	ctrl.SetControllerReference(cr, cm, r.Scheme)
	return r.Client.Create(context.TODO(), cm)
}

// reconcileGrafanaDeployment will ensure the Deployment resource is present for the ArgoCD Grafana component.
func (r *ArgoClusterReconciler) reconcileGrafanaDeployment(cr *v1alpha1.ArgoCD) error {
	deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "grafana", "grafana")
	if resources.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
		if !cr.Spec.Grafana.Enabled {
			// Deployment exists but enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), deploy)
		}
		if hasGrafanaSpecChanged(deploy, cr) {
			deploy.Spec.Replicas = cr.Spec.Grafana.Size
			return r.Client.Update(context.TODO(), deploy)
		}
		return nil // Deployment found, do nothing
	}

	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	deploy.Spec.Replicas = getGrafanaReplicas(cr)

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Image:           getGrafanaContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "grafana",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 3000,
			},
		},
		Resources: getGrafanaResources(cr),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "grafana-config",
				MountPath: "/etc/grafana",
			}, {
				Name:      "grafana-datasources-config",
				MountPath: "/etc/grafana/provisioning/datasources",
			}, {
				Name:      "grafana-dashboards-config",
				MountPath: "/etc/grafana/provisioning/dashboards",
			}, {
				Name:      "grafana-dashboard-templates",
				MountPath: "/var/lib/grafana/dashboards",
			},
		},
	}}

	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "grafana-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.NameWithSuffix(cr.ObjectMeta, "grafana-config"),
					},
					Items: []corev1.KeyToPath{{
						Key:  "grafana.ini",
						Path: "grafana.ini",
					}},
				},
			},
		}, {
			Name: "grafana-datasources-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.NameWithSuffix(cr.ObjectMeta, "grafana-config"),
					},
					Items: []corev1.KeyToPath{{
						Key:  "datasource.yaml",
						Path: "datasource.yaml",
					}},
				},
			},
		}, {
			Name: "grafana-dashboards-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.NameWithSuffix(cr.ObjectMeta, "grafana-config"),
					},
					Items: []corev1.KeyToPath{{
						Key:  "provider.yaml",
						Path: "provider.yaml",
					}},
				},
			},
		}, {
			Name: "grafana-dashboard-templates",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.NameWithSuffix(cr.ObjectMeta, "grafana-dashboards"),
					},
				},
			},
		},
	}

	ctrl.SetControllerReference(cr, deploy, r.Scheme)
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileGrafanaIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ArgoClusterReconciler) reconcileGrafanaIngress(cr *v1alpha1.ArgoCD) error {
	ingress := resources.NewIngressWithSuffix(cr.ObjectMeta, "grafana")
	if resources.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled {
		return nil // Grafana itself or Ingress not enabled, move along...
	}

	// Add annotations
	atns := common.DefaultIngressAnnotations()
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Grafana.Ingress.Annotations) > 0 {
		atns = cr.Spec.Grafana.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getGrafanaHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Grafana.Ingress.Path),
							Backend: extv1beta1.IngressBackend{
								ServiceName: common.NameWithSuffix(cr.ObjectMeta, "grafana"),
								ServicePort: intstr.FromString("http"),
							},
						},
					},
				},
			},
		},
	}

	// Add TLS options
	ingress.Spec.TLS = []extv1beta1.IngressTLS{
		{
			Hosts: []string{
				cr.Name,
				getGrafanaHost(cr),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Grafana.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Grafana.Ingress.TLS
	}

	ctrl.SetControllerReference(cr, ingress, r.Scheme)
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileGrafanaRoute will ensure that the ArgoCD Grafana Route is present.
func (r *ArgoClusterReconciler) reconcileGrafanaRoute(cr *v1alpha1.ArgoCD) error {
	route := resources.NewRouteWithSuffix(cr.ObjectMeta, "grafana")
	if resources.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
		if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
		return nil // Route found, do nothing
	}

	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Route.Enabled {
		return nil // Grafana itself or Route not enabled, do nothing.
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Grafana.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Grafana.Route.Annotations
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Grafana.Host) > 0 {
		route.Spec.Host = cr.Spec.Grafana.Host // TODO: What additional role needed for this?
	}

	// Allow override of the Path for the Route
	if len(cr.Spec.Grafana.Route.Path) > 0 {
		route.Spec.Path = cr.Spec.Grafana.Route.Path
	}

	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("http"),
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Grafana.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Grafana.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = common.NameWithSuffix(cr.ObjectMeta, "grafana")

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Grafana.Route.WildcardPolicy != nil && len(*cr.Spec.Grafana.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Grafana.Route.WildcardPolicy
	}

	ctrl.SetControllerReference(cr, route, r.Scheme)
	return r.Client.Create(context.TODO(), route)
}

// reconcileGrafanaSecret will ensure that the Grafana Secret is present.
func (r *ArgoClusterReconciler) reconcileGrafanaSecret(cr *v1alpha1.ArgoCD) error {
	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	clusterSecret := resources.NewSecretWithSuffix(cr.ObjectMeta, "cluster")
	secret := resources.NewSecretWithSuffix(cr.ObjectMeta, "grafana")

	if !resources.IsObjectFound(r.Client, cr.Namespace, clusterSecret.Name, clusterSecret) {
		log.Info(fmt.Sprintf("cluster secret [%s] not found, waiting to reconcile grafana secret [%s]", clusterSecret.Name, secret.Name))
		return nil
	}

	if resources.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		actual := string(secret.Data[common.ArgoCDKeyGrafanaAdminPassword])
		expected := string(clusterSecret.Data[common.ArgoCDKeyAdminPassword])

		if actual != expected {
			log.Info("cluster secret changed, updating and reloading grafana")
			secret.Data[common.ArgoCDKeyGrafanaAdminPassword] = clusterSecret.Data[common.ArgoCDKeyAdminPassword]
			if err := r.Client.Update(context.TODO(), secret); err != nil {
				return err
			}

			// Regenerate the Grafana configuration
			cm := resources.NewConfigMapWithSuffix(cr.ObjectMeta, "grafana-config")
			if !resources.IsObjectFound(r.Client, cm.Namespace, cm.Name, cm) {
				log.Info("unable to locate grafana-config")
				return nil
			}

			if err := r.Client.Delete(context.TODO(), cm); err != nil {
				return err
			}

			// Trigger rollout of Grafana Deployment
			deploy := resources.NewDeploymentWithSuffix(cr.ObjectMeta, "grafana", "grafana")
			return r.triggerRollout(deploy, "admin.password.changed")
		}
		return nil // Nothing has changed, move along...
	}

	// Secret not found, create it...

	secretKey, err := generateGrafanaSecretKey()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyGrafanaAdminUsername: []byte(common.ArgoCDDefaultGrafanaAdminUsername),
		common.ArgoCDKeyGrafanaAdminPassword: clusterSecret.Data[common.ArgoCDKeyAdminPassword],
		common.ArgoCDKeyGrafanaSecretKey:     secretKey,
	}

	ctrl.SetControllerReference(cr, secret, r.Scheme)
	return r.Client.Create(context.TODO(), secret)
}

// reconcileGrafanaService will ensure that the Service for Grafana is present.
func (r *ArgoClusterReconciler) reconcileGrafanaService(cr *v1alpha1.ArgoCD) error {
	svc := resources.NewServiceWithSuffix(cr.ObjectMeta, "grafana", "grafana")
	if resources.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.Grafana.Enabled {
			// Service exists but enabled flag has been set to false, delete the Service
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: common.NameWithSuffix(cr.ObjectMeta, "grafana"),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       80,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(3000),
		},
	}

	ctrl.SetControllerReference(cr, svc, r.Scheme)
	return r.Client.Create(context.TODO(), svc)
}
