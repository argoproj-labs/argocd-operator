package applicationset

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"
)

// reconcileStatus will ensure that the appset controller status is updated for the given ArgoCD instance
func (asr *ApplicationSetReconciler) ReconcileStatus() error {

	// TO DO

	return asr.updateInstanceStatus()
}

func (asr *ApplicationSetReconciler) updateInstanceStatus() error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := asr.Client.Status().Update(context.TODO(), asr.Instance); err != nil {
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
