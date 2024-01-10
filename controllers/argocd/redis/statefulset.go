package redis

import "github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"

func (rr *RedisReconciler) TriggerStatefulSetRollout(name, namespace, key string) error {
	return argocdcommon.TriggerStatefulSetRollout(name, namespace, key, rr.Client)
}
