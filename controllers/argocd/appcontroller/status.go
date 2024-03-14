package appcontroller

import (
	"context"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"
)

// reconcileStatus will ensure that the app-controller status is updated for the given ArgoCD instance
func (acr *AppControllerReconciler) ReconcileStatus() error {
	status := common.ArgoCDStatusUnknown

	ss, err := workloads.GetStatefulSet(resourceName, acr.Instance.Namespace, acr.Client)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve statefulset %s", resourceName)
	}

	status = common.ArgoCDStatusPending

	if ss.Spec.Replicas != nil {
		if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
			status = common.ArgoCDStatusRunning
		}
	}

	if acr.Instance.Status.Redis != status {
		acr.Instance.Status.Redis = status
	}

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
