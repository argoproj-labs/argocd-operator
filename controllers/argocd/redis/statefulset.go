package redis

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (rr *RedisReconciler) TriggerStatefulSetRollout(name, namespace, key string) error {
	return argocdcommon.TriggerStatefulSetRollout(name, namespace, key, rr.Client)
}

func (rr *RedisReconciler) deleteStatefulSet(name, namespace string) error {
	if err := workloads.DeleteStatefulSet(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteStatefulSet: failed to delete stateful set %s", name)
	}
	rr.Logger.V(0).Info("deleteStatefulSet: stateful set deleted", "name", name, "namespace", namespace)
	return nil
}
