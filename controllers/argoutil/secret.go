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

package argoutil

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

// FetchSecret will retrieve the object with the given Name using the provided client.
// The result will be returned.
func FetchSecret(client client.Client, meta metav1.ObjectMeta, name string) (*corev1.Secret, error) {
	a := &argoproj.ArgoCD{}
	a.ObjectMeta = meta
	secret := NewSecretWithName(a, name)
	return secret, FetchObject(client, meta.Namespace, name, secret)
}

// NewTLSSecret returns a new TLS Secret based on the given metadata with the provided suffix on the Name.
func NewTLSSecret(cr *argoproj.ArgoCD, suffix string) *corev1.Secret {
	secret := NewSecretWithSuffix(cr, suffix)
	secret.Type = corev1.SecretTypeTLS
	return secret
}

// NewSecret returns a new Secret based on the given metadata.
func NewSecret(cr *argoproj.ArgoCD) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: LabelsForCluster(cr),
		},
		Type: corev1.SecretTypeOpaque,
	}
	AddTrackedByOperatorLabel(&secret.ObjectMeta)
	return secret
}

// NewSecretWithName returns a new Secret based on the given metadata with the provided Name.
func NewSecretWithName(cr *argoproj.ArgoCD, name string) *corev1.Secret {
	secret := NewSecret(cr)

	secret.Name = name
	secret.Namespace = cr.Namespace
	// Truncate the name for labels to stay within 63 character limit
	secret.Labels[common.ArgoCDKeyName] = TruncateWithHash(name)

	return secret
}

// NewSecretWithSuffix returns a new Secret based on the given metadata with the provided suffix on the Name.
func NewSecretWithSuffix(cr *argoproj.ArgoCD, suffix string) *corev1.Secret {
	return NewSecretWithName(cr, fmt.Sprintf("%s-%s", cr.Name, suffix))
}

// GetSecretNameWithSuffix returns the truncated secret name for the given suffix.
// This function should be used when referencing secret names in other resources.
func GetSecretNameWithSuffix(cr *argoproj.ArgoCD, suffix string) string {
	fullName := fmt.Sprintf("%s-%s", cr.Name, suffix)
	return TruncateWithHash(fullName)
}

func CreateTLSSecret(client client.Client, name string, namespace string, data map[string][]byte) error {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: data,
	}
	AddTrackedByOperatorLabel(&secret.ObjectMeta)
	LogResourceCreation(log, &secret)
	return client.Create(context.TODO(), &secret)
}

func CreateSecret(client client.Client, name string, namespace string, data map[string][]byte) error {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	AddTrackedByOperatorLabel(&secret.ObjectMeta)
	LogResourceCreation(log, &secret)
	return client.Create(context.TODO(), &secret)
}
