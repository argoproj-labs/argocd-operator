package reposerver

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"
)

// reconcileStatus will ensure that the Repo-server status is updated for the given ArgoCD instance
func (rsr *RepoServerReconciler) ReconcileStatus() error {

	// TO DO

	return rsr.updateInstanceStatus()
}

func (rsr *RepoServerReconciler) updateInstanceStatus() error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := rsr.Client.Status().Update(context.TODO(), rsr.Instance); err != nil {
			return errors.Wrap(err, "UpdateInstanceStatus: failed to update instance status")
		}
		return nil
	})

	if err != nil {
		// May be conflict if max retries were hit, or may be something unrelated
		// like permissions or a network error
		return err
	}
	return nil
}
