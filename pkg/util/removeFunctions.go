package util

import (
	"fmt"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FetchSecret will retrieve the object with the given Name using the provided client.
// The result will be returned.
func FetchSecret(client client.Client, meta metav1.ObjectMeta, name string) (*corev1.Secret, error) {
	a := &argoproj.ArgoCD{}
	a.ObjectMeta = meta
	secret := NewSecretWithName(a, name)
	return secret, FetchObject(client, meta.Namespace, name, secret)
}

// NewSecret returns a new Secret based on the given metadata.
func NewSecret(cr *argoproj.ArgoCD) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: argoutil.LabelsForCluster(cr),
		},
		Type: corev1.SecretTypeOpaque,
	}
}

// NewTLSSecret returns a new TLS Secret based on the given metadata with the provided suffix on the Name.
func NewTLSSecret(cr *argoproj.ArgoCD, suffix string) *corev1.Secret {
	secret := NewSecretWithSuffix(cr, suffix)
	secret.Type = corev1.SecretTypeTLS
	return secret
}

// NewSecretWithName returns a new Secret based on the given metadata with the provided Name.
func NewSecretWithName(cr *argoproj.ArgoCD, name string) *corev1.Secret {
	secret := NewSecret(cr)

	secret.ObjectMeta.Name = name
	secret.ObjectMeta.Namespace = cr.Namespace
	secret.ObjectMeta.Labels[common.AppK8sKeyName] = name

	return secret
}

// NewSecretWithSuffix returns a new Secret based on the given metadata with the provided suffix on the Name.
func NewSecretWithSuffix(cr *argoproj.ArgoCD, suffix string) *corev1.Secret {
	return NewSecretWithName(cr, fmt.Sprintf("%s-%s", cr.Name, suffix))
}
