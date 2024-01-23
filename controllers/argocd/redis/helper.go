package redis

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// UseTLS decides whether Redis component should communicate with TLS or not
func (rr *RedisReconciler) UseTLS() bool {
	tlsSecret, err := workloads.GetSecret(common.ArgoCDRedisServerTLSSecretName, rr.Instance.Namespace, rr.Client)
	if err != nil {
		if apierrors.IsNotFound(err) {
			rr.Logger.V(1).Info("skipping TLS enforcement")
			return false
		}
		rr.Logger.Error(err, "UseTLS: failed to retrieve tls secret", "name", common.ArgoCDRedisServerTLSSecretName, "namespace", rr.Instance.Namespace)
		return false
	}

	secretOwner, err := argocdcommon.FindSecretOwnerInstance(types.NamespacedName{Name: tlsSecret.Name, Namespace: tlsSecret.Namespace}, rr.Client)
	if err != nil {
		rr.Logger.Error(err, "UseTLS: failed to find secret owning instance")
		return false
	}

	if !reflect.DeepEqual(secretOwner, types.NamespacedName{}) {
		return true
	}

	return false
}

// GetServerAddress will return the Redis service address for the given ArgoCD instance
func (rr *RedisReconciler) GetServerAddress() string {
	return ""
}
