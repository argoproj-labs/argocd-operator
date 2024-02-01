package redis

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// reconcileTLSSecret checks whether the argocd-operator-redis-tls secret
// has changed since our last reconciliation loop. It does so by comparing the
// checksum of tls.crt and tls.key in the status of the ArgoCD CR against the
// values calculated from the live state in the cluster.
func (rr *RedisReconciler) reconcileTLSSecret() error {
	var reconErrs util.MultiError
	var sha256sum string

	if !rr.TLSEnabled {
		rr.Logger.Debug("reconcileTLSSecret: TLS disabled; skipping TLS secret reconciliation")
		return nil
	}

	sha256sum, err := argocdcommon.TLSSecretChecksum(types.NamespacedName{Name: common.ArgoCDRedisServerTLSSecretName, Namespace: rr.Instance.Namespace}, rr.Client)
	if err != nil {
		reconErrs.Append(errors.Wrapf(err, "reconcileTLSSecret: failed to calculate checksum for %s in namespace %s", common.ArgoCDRedisServerTLSSecretName, rr.Instance.Namespace))
		return reconErrs
	}

	if sha256sum == "" {
		rr.Logger.Debug("reconcileTLSSecret: received empty checksum; secret either not found, or is of type other than kubernetes.io/tls")
		return nil
	}

	// The content of the TLS secret has changed since we last looked if the
	// calculated checksum doesn't match the one stored in the status.
	if rr.Instance.Status.RedisTLSChecksum != sha256sum {
		// We store the value early to prevent a possible restart loop, for the
		// cost of a possibly missed restart when we cannot update the status
		// field of the resource.
		rr.Instance.Status.RedisTLSChecksum = sha256sum
		err = rr.updateInstanceStatus()
		if err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileTLSSecret"))
		}

		// trigger redis rollout
		if err := rr.TriggerRollout(common.RedisTLSCertChangedKey); err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileTLSSecret"))
		}

		// trigger server rollout
		if err := rr.Server.TriggerRollout(common.RedisTLSCertChangedKey); err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileTLSSecret"))
		}

		// trigger repo-server rollout
		if err := rr.RepoServer.TriggerRollout(common.RedisTLSCertChangedKey); err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileTLSSecret"))
		}

		// trigger app-controller rollout
		if err := rr.Appcontroller.TriggerRollout(common.RedisTLSCertChangedKey); err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileTLSSecret"))
		}
	}
	return reconErrs.ErrOrNil()
}

func (rr *RedisReconciler) deleteSecret(name, namespace string) error {
	if err := workloads.DeleteSecret(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteSecret: failed to delete secret %s", name)
	}
	rr.Logger.Info("secret deleted", "name", name, "namespace", namespace)
	return nil
}
