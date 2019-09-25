package argocd

import (
	"context"

	argoproj "github.com/jmckind/argocd-operator/pkg/apis/argoproj/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// newSecret retuns a new Secret instance.
func newSecret(name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app.kubernetes.io/name":    name,
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Type: corev1.SecretTypeOpaque,
	}
}

func (r *ReconcileArgoCD) reconcileSecrets(cr *argoproj.ArgoCD) error {
	secret := newSecret("argocd-secret")
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: secret.Name}, secret)
	if found {
		// ConfigMap found, do nothing
		return nil
	}
	return r.client.Create(context.TODO(), secret)
}
