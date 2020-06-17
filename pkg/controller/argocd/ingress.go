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

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// getDefaultIngressAnnotations will return the default Ingress Annotations for the given ArgoCD.
func getDefaultIngressAnnotations(cr *argoprojv1a1.ArgoCD) map[string]string {
	annotations := make(map[string]string)
	annotations[common.ArgoCDKeyIngressClass] = "nginx"
	return annotations
}

// getArgoServerPath will return the Ingress Path for the Argo CD component.
func getPathOrDefault(path string) string {
	result := common.ArgoCDDefaultIngressPath
	if len(path) > 0 {
		result = path
	}
	return result
}

// newIngress returns a new Ingress instance for the given ArgoCD.
func newIngress(cr *argoprojv1a1.ArgoCD) *extv1beta1.Ingress {
	return &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newIngressWithName returns a new Ingress with the given name and ArgoCD.
func newIngressWithName(name string, cr *argoprojv1a1.ArgoCD) *extv1beta1.Ingress {
	ingress := newIngress(cr)
	ingress.ObjectMeta.Name = name

	lbls := ingress.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	ingress.ObjectMeta.Labels = lbls

	return ingress
}

// newIngressWithSuffix returns a new Ingress with the given name suffix for the ArgoCD.
func newIngressWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) *extv1beta1.Ingress {
	return newIngressWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), cr)
}

// reconcileIngresses will ensure that all ArgoCD Ingress resources are present.
func (r *ReconcileArgoCD) reconcileIngresses(cr *argoprojv1a1.ArgoCD) error {
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
	return nil
}

// reconcileArgoServerIngress will ensure that the ArgoCD Server Ingress is present.
func (r *ReconcileArgoCD) reconcileArgoServerIngress(cr *argoprojv1a1.ArgoCD) error {
	ingress := newIngressWithSuffix("server", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Server.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Server.Ingress.Enabled {
		return nil // Ingress not enabled, move along...
	}

	// Add annotations
	atns := getDefaultIngressAnnotations(cr)
	atns[common.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[common.ArgoCDKeyIngressBackendProtocol] = "HTTP"

	// Override default annotations if specified
	if len(cr.Spec.Server.Ingress.Annotations) > 0 {
		atns = cr.Spec.Server.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getArgoServerHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Server.Ingress.Path),
							Backend: extv1beta1.IngressBackend{
								ServiceName: nameWithSuffix("server", cr),
								ServicePort: intstr.FromString("http"),
							},
						},
					},
				},
			},
		},
	}

	// Add default TLS options
	ingress.Spec.TLS = []extv1beta1.IngressTLS{
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

	if err := controllerutil.SetControllerReference(cr, ingress, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), ingress)
}

// reconcileArgoServerGRPCIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ReconcileArgoCD) reconcileArgoServerGRPCIngress(cr *argoprojv1a1.ArgoCD) error {
	ingress := newIngressWithSuffix("grpc", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Server.GRPC.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Server.GRPC.Ingress.Enabled {
		return nil // Ingress not enabled, move along...
	}

	// Add annotations
	atns := getDefaultIngressAnnotations(cr)
	atns[common.ArgoCDKeyIngressBackendProtocol] = "GRPC"

	// Override default annotations if specified
	if len(cr.Spec.Server.GRPC.Ingress.Annotations) > 0 {
		atns = cr.Spec.Server.GRPC.Ingress.Annotations
	}

	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getArgoServerGRPCHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: getPathOrDefault(cr.Spec.Server.GRPC.Ingress.Path),
							Backend: extv1beta1.IngressBackend{
								ServiceName: nameWithSuffix("server", cr),
								ServicePort: intstr.FromString("https"),
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
				getArgoServerGRPCHost(cr),
			},
			SecretName: common.ArgoCDSecretName,
		},
	}

	// Allow override of TLS options if specified
	if len(cr.Spec.Server.GRPC.Ingress.TLS) > 0 {
		ingress.Spec.TLS = cr.Spec.Server.GRPC.Ingress.TLS
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), ingress)
}

// reconcileGrafanaIngress will ensure that the ArgoCD Server GRPC Ingress is present.
func (r *ReconcileArgoCD) reconcileGrafanaIngress(cr *argoprojv1a1.ArgoCD) error {
	ingress := newIngressWithSuffix("grafana", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Grafana.Enabled || !cr.Spec.Grafana.Ingress.Enabled {
		return nil // Grafana itself or Ingress not enabled, move along...
	}

	// Add annotations
	atns := getDefaultIngressAnnotations(cr)
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
								ServiceName: nameWithSuffix("grafana", cr),
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

	if err := controllerutil.SetControllerReference(cr, ingress, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), ingress)
}

// reconcilePrometheusIngress will ensure that the Prometheus Ingress is present.
func (r *ReconcileArgoCD) reconcilePrometheusIngress(cr *argoprojv1a1.ArgoCD) error {
	ingress := newIngressWithSuffix("prometheus", cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Prometheus.Enabled || !cr.Spec.Prometheus.Ingress.Enabled {
		return nil // Prometheus itself or Ingress not enabled, move along...
	}

	// Add annotations
	atns := getDefaultIngressAnnotations(cr)
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

	if err := controllerutil.SetControllerReference(cr, ingress, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), ingress)
}
