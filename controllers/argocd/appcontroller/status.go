package appcontroller

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"
)

// reconcileStatus will ensure that the app-controller status is updated for the given ArgoCD instance
func (acr *AppControllerReconciler) ReconcileStatus() error {

	// TO DO

	return acr.updateInstanceStatus()
}

func (acr *AppControllerReconciler) updateInstanceStatus() error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := acr.Client.Status().Update(context.TODO(), acr.Instance); err != nil {
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
