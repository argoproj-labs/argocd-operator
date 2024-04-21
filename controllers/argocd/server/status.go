package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"

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
	return resource.UpdateStatusSubResource(sr.Instance, sr.Client)
}
