package applicationset

import (
	"context"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// reconcileStatus will ensure that the appset controller status is updated for the given ArgoCD instance
func (asr *ApplicationSetReconciler) ReconcileStatus() error {
	status := common.ArgoCDStatusUnknown

	if asr.Instance.Spec.ApplicationSet.IsEnabled() {
		d, err := workloads.GetDeployment(resourceName, asr.Instance.Namespace, asr.Client)
		if err != nil {
			return errors.Wrapf(err, "failed to retrieve deployment %s", resourceName)
		}

		status = common.ArgoCDStatusPending

		if d.Spec.Replicas != nil {
			if d.Status.ReadyReplicas == *d.Spec.Replicas {
				status = common.ArgoCDStatusRunning
			} else if d.Status.Conditions != nil {
				for _, condition := range d.Status.Conditions {
					if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
						// Deployment has failed
						status = common.ArgoCDStatusFailed
						break
					}
				}
			}
		}
	}

	if asr.Instance.Status.ApplicationController != status {
		asr.Instance.Status.ApplicationController = status
	}

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
