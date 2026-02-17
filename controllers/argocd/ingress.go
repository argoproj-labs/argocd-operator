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
	"reflect"
	"strings"

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
	ingress.Name = name

	lbls := ingress.Labels
	lbls[common.ArgoCDKeyName] = name
	ingress.Labels = lbls

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
	existingIngress := newIngressWithSuffix("server", cr)

	objectFound, err := argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, existingIngress)
	if err != nil {
		return err
	}

	if !cr.Spec.Server.Ingress.Enabled {
		if objectFound {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			argoutil.LogResourceDeletion(log, ingress, "server ingress is disabled")
			return r.Delete(context.TODO(), ingress)
		}
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

	ingress.Annotations = atns

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
	if objectFound {
		var changes []string
		// If Ingress found and enabled, make sure the ingressClassName is up-to-date
		if existingIngress.Spec.IngressClassName != cr.Spec.Server.Ingress.IngressClassName {
			existingIngress.Spec.IngressClassName = cr.Spec.Server.Ingress.IngressClassName
			changes = append(changes, "ingress class name")
		}
		if !reflect.DeepEqual(cr.Spec.Server.Ingress.Annotations, existingIngress.Annotations) {
			existingIngress.Annotations = cr.Spec.Server.Ingress.Annotations
			changes = append(changes, "annotations")
		}
		if !reflect.DeepEqual(ingress.Spec.Rules, existingIngress.Spec.Rules) {
			existingIngress.Spec.Rules = ingress.Spec.Rules
			changes = append(changes, "ingress rules")
		}
		if !reflect.DeepEqual(ingress.Spec.TLS, existingIngress.Spec.TLS) {
			existingIngress.Spec.TLS = ingress.Spec.TLS
			changes = append(changes, "ingress tls")
		}
		if len(changes) > 0 {
			argoutil.LogResourceUpdate(log, existingIngress, "updating", strings.Join(changes, ", "))
			return r.Update(context.TODO(), existingIngress)
		}
		return nil // Ingress with no changes to apply, do nothing
	}
	if err := controllerutil.SetControllerReference(cr, ingress, r.Scheme); err != nil {
		return err
	}
	argoutil.LogResourceCreation(log, ingress)
	return r.Create(context.TODO(), ingress)
}

// reconcileArgoServerGRPCIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ReconcileArgoCD) reconcileArgoServerGRPCIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("grpc", cr)

	ingressExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress)
	if err != nil {
		return err
	}
	if ingressExists {
		if !cr.Spec.Server.GRPC.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			argoutil.LogResourceDeletion(log, ingress, "server grpc ingress is disabled")
			return r.Delete(context.TODO(), ingress)
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

	ingress.Annotations = atns

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
	argoutil.LogResourceCreation(log, ingress)
	return r.Create(context.TODO(), ingress)
}

// reconcileGrafanaIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ReconcileArgoCD) reconcileGrafanaIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("grafana", cr)
	ingressExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress)
	if err != nil {
		return err
	}
	if ingressExists {
		//lint:ignore SA1019 known to be deprecated
		if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled { //nolint:staticcheck // SA1019: We must test deprecated fields.
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			var explanation string
			//lint:ignore SA1019 known to be deprecated
			if !cr.Spec.Grafana.Enabled { //nolint:staticcheck // SA1019: We must test deprecated fields.
				explanation = "grafana is disabled"
			} else {
				explanation = "grafana ingress is disabled"
			}
			argoutil.LogResourceDeletion(log, ingress, explanation)
			return r.Delete(context.TODO(), ingress)
		}
		log.Info(grafanaDeprecatedWarning)
		return nil // Ingress found and enabled, do nothing
	}

	//lint:ignore SA1019 known to be deprecated
	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled { //nolint:staticcheck // SA1019: We must test deprecated fields.
		return nil // Grafana itself or Ingress not enabled, move along...
	}

	log.Info(grafanaDeprecatedWarning)

	return nil
}

// reconcilePrometheusIngress will ensure that the Prometheus Ingress is present.
func (r *ReconcileArgoCD) reconcilePrometheusIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("prometheus", cr)
	ingressExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress)
	if err != nil {
		return err
	}
	if ingressExists {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			var explanation string
			if !cr.Spec.Prometheus.Enabled {
				explanation = "prometheus is disabled"
			} else {
				explanation = "prometheus ingress is disabled"
			}
			argoutil.LogResourceDeletion(log, ingress, explanation)
			return r.Delete(context.TODO(), ingress)
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

	ingress.Annotations = atns

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
	argoutil.LogResourceCreation(log, ingress)
	return r.Create(context.TODO(), ingress)
}

// reconcileApplicationSetControllerIngress will ensure that the ApplicationSetController Ingress is present.
func (r *ReconcileArgoCD) reconcileApplicationSetControllerIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix(common.ApplicationSetServiceNameSuffix, cr)
	ingressExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, ingress.Name, ingress)
	if err != nil {
		return err
	}
	if ingressExists {
		if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Ingress.Enabled {
			var explanation string
			if cr.Spec.ApplicationSet == nil {
				explanation = "applicationset is disabled"
			} else {
				explanation = "applicationset webhook ingress is disabled"
			}
			argoutil.LogResourceDeletion(log, ingress, explanation)
			return r.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if cr.Spec.ApplicationSet == nil || !cr.Spec.ApplicationSet.WebhookServer.Ingress.Enabled {
		log.Info("applicationset or applicationset webhook ingress disabled")
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

	ingress.Annotations = atns

	pathType := networkingv1.PathTypeImplementationSpecific
	httpServerHost, err := getApplicationSetHTTPServerHost(cr)
	if err != nil {
		return err
	}

	// Add rules
	ingress.Spec.Rules = []networkingv1.IngressRule{
		{
			Host: httpServerHost,
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
	argoutil.LogResourceCreation(log, ingress)
	return r.Create(context.TODO(), ingress)
}
