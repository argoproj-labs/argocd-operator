package resource

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func UpdateStatusSubResource(instance client.Object, cl client.Client) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := cl.Status().Update(context.TODO(), instance); err != nil {
			return errors.Wrap(err, "UpdateInstanceStatus: failed to update instance status")
		}
		return nil
	})

	// May be conflict if max retries were hit, or may be something unrelated
	// like permissions or a network error
	return err
}
