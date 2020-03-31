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
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FetchSecret will retrieve the object with the given Name using the provided client.
// The result will be returned.
func FetchSecret(client client.Client, meta metav1.ObjectMeta, name string) (*corev1.Secret, error) {
	secret := NewSecretWithName(meta, name)
	return secret, FetchObject(client, meta.Namespace, name, secret)
}

// NewTLSSecret returns a new TLS Secret based on the given metadata with the provided suffix on the Name.
func NewTLSSecret(meta metav1.ObjectMeta, suffix string) *corev1.Secret {
	secret := NewSecretWithSuffix(meta, suffix)
	secret.Type = corev1.SecretTypeTLS
	return secret
}

// NewSecret returns a new Secret based on the given metadata.
func NewSecret(meta metav1.ObjectMeta) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: meta.Namespace,
			Labels:    AppendStringMap(DefaultLabels(meta.Name), meta.Labels),
		},
		Type: corev1.SecretTypeOpaque,
	}
}

// NewSecretWithName returns a new Secret based on the given metadata with the provided Name.
func NewSecretWithName(meta metav1.ObjectMeta, name string) *corev1.Secret {
	secret := NewSecret(meta)

	secret.ObjectMeta.Name = name
	secret.ObjectMeta.Labels[common.ArgoCDKeyName] = name

	return secret
}

// NewSecretWithSuffix returns a new Secret based on the given metadata with the provided suffix on the Name.
func NewSecretWithSuffix(meta metav1.ObjectMeta, suffix string) *corev1.Secret {
	return NewSecretWithName(meta, fmt.Sprintf("%s-%s", meta.Name, suffix))
}
