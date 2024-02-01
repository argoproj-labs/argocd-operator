package redis

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"
)

// reconcileStatusRedis will ensure that the Redis status is updated for the given ArgoCD instance
func (rr *RedisReconciler) ReconcileStatus() error {

	// TO DO

	return rr.updateInstanceStatus()
}

func (rr *RedisReconciler) updateInstanceStatus() error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := rr.Client.Status().Update(context.TODO(), rr.Instance); err != nil {
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
