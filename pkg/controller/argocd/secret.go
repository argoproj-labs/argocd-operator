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

func (r *ReconcileArgoCD) reconcileSecrets(cr *argoproj.ArgoCD) error {
	secret := newSecret("argocd-secret", cr.Namespace)
	found := r.isObjectFound(types.NamespacedName{Namespace: cr.Namespace, Name: secret.Name}, secret)
	if found {
		// ConfigMap found, do nothing
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), secret)
}
