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

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// newSecret retuns a new Secret instance.
func newSecret(name string, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":    name,
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func (r *ReconcileArgoCD) reconcileArgoSecret(cr *argoproj.ArgoCD) error {
	secret := newSecret("argocd-secret", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: secret.Name}, secret)
	if found {
		return nil // Secret found, do nothing
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), secret)
}

func (r *ReconcileArgoCD) reconcileSecrets(cr *argoproj.ArgoCD) error {
	if err := r.reconcileArgoSecret(cr); err != nil {
		return err
	}

	if IsOpenShift() {
		if err := r.reconcileGrafanaSecret(cr); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileArgoCD) reconcileGrafanaSecret(cr *argoproj.ArgoCD) error {
	secret := newSecret("argocd-grafana-secret", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: secret.Name}, secret)
	if found {
		return nil // Secret found, do nothing
	}

	secret.Data = map[string][]byte{
		"admin": []byte("secret"),
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), secret)
}
