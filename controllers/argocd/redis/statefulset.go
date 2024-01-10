package redis

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (rr *RedisReconciler) TriggerStatefulSetRollout(name, namespace, key string) error {
	return argocdcommon.TriggerStatefulSetRollout(name, namespace, key, rr.Client)
}

func (rr *RedisReconciler) deleteStatefulSet(name, namespace string) error {
	if err := workloads.DeleteStatefulSet(name, namespace, rr.Client); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		rr.Logger.Error(err, "DeleteStatefulSet: failed to delete StatefulSet", "name", name, "namespace", namespace)
		return err
	}
	rr.Logger.V(0).Info("DeleteStatefulSet: StatefulSet deleted", "name", name, "namespace", namespace)
	return nil
}
