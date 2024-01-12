package redis

import (
	"crypto/sha256"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

// reconcileTLSSecret checks whether the argocd-operator-redis-tls secret
// has changed since our last reconciliation loop. It does so by comparing the
// checksum of tls.crt and tls.key in the status of the ArgoCD CR against the
// values calculated from the live state in the cluster.
func (rr *RedisReconciler) reconcileTLSSecret() []error {
	var reconErrs []error
	var sha256sum string

	tlsSecret, err := workloads.GetSecret(common.ArgoCDRedisServerTLSSecretName, rr.Instance.Namespace, rr.Client)
	if err != nil {
		reconErrs = append(reconErrs, errors.Wrap(err, fmt.Sprintf("reconcileTLSSecret: failed to retrieve secret %s in namespace %s", common.ArgoCDRedisServerTLSSecretName, rr.Instance.Namespace)))
		return reconErrs
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
			reconErrs = append(reconErrs, errors.Wrap(err, "reconcileTLSSecret: failed to update instance status"))
			return reconErrs
		}

		// trigger redis rollout
		if err := rr.TriggerRollout(common.RedisTLSCertChangedKey); err != nil {
			reconErrs = append(reconErrs, err)
		}

		// trigger server rollout
		if err := rr.Server.TriggerRollout(common.RedisTLSCertChangedKey); err != nil {
			reconErrs = append(reconErrs, err)
		}

		// trigger repo-server rollout
		if err := rr.RepoServer.TriggerRollout(common.RedisTLSCertChangedKey); err != nil {
			reconErrs = append(reconErrs, err)
		}

		// trigger app-controller rollout
		if err := rr.Appcontroller.TriggerRollout(common.RedisTLSCertChangedKey); err != nil {
			reconErrs = append(reconErrs, err)
		}

	}
	return reconErrs
}
