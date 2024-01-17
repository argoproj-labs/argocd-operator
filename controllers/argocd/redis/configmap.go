package redis

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	haproxyCfgKey             = "haproxy.cfg"
	haproxyScriptKey          = "haproxy_init.sh"
	initScriptKey             = "init.sh"
	redisConfKey              = "redis.conf"
	sentinelConfKey           = "sentinel.Conf"
	livenessScriptKey         = "redis_liveness.sh"
	readinessScriptKey        = "redis_readiness.sh"
	sentinelLivenessScriptKey = "sentinel_liveness.sh"
)

// reconcileHAConfigMap will ensure that the Redis HA ConfigMap is present for the given ArgoCD instance
func (rr *RedisReconciler) reconcileHAConfigMap() error {
	cmRequest := workloads.ConfigMapRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        common.ArgoCDRedisHAConfigMapName,
			Namespace:   rr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: rr.Instance.Annotations,
		},
		Data: map[string]string{
			haproxyCfgKey:    rr.getHAProxyConfig(),
			haproxyScriptKey: rr.getHAProxyScript(),
			initScriptKey:    rr.getInitScript(),
			redisConfKey:     rr.getConf(),
			sentinelConfKey:  rr.getSentinelConf(),
		},
	}

	desiredCM, err := workloads.RequestConfigMap(cmRequest)
	if err != nil {
		rr.Logger.V(1).Info("reconcileHAConfigMap: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileHAConfigMap: failed to request configMap %s in namespace %s", desiredCM.Name, desiredCM.Namespace)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredCM, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileHAConfigMap: failed to set owner reference for configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
	}

	_, err = workloads.GetConfigMap(desiredCM.Name, desiredCM.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileHAConfigMap: failed to retrieve configMap %s in namespace %s", desiredCM.Name, desiredCM.Namespace)
		}

		if err = workloads.CreateConfigMap(desiredCM, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileHAConfigMap: failed to create configMap %s in namespace %s", desiredCM.Name, desiredCM.Namespace)
		}
		rr.Logger.V(0).Info("config map created", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
		return nil
	}

	return nil
}

// reconcileHAHealthConfigMap will ensure that the Redis HA Health ConfigMap is present for the given ArgoCD.
func (rr *RedisReconciler) reconcileHAHealthConfigMap() error {
	cmRequest := workloads.ConfigMapRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        common.ArgoCDRedisHAHealthConfigMapName,
			Namespace:   rr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: rr.Instance.Annotations,
		},
		Data: map[string]string{
			livenessScriptKey:         rr.getLivenessScript(),
			readinessScriptKey:        rr.getReadinessScript(),
			sentinelLivenessScriptKey: rr.getSentinelLivenessScript(),
		},
	}

	desiredCM, err := workloads.RequestConfigMap(cmRequest)
	if err != nil {
		rr.Logger.V(1).Info("reconcileHAHealthConfigMap: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileHAHealthConfigMap: failed to request configMap %s", desiredCM.Namespace)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredCM, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileHAHealthConfigMap: failed to set owner reference for configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
	}

	_, err = workloads.GetConfigMap(desiredCM.Name, desiredCM.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileHAHealthConfigMap: failed to retrieve configMap %s in namespace %s", desiredCM.Name, desiredCM.Namespace)
		}

		if err = workloads.CreateConfigMap(desiredCM, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileHAHealthConfigMap: failed to create configMap %s in namespace %s", desiredCM.Name, desiredCM.Namespace)
		}
		rr.Logger.V(0).Info("configMap created", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
		return nil
	}

	return nil
}

func (rr *RedisReconciler) deleteConfigMap(name, namespace string) error {
	if err := workloads.DeleteConfigMap(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteConfigMap: failed to delete config map %s", name)
	}
	rr.Logger.V(0).Info("config map deleted", "name", name, "namespace", namespace)
	return nil
}
