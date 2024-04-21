package redis

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
)

// ReconcileStatus will ensure that the Redis status is updated for the given ArgoCD instance
func (rr *RedisReconciler) ReconcileStatus() error {
	status := common.ArgoCDStatusUnknown

	if rr.Instance.Spec.Redis.IsEnabled() {
		if rr.Instance.Spec.HA.Enabled {
			ss, err := workloads.GetStatefulSet(HAServerResourceName, rr.Instance.Namespace, rr.Client)
			if err != nil {
				return errors.Wrapf(err, "failed to retrieve statefulset %s", HAServerResourceName)
			}

			status = common.ArgoCDStatusPending

			if ss.Spec.Replicas != nil {
				if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
					status = common.ArgoCDStatusRunning
				}
			}
		} else {
			deploy, err := workloads.GetDeployment(resourceName, rr.Instance.Namespace, rr.Client)
			if err != nil {
				return errors.Wrapf(err, "failed to retrieve deployment %s", resourceName)
			}

			status = common.ArgoCDStatusPending

			if deploy.Spec.Replicas != nil {
				if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
					status = common.ArgoCDStatusRunning
				}
			}
		}
	}

	if rr.Instance.Status.Redis != status {
		rr.Instance.Status.Redis = status
	}

	return rr.updateInstanceStatus()
}

func (rr *RedisReconciler) updateInstanceStatus() error {
	return resource.UpdateStatusSubResource(rr.Instance, rr.Client)
}
