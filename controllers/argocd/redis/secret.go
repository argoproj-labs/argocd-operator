package redis

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
)

// reconcileTLSSecret checks whether the argocd-operator-redis-tls secret
// has changed since our last reconciliation loop. It does so by comparing the
// checksum of tls.crt and tls.key in the status of the ArgoCD CR against the
// values calculated from the live state in the cluster.
func (rr *RedisReconciler) reconcileTLSSecret() []error {
	var reconErrs []error
	var sha256sum string

	sha256sum, err := argocdcommon.TLSSecretChecksum(types.NamespacedName{Name: common.ArgoCDRedisServerTLSSecretName, Namespace: rr.Instance.Namespace}, rr.Client)
	if err != nil {
		reconErrs = append(reconErrs, errors.Wrapf(err, "reconcileTLSSecret: failed to calculate checksum for %s in namespace %s", common.ArgoCDRedisServerTLSSecretName, rr.Instance.Namespace))
		return reconErrs
	}

	if sha256sum == "" {
		rr.Logger.V(1).Info("reconcileTLSSecret: received empty checksum; secret of type other than kubernetes.io/tls encountered")
		return reconErrs
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
			reconErrs = append(reconErrs, errors.Wrapf(err, "reconcileTLSSecret: failed to update instance status"))
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
