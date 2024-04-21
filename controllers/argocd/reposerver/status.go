package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// ReconcileStatus will ensure that the Repo-server status is updated for the given ArgoCD instance
func (rsr *RepoServerReconciler) ReconcileStatus() error {
	status := common.ArgoCDStatusUnknown

	if rsr.Instance.Spec.Repo.IsEnabled() {
		deploy, err := workloads.GetDeployment(resourceName, rsr.Instance.Namespace, rsr.Client)
		if err != nil {
			return errors.Wrapf(err, "reconcileStatus: failed to retrieve deployment %s", resourceName)
		}

		status = common.ArgoCDStatusPending

		if deploy.Spec.Replicas != nil {
			if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
				status = common.ArgoCDStatusRunning
			} else if deploy.Status.Conditions != nil {
				for _, condition := range deploy.Status.Conditions {
					if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
						// Deployment has failed
						status = common.ArgoCDStatusFailed
						break
					}
				}
			}
		}
	}

	if rsr.Instance.Status.Repo != status {
		rsr.Instance.Status.Repo = status
	}

	return rsr.updateInstanceStatus()
}

func (rsr *RepoServerReconciler) updateInstanceStatus() error {
	return resource.UpdateStatusSubResource(rsr.Instance, rsr.Client)
}
