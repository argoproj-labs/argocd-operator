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

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// getArgoServerPath will return the Ingress Path for the Argo CD component.
func getPathOrDefault(path string) string {
	result := common.ArgoCDDefaultIngressPath
	if len(path) > 0 {
		result = path
	}
	return result
}

// newIngress returns a new Ingress instance for the given ArgoCD.
func newIngress(cr *argoproj.ArgoCD) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
	}
}

// newIngressWithName returns a new Ingress with the given name and ArgoCD.
func newIngressWithName(name string, cr *argoproj.ArgoCD) *networkingv1.Ingress {
	ingress := newIngress(cr)
	ingress.ObjectMeta.Name = name

	lbls := ingress.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	ingress.ObjectMeta.Labels = lbls

	return ingress
}

// newIngressWithSuffix returns a new Ingress with the given name suffix for the ArgoCD.
func newIngressWithSuffix(suffix string, cr *argoproj.ArgoCD) *networkingv1.Ingress {
	return newIngressWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), cr)
}

// reconcileIngresses will ensure that all ArgoCD Ingress resources are present.
func (r *ReconcileArgoCD) reconcileIngresses(cr *argoproj.ArgoCD) error {
	if err := r.reconcileArgoServerIngress(cr); err != nil {
		return err
	}

	if err := r.reconcileArgoServerGRPCIngress(cr); err != nil {
		return err
	}

	if err := r.reconcileGrafanaIngress(cr); err != nil {
		return err
	}

	if err := r.reconcilePrometheusIngress(cr); err != nil {
		return err
	}

	if err := r.reconcileApplicationSetControllerIngress(cr); err != nil {
		return err
	}

	return nil
}

// reconcileArgoServerIngress will ensure that the ArgoCD Server Ingress is present.
func (r *ReconcileArgoCD) reconcileArgoServerIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("server", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Server.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Server.Ingress.Enabled {
		return nil // Ingress not enabled, move along...
	}

	// Add default annotations
	atns := make(map[string]string)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Server.Ingress.Annotations) > 0 {
		atns = cr.Spec.Server.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	ingress.Spec.IngressClassName = cr.Spec.Server.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	// Add rules
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getArgoServerHost(cr),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Server.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: nameWithSuffix("server", cr),
									Port: networkingv1.ServiceBackendPort{
										Name: "http",
									},
								},
							},
							PathType: &pathType,
						},
					},
				},
			},
		},
	}

	// Add default TLS options
	ingress.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts: []string{
				getArgoServerHost(cr),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Server.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Server.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileArgoServerGRPCIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ReconcileArgoCD) reconcileArgoServerGRPCIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("grpc", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Server.GRPC.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Server.GRPC.Ingress.Enabled {
		return nil // Ingress not enabled, move along...
	}

	// Add default annotations
	atns := make(map[string]string)
	atns[common.ArgoCDKeyIngressBackendProtocol] = "GRPC"

	// Override default annotations if specified
	if len(cr.Spec.Server.GRPC.Ingress.Annotations) > 0 {
		atns = cr.Spec.Server.GRPC.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	ingress.Spec.IngressClassName = cr.Spec.Server.GRPC.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	// Add rules
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getArgoServerGRPCHost(cr),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Server.GRPC.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: nameWithSuffix("server", cr),
									Port: networkingv1.ServiceBackendPort{
										Name: "https",
									},
								},
							},
							PathType: &pathType,
						},
					},
				},
			},
		},
	}

	// Add TLS options
	ingress.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts: []string{
				getArgoServerGRPCHost(cr),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Server.GRPC.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Server.GRPC.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileGrafanaIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ReconcileArgoCD) reconcileGrafanaIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("grafana", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled {
		return nil // Grafana itself or Ingress not enabled, move along...
	}

	// Add default annotations
	atns := make(map[string]string)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Grafana.Ingress.Annotations) > 0 {
		atns = cr.Spec.Grafana.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	ingress.Spec.IngressClassName = cr.Spec.Grafana.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	// Add rules
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getGrafanaHost(cr),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Grafana.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: nameWithSuffix("grafana", cr),
									Port: networkingv1.ServiceBackendPort{
										Name: "http",
									},
								},
							},
							PathType: &pathType,
						},
					},
				},
			},
		},
	}

	// Add TLS options
	ingress.Spec.TLS = []networkingv1.IngressTLS{
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

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// reconcilePrometheusIngress will ensure that the Prometheus Ingress is present.
func (r *ReconcileArgoCD) reconcilePrometheusIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("prometheus", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
		return nil // Prometheus itself or Ingress not enabled, move along...
	}

	// Add default annotations
	atns := make(map[string]string)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Prometheus.Ingress.Annotations) > 0 {
		atns = cr.Spec.Prometheus.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	ingress.Spec.IngressClassName = cr.Spec.Prometheus.Ingress.IngressClassName

	pathType := networkingv1.PathTypeImplementationSpecific
	// Add rules
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getPrometheusHost(cr),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Prometheus.Ingress.Path),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "prometheus-operated",
									Port: networkingv1.ServiceBackendPort{
										Name: "web",
									},
								},
							},
							PathType: &pathType,
						},
					},
				},
			},
		},
	}

	// Add TLS options
	ingress.Spec.TLS = []networkingv1.IngressTLS{
		{
			Hosts:      []string{cr.Name},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Prometheus.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Prometheus.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}

// reconcileApplicationSetControllerIngress will ensure that the ApplicationSetController Ingress is present.
func (r *ReconcileArgoCD) reconcileApplicationSetControllerIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix(common.ApplicationSetServiceNameSuffix, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress) {
		if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Ingress.Enabled {
			return r.Client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Ingress.Enabled {
		log.Info("not enabled")
		return nil // Ingress not enabled, move along...
	}

	// Add annotations
	atns := make(map[string]string)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.ApplicationSet.WebhookServer.Ingress.Annotations) > 0 {
		atns = cr.Spec.ApplicationSet.WebhookServer.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	pathType := networkingv1.PathTypeImplementationSpecific
	// Add rules
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: getApplicationSetHTTPServerHost(cr),
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: "/api/webhook",
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: nameWithSuffix(common.ApplicationSetServiceNameSuffix, cr),
									Port: networkingv1.ServiceBackendPort{
										Name: "webhook",
									},
								},
							},
							PathType: &pathType,
						},
					},
				},
			},
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.ApplicationSet.WebhookServer.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.ApplicationSet.WebhookServer.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ingress)
}
