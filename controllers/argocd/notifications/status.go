package notifications

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"
)

// reconcileStatus will ensure that the notifications controller status is updated for the given ArgoCD instance
func (nr *NotificationsReconciler) ReconcileStatus() error {

	// TO DO

	return nr.updateInstanceStatus()
}

func (nr *NotificationsReconciler) updateInstanceStatus() error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := nr.Client.Status().Update(context.TODO(), nr.Instance); err != nil {
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
