package server

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"
)

// reconcileStatus will ensure that the server status is updated for the given ArgoCD instance
func (sr *ServerReconciler) ReconcileStatus() error {

	// TO DO

	return sr.updateInstanceStatus()
}

func (sr *ServerReconciler) updateInstanceStatus() error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := sr.Client.Status().Update(context.TODO(), sr.Instance); err != nil {
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
