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

	argoproj "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj"
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// getDefaultIngressAnnotations will return the default Ingress Annotations for the given ArgoCD.
func getDefaultIngressAnnotations(cr *argoprojv1a1.ArgoCD) map[string]string {
	annotations := make(map[string]string)
	annotations[argoproj.ArgoCDKeyIngressClass] = "nginx"
	return annotations
}

// getIngressAnnotations will retun the Ingress Annotations for the given ArgoCD.
func getIngressAnnotations(cr *argoprojv1a1.ArgoCD) map[string]string {
	atns := getDefaultIngressAnnotations(cr)

	if len(cr.Spec.Ingress.Annotations) > 0 {
		atns = argoutil.AppendStringMap(atns, cr.Spec.Ingress.Annotations)
	}

	return atns
}

// getIngressPath will return the Ingress Path for the given ArgoCD.
func getIngressPath(cr *argoprojv1a1.ArgoCD) string {
	path := argoproj.ArgoCDDefaultIngressPath
	if len(cr.Spec.Ingress.Path) > 0 {
		path = cr.Spec.Ingress.Path
	}
	return path
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
	lbls[argoproj.ArgoCDKeyName] = name
	ingress.ObjectMeta.Labels = lbls

	return ingress
}

// newIngressWithSuffix returns a new Ingress with the given name suffix for the ArgoCD.
func newIngressWithSuffix(suffix string, cr *argoprojv1a1.ArgoCD) *extv1beta1.Ingress {
	return newIngressWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), cr)
}

// reconcileIngresses will ensure that all ArgoCD Ingress resources are present.
func (r *ReconcileArgoCD) reconcileIngresses(cr *argoprojv1a1.ArgoCD) error {
	if !cr.Spec.Ingress.Enabled {
		return nil // Ingress not enabled, do nothing.
	}

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
	ingress := newIngress(cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, ingress.Name, ingress) {
		if !cr.Spec.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	// Add annotations
	atns := getDefaultIngressAnnotations(cr)
	atns[argoproj.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[argoproj.ArgoCDKeyIngressBackendProtocol] = "HTTP"
	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getArgoServerHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: getIngressPath(cr),
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

	// Add TLS options
	ingress.Spec.TLS = []extv1beta1.IngressTLS{
		{
			Hosts:      []string{cr.Name},
			SecretName: argoproj.ArgoCDSecretName,
		},
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
		if !cr.Spec.Ingress.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	// Add annotations
	atns := getDefaultIngressAnnotations(cr)
	atns[argoproj.ArgoCDKeyIngressBackendProtocol] = "GRPC"
	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getArgoServerGRPCHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: getIngressPath(cr),
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
			Hosts:      []string{cr.Name},
			SecretName: argoproj.ArgoCDSecretName,
		},
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
		if !cr.Spec.Ingress.Enabled || !cr.Spec.Grafana.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Grafana.Enabled {
		return nil // Grafana not enabled, do nothing.
	}

	// Add annotations
	atns := getDefaultIngressAnnotations(cr)
	atns[argoproj.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[argoproj.ArgoCDKeyIngressBackendProtocol] = "HTTP"
	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getGrafanaHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: getIngressPath(cr),
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
			Hosts:      []string{cr.Name},
			SecretName: argoproj.ArgoCDSecretName,
		},
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
		if !cr.Spec.Ingress.Enabled || !cr.Spec.Prometheus.Enabled {
			// Ingress exists but enabled flag has been set to false, delete the Ingress
			return r.client.Delete(context.TODO(), ingress)
		}
		return nil // Ingress found and enabled, do nothing
	}

	if !cr.Spec.Prometheus.Enabled {
		return nil // Prometheus not enabled, do nothing.
	}

	// Add annotations
	atns := getDefaultIngressAnnotations(cr)
	atns[argoproj.ArgoCDKeyIngressSSLRedirect] = "true"
	atns[argoproj.ArgoCDKeyIngressBackendProtocol] = "HTTP"
	ingress.ObjectMeta.Annotations = atns

	// Add rules
	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getPrometheusHost(cr),
			IngressRuleValue: extv1beta1.IngressRuleValue{
				HTTP: &extv1beta1.HTTPIngressRuleValue{
					Paths: []extv1beta1.HTTPIngressPath{
						{
							Path: getIngressPath(cr),
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
			SecretName: argoproj.ArgoCDSecretName,
		},
	}

	if err := controllerutil.SetControllerReference(cr, ingress, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), ingress)
}
