package redis

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	appsv1 "k8s.io/api/apps/v1"
)

func (rr *RedisReconciler) getDesiredHAProxyDeployment() *appsv1.Deployment {
	desiredDeployment := &appsv1.Deployment{}

	return desiredDeployment
}

// TriggerDeploymentRollout starts redis deployment rollout by updating the given key
func (rr *RedisReconciler) TriggerDeploymentRollout(name, namespace, key string) error {
	return argocdcommon.TriggerDeploymentRollout(name, namespace, key, rr.Client)
}
