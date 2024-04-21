package notifications

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// reconcileStatus will ensure that the notifications controller status is updated for the given ArgoCD instance
func (nr *NotificationsReconciler) ReconcileStatus() error {
	status := common.ArgoCDStatusUnknown

	if nr.Instance.Spec.Notifications.Enabled {
		d, err := workloads.GetDeployment(resourceName, nr.Instance.Namespace, nr.Client)
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

	if nr.Instance.Status.NotificationsController != status {
		nr.Instance.Status.NotificationsController = status
	}

	return nr.updateInstanceStatus()
}

func (nr *NotificationsReconciler) updateInstanceStatus() error {
	return resource.UpdateStatusSubResource(nr.Instance, nr.Client)
}
