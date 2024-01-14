package redis

import (
	"context"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
)

// reconcileStatusRedis will ensure that the Redis status is updated for the given ArgoCD instance
func (rr *RedisReconciler) reconcileStatus() error {
	status := common.ArgoCDStatusUnknown

	if !rr.Instance.Spec.HA.Enabled {
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
	} else {
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
	}

	if rr.Instance.Status.Redis != status {
		rr.Instance.Status.Redis = status
	}

	return rr.UpdateInstanceStatus()
}

func (rr *RedisReconciler) UpdateInstanceStatus() error {
	return rr.Client.Status().Update(context.TODO(), rr.Instance)
}
