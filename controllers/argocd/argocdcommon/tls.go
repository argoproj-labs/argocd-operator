package argocdcommon

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UseTLS, on being invoked by a component, looks for a specified TLS secret on the cluster. If this secret is found, and is owned (either directly or indirectly) by an Argo CD instance, UseTLS returns true. In all other cases it returns false
func UseTLS(secretName, secretNs string, client client.Client, logger *util.Logger) bool {
	tlsSecret, err := workloads.GetSecret(secretName, secretNs, client)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Debug("TLS secret not found; skipping TLS enforcement")
			return false
		}
		logger.Error(err, "UseTLS: failed to retrieve tls secret", "name", secretName, "namespace", secretNs)
		return false
	}

	if tlsSecret.Type != corev1.SecretTypeTLS {
		// We only process secrets of type kubernetes.io/tls
		logger.Debug("secret is not of type kubernetes.io/tls ; skipping TLS enforcement", "name", tlsSecret.Name, "namespace", tlsSecret.Namespace)
		return false
	}

	secretOwner, err := FindSecretOwnerInstance(types.NamespacedName{Name: tlsSecret.Name, Namespace: tlsSecret.Namespace}, client)
	if err != nil {
		logger.Error(err, "UseTLS: failed to find tls secret owner", "name", tlsSecret.Name, "namespace", tlsSecret.Namespace)
		return false
	}

	if !reflect.DeepEqual(secretOwner, types.NamespacedName{}) {
		return true
	}

	logger.Debug("no owner instance found for secret ; skipping TLS enforcement", "name", tlsSecret.Name, "namespace", tlsSecret.Namespace)
	return false
}
