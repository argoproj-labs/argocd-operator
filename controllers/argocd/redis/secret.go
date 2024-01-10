package redis

import (
	"crypto/sha256"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	corev1 "k8s.io/api/core/v1"
)

// reconcileTLSSecret checks whether the argocd-operator-redis-tls secret
// has changed since our last reconciliation loop. It does so by comparing the
// checksum of tls.crt and tls.key in the status of the ArgoCD CR against the
// values calculated from the live state in the cluster.
func (rr *RedisReconciler) reconcileTLSSecret() error {
	var sha256sum string

	tlsSecret, err := workloads.GetSecret(common.ArgoCDRedisServerTLSSecretName, rr.Instance.Namespace, rr.Client)
	if err != nil {
		rr.Logger.Error(err, "reconcileTLSSecret: failed to retrieve secret", "name", common.ArgoCDRedisServerTLSSecretName, "namespace", rr.Instance.Namespace)
		return err
	}

	if tlsSecret.Type != corev1.SecretTypeTLS {
		// We only process secrets of type kubernetes.io/tls
		rr.Logger.V(1).Info("reconcileTLSSecret: skipping secret of type other than kubernetes.io/tls")
		return nil
	}

	// We do the checksum over a concatenated byte stream of cert + key
	crt, crtOk := tlsSecret.Data[corev1.TLSCertKey]
	key, keyOk := tlsSecret.Data[corev1.TLSPrivateKeyKey]
	if crtOk && keyOk {
		var sumBytes []byte
		sumBytes = append(sumBytes, crt...)
		sumBytes = append(sumBytes, key...)
		sha256sum = fmt.Sprintf("%x", sha256.Sum256(sumBytes))
	}

	// The content of the TLS secret has changed since we last looked if the
	// calculated checksum doesn't match the one stored in the status.
	if rr.Instance.Status.RedisTLSChecksum != sha256sum {
		// We store the value early to prevent a possible restart loop, for the
		// cost of a possibly missed restart when we cannot update the status
		// field of the resource.
		rr.Instance.Status.RedisTLSChecksum = sha256sum
		err = rr.UpdateInstanceStatus()
		if err != nil {
			rr.Logger.Error(err, "reconcileTLSSecret: failed to update instance status")
			return err
		}

		// trigger redis rollout
		err := rr.TriggerRollout()

		// trigger server rollout
		err = rr.Server.TriggerRollout()

		// trigger repo-server rollout
		err = rr.RepoServer.TriggerRollout()

		// trigger app-controller rollout
		err = rr.Appcontroller.TriggerRollout()

	}
	return nil
}
