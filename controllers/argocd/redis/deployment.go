package redis

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (rr *RedisReconciler) getDesiredHAProxyDeployment() *appsv1.Deployment {
	desiredDeployment := &appsv1.Deployment{}

	return desiredDeployment
}

// TriggerDeploymentRollout starts redis deployment rollout by updating the given key
func (rr *RedisReconciler) TriggerDeploymentRollout(name, namespace, key string) error {
	return argocdcommon.TriggerDeploymentRollout(name, namespace, key, rr.Client)
}

func (rr *RedisReconciler) deleteDeployment(name, namespace string) error {
	if err := workloads.DeleteDeployment(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteDeployment: failed to delete deployment %s", name)
	}
	rr.Logger.V(0).Info("deleteDeployment: deployment deleted", "name", name, "namespace", namespace)
	return nil
}
