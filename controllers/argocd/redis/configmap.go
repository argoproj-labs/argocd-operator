package redis

import (
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (rr *RedisReconciler) deleteConfigMap(name, namespace string) error {
	if err := workloads.DeleteConfigMap(name, namespace, rr.Client); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		rr.Logger.Error(err, "DeleteConfigMap: failed to delete configMap", "name", name, "namespace", namespace)
		return err
	}
	rr.Logger.V(0).Info("DeleteConfigMap: configMap deleted", "name", name, "namespace", namespace)
	return nil
}
