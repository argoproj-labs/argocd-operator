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

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ArgoCDAppName is the application name for labels.
	ArgoCDAppName = "argocd"

	// ArgoCDConfigMapName is the upstream hard-coded ArgoCD ConfigMap name.
	ArgoCDConfigMapName = "argocd-cm"

	// ArgoCDGrafanaConfigMapSuffix is the default suffix for the Grafana configuration ConfigMap.
	ArgoCDGrafanaConfigMapSuffix = "grafana-config"

	// ArgoCDGrafanaDashboardConfigMapSuffix is the default suffix for the Grafana dashboards ConfigMap.
	ArgoCDGrafanaDashboardConfigMapSuffix = "grafana-dashboards"

	// ArgoCDKnownHostsConfigMapName is the upstream hard-coded SSH known hosts data ConfigMap name.
	ArgoCDKnownHostsConfigMapName = "argocd-ssh-known-hosts-cm"

	// ArgoCDKeyComponent is the resource component key for labels.
	ArgoCDKeyComponent = "app.kubernetes.io/component"

	// ArgoCDKeyName is the resource name key for labels.
	ArgoCDKeyName = "app.kubernetes.io/name"

	// ArgoCDKeyPartOf is the resource part-of key for labels.
	ArgoCDKeyPartOf = "app.kubernetes.io/part-of"

	// ArgoCDRBACConfigMapName is the upstream hard-coded RBAC ConfigMap name.
	ArgoCDRBACConfigMapName = "argocd-rbac-cm"

	// ArgoCDSecretName is the upstream hard-coded ArgoCD Secret name.
	ArgoCDSecretName = "argocd-secret"

	// ArgoCDTLSCertsConfigMapName is the upstream hard-coded TLS certificate data ConfigMap name.
	ArgoCDTLSCertsConfigMapName = "argocd-tls-certs-cm"
)

var isOpenshiftCluster = false

// fetchObject will retrieve the object with the given namespace and name using the Kubernetes API.
// The result will be stored in the given object.
func (r *ReconcileArgoCD) fetchObject(namespace string, name string, obj runtime.Object) error {
	return r.client.Get(context.TODO(), types.NamespacedName{Namespace: namespace, Name: name}, obj)
}

// isObjectFound will perform a basic check that the given object exists via the Kubernetes API.
// If an error occurs as part of the check, the function will return false.
func (r *ReconcileArgoCD) isObjectFound(namespace string, name string, obj runtime.Object) bool {
	if err := r.fetchObject(namespace, name, obj); err != nil {
		return false
	}
	return true
}

// IsOpenShift returns true if the operator is running in an OpenShift environment.
func IsOpenShift() bool {
	return isOpenshiftCluster
}

func nameWithSuffix(suffix string, cr *argoproj.ArgoCD) string {
	return fmt.Sprintf("%s-%s", cr.Name, suffix)
}

// VerifyOpenShift will verify that the OpenShift API is present, indicating an OpenShift cluster.
func VerifyOpenShift() error {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "unable to get k8s config")
		return err
	}

	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Error(err, "unable to create k8s client")
		return err
	}

	gv := schema.GroupVersion{
		Group:   routev1.GroupName,
		Version: routev1.GroupVersion.Version,
	}

	err = discovery.ServerSupportsVersion(k8s, gv)
	if err == nil {
		log.Info("openshift verified")
		isOpenshiftCluster = true
	}
	return nil
}

// reconcileCertificateAuthority will reconcile all Certificate Authority resources.
func (r *ReconcileArgoCD) reconcileCertificateAuthority(cr *argoproj.ArgoCD) error {
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
func (r *ReconcileArgoCD) reconcileOpenShiftResources(cr *argoproj.ArgoCD) error {
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
func (r *ReconcileArgoCD) reconcileResources(cr *argoproj.ArgoCD) error {
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

	if IsOpenShift() {
		log.Info("reconciling openshift resources")
		if err := r.reconcileOpenShiftResources(cr); err != nil {
			return err
		}
	}
	return nil
}

// defaultLabels returns the default set of labels for the cluster.
func defaultLabels(cr *argoproj.ArgoCD) map[string]string {
	return map[string]string{
		ArgoCDKeyName:   cr.Name,
		ArgoCDKeyPartOf: ArgoCDAppName,
	}
}

// labelsForCluster returns the labels for all cluster resources.
func labelsForCluster(cr *argoproj.ArgoCD) map[string]string {
	labels := defaultLabels(cr)
	for key, val := range cr.ObjectMeta.Labels {
		labels[key] = val
	}
	return labels
}

// setDefaults sets the default vaules for the spec and returns true if the spec was changed.
func setDefaults(cr *argoproj.ArgoCD) bool {
	changed := false
	return changed
}

// watchResources will register Watches for each of the supported Resources.
func watchResources(c controller.Controller) error {
	// Watch for changes to primary resource ArgoCD
	if err := c.Watch(&source.Kind{Type: &argoproj.ArgoCD{}}, &handler.EnqueueRequestForObject{}); err != nil {
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

	if IsOpenShift() {
		if err := watchOpenShiftResources(c); err != nil {
			return err
		}
	}
	return nil
}

// watchOpenShiftResources will register Watches for each of the OpenShift supported Resources.
func watchOpenShiftResources(c controller.Controller) error {
	// Watch OpenShift Route sub-resources owned by ArgoCD instances.
	if err := watchOwnedResource(c, &routev1.Route{}); err != nil {
		return err
	}

	// Watch Prometheus sub-resources owned by ArgoCD instances.
	if err := watchOwnedResource(c, &monitoringv1.Prometheus{}); err != nil {
		return err
	}

	// Watch Prometheus ServiceMonitor sub-resources owned by ArgoCD instances.
	if err := watchOwnedResource(c, &monitoringv1.ServiceMonitor{}); err != nil {
		return err
	}
	return nil
}

func watchOwnedResource(c controller.Controller, obj runtime.Object) error {
	return c.Watch(&source.Kind{Type: obj}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &argoproj.ArgoCD{},
	})
}

// withClusterLabels will add the given labels to the labels for the cluster and return the result.
func withClusterLabels(cr *argoproj.ArgoCD, addLabels map[string]string) map[string]string {
	labels := labelsForCluster(cr)
	for key, val := range addLabels {
		labels[key] = val
	}
	return labels
}
