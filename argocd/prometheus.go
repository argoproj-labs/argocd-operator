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

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	routev1 "github.com/openshift/api/route/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// getPrometheusHost will return the hostname value for Prometheus.
func getPrometheusHost(cr *v1alpha1.ArgoCD) string {
	host := common.NameWithSuffix(cr.ObjectMeta, "prometheus")
	if len(cr.Spec.Prometheus.Host) > 0 {
		host = cr.Spec.Prometheus.Host
	}
	return host
}

// getPrometheusSize will return the size value for the Prometheus replica count.
func getPrometheusReplicas(cr *v1alpha1.ArgoCD) *int32 {
	replicas := common.ArgoCDDefaultPrometheusReplicas
	if cr.Spec.Prometheus.Size != nil {
		if *cr.Spec.Prometheus.Size >= 0 && *cr.Spec.Prometheus.Size != replicas {
			replicas = *cr.Spec.Prometheus.Size
		}
	}
	return &replicas
}

// hasPrometheusSpecChanged will return true if the supported properties differs in the actual versus the desired state.
func hasPrometheusSpecChanged(actual *monitoringv1.Prometheus, desired *v1alpha1.ArgoCD) bool {
	// Replica count
	if desired.Spec.Prometheus.Size != nil && *desired.Spec.Prometheus.Size >= 0 { // Valid replica count specified in desired state
		if actual.Spec.Replicas != nil { // Actual replicas value is set
			if *actual.Spec.Replicas != *desired.Spec.Prometheus.Size {
				return true
			}
		} else if *desired.Spec.Prometheus.Size != common.ArgoCDDefaultPrometheusReplicas { // Actual replicas value is NOT set, but desired replicas differs from the default
			return true
		}
	} else { // Replica count NOT specified in desired state
		if actual.Spec.Replicas != nil && *actual.Spec.Replicas != common.ArgoCDDefaultPrometheusReplicas {
			return true
		}
	}
	return false
}

// reconcilePrometheusIngress will ensure that the Prometheus Ingress is present.
func (r *ArgoClusterReconciler) reconcilePrometheusIngress(cr *v1alpha1.ArgoCD) error {
	ingress := resources.NewIngressWithSuffix(cr.ObjectMeta, "prometheus")
	if resources.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
		return nil // Prometheus itself or Ingress not enabled, move along...
	}

	// Add annotations
	atns := common.DefaultIngressAnnotations()
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Prometheus.Ingress.Annotations) > 0 {
		atns = cr.Spec.Prometheus.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getPrometheusHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Prometheus.Ingress.Path),
							Backend: extv1beta1.IngressBackend{
								ServiceName: "prometheus-operated",
								ServicePort: intstr.FromString("web"),
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
			Hosts:      []string{cr.Name},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Prometheus.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Prometheus.Ingress.TLS
	}

	ctrl.SetControllerReference(cr, ingress, r.Scheme)
	return r.Client.Create(context.TODO(), ingress)
}

// reconcilePrometheus will ensure that Prometheus is present for ArgoCD metrics.
func (r *ArgoClusterReconciler) reconcilePrometheus(cr *v1alpha1.ArgoCD) error {
	prometheus := resources.NewPrometheus(cr.ObjectMeta)
	if resources.IsObjectFound(r.Client, cr.Namespace, prometheus.Name, prometheus) {
		if !cr.Spec.Prometheus.Enabled {
			// Prometheus exists but enabled flag has been set to false, delete the Prometheus
			return r.Client.Delete(context.TODO(), prometheus)
		}
		if hasPrometheusSpecChanged(prometheus, cr) {
			prometheus.Spec.Replicas = cr.Spec.Prometheus.Size
			return r.Client.Update(context.TODO(), prometheus)
		}
		return nil // Prometheus found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	prometheus.Spec.Replicas = getPrometheusReplicas(cr)
	prometheus.Spec.ServiceAccountName = "prometheus-k8s"
	prometheus.Spec.ServiceMonitorSelector = &metav1.LabelSelector{}

	ctrl.SetControllerReference(cr, prometheus, r.Scheme)
	return r.Client.Create(context.TODO(), prometheus)
}

// reconcilePrometheusRoute will ensure that the ArgoCD Prometheus Route is present.
func (r *ArgoClusterReconciler) reconcilePrometheusRoute(cr *v1alpha1.ArgoCD) error {
	route := resources.NewRouteWithSuffix(cr.ObjectMeta, "prometheus")
	if resources.IsObjectFound(r.Client, cr.Namespace, route.Name, route) {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Route.Enabled {
			// Route exists but enabled flag has been set to false, delete the Route
			return r.Client.Delete(context.TODO(), route)
		}
		return nil // Route found, do nothing
	}

	if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Route.Enabled {
		return nil // Prometheus itself or Route not enabled, do nothing.
	}

	// Allow override of the Annotations for the Route.
	if len(cr.Spec.Prometheus.Route.Annotations) > 0 {
		route.Annotations = cr.Spec.Prometheus.Route.Annotations
	}

	// Allow override of the Host for the Route.
	if len(cr.Spec.Prometheus.Host) > 0 {
		route.Spec.Host = cr.Spec.Prometheus.Host // TODO: What additional role needed for this?
	}

	route.Spec.Port = &routev1.RoutePort{
		TargetPort: intstr.FromString("web"),
	}

	// Allow override of TLS options for the Route
	if cr.Spec.Prometheus.Route.TLS != nil {
		route.Spec.TLS = cr.Spec.Prometheus.Route.TLS
	}

	route.Spec.To.Kind = "Service"
	route.Spec.To.Name = "prometheus-operated"

	// Allow override of the WildcardPolicy for the Route
	if cr.Spec.Prometheus.Route.WildcardPolicy != nil && len(*cr.Spec.Prometheus.Route.WildcardPolicy) > 0 {
		route.Spec.WildcardPolicy = *cr.Spec.Prometheus.Route.WildcardPolicy
	}

	ctrl.SetControllerReference(cr, route, r.Scheme)
	return r.Client.Create(context.TODO(), route)
}
