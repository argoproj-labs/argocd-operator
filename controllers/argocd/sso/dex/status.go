package dex

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// ReconcileStatus will ensure that the server status is updated for the given ArgoCD instance
func (dr *DexReconciler) ReconcileStatus() string {
	status := common.ArgoCDStatusUnknown

	if dr.Instance.Spec.Server.IsEnabled() {
		d, err := workloads.GetDeployment(resourceName, dr.Instance.Namespace, dr.Client)
		if err != nil {
			dr.Logger.Error(err, "ReconcileStatus: failed to retrieve deployment", "name", resourceName)
			return status
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

	return status
}
