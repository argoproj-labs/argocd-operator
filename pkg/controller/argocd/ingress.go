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

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// getDefaultIngressAnnotations will return the default Ingress Annotations for the given ArgoCD.
func getDefaultIngressAnnotations(cr *argoproj.ArgoCD) map[string]string {
	annotations := make(map[string]string)
	annotations[ArgoCDKeyIngressClass] = "nginx"
	annotations[ArgoCDKeyIngressSSLRedirect] = "true"
	annotations[ArgoCDKeyIngressSSLPassthrough] = "true"
	return annotations
}

// getIngressAnnotations will retun the Ingress Annotations for the given ArgoCD.
func getIngressAnnotations(cr *argoproj.ArgoCD) map[string]string {
	if len(cr.Spec.Ingress.Annotations) > 0 {
		return cr.Spec.Ingress.Annotations
	}
	return getDefaultIngressAnnotations(cr)
}

// getIngressHost will retun the Ingress host for the given ArgoCD.
func getIngressHost(cr *argoproj.ArgoCD) string {
	host := cr.Name
	if len(cr.Spec.Ingress.Host) > 0 {
		host = cr.Spec.Ingress.Host
	}
	return host
}

// getIngressPath will return the Ingress Path for the given ArgoCD.
func getIngressPath(cr *argoproj.ArgoCD) string {
	path := ArgoCDDefaultIngressPath
	if len(cr.Spec.Ingress.Path) > 0 {
		path = cr.Spec.Ingress.Path
	}
	return path
}

// newIngress returns a new Ingress instance for the given ArgoCD.
func newIngress(cr *argoproj.ArgoCD) *extv1beta1.Ingress {
	return &extv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

// newIngressWithName returns a new Ingress with the given name and ArgoCD.
func newIngressWithName(name string, cr *argoproj.ArgoCD) *extv1beta1.Ingress {
	ingress := newIngress(cr)
	ingress.ObjectMeta.Name = name

	lbls := ingress.ObjectMeta.Labels
	lbls[ArgoCDKeyName] = name
	ingress.ObjectMeta.Labels = lbls

	return ingress
}

// newIngressWithSuffix returns a new Ingress with the given name suffix for the ArgoCD.
func newIngressWithSuffix(suffix string, cr *argoproj.ArgoCD) *extv1beta1.Ingress {
	return newIngressWithName(fmt.Sprintf("%s-%s", cr.Name, suffix), cr)
}

// reconcileIngresses will ensure that all ArgoCD Ingress resources are present.
func (r *ReconcileArgoCD) reconcileIngresses(cr *argoproj.ArgoCD) error {
	if !cr.Spec.Ingress.Enabled {
		return nil // Ingress not enabled, do nothing.
	}

	if err := r.reconcileServerIngress(cr); err != nil {
		return err
	}
	return nil
}

// reconcileServerIngress will ensure that the ArgoCD Server Ingress is present.
func (r *ReconcileArgoCD) reconcileServerIngress(cr *argoproj.ArgoCD) error {
	ingress := newIngressWithSuffix("server", cr)
	if r.isObjectFound(cr.Namespace, ingress.Name, ingress) {
		return nil // Ingress found, do nothing
	}

	ingress.ObjectMeta.Annotations = getIngressAnnotations(cr)

	ingress.Spec.Rules = []extv1beta1.IngressRule{
		{
			Host: getIngressHost(cr),
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

	if err := controllerutil.SetControllerReference(cr, ingress, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), ingress)
}
