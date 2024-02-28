package server

import (
	"context"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	"k8s.io/client-go/util/retry"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// ReconcileStatus will ensure that the server status is updated for the given ArgoCD instance
func (sr *ServerReconciler) ReconcileStatus() error {
	status := common.ArgoCDStatusUnknown

	if sr.Instance.Spec.Server.IsEnabled() {
		d, err := workloads.GetDeployment(resourceName, sr.Instance.Namespace, sr.Client)
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

	if sr.Instance.Status.Server != status {
		sr.Instance.Status.Server = status
	}

	return sr.updateInstanceStatus()
}

func (sr *ServerReconciler) updateInstanceStatus() error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := sr.Client.Status().Update(context.TODO(), sr.Instance); err != nil {
			return errors.Wrap(err, "updateInstanceStatus: failed to update instance status")
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
