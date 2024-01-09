package redis

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rr *RedisReconciler) reconcileHA() []error {
	var reconciliationErrors []error
	// reconcile configmaps
	reconciliationErrors = append(reconciliationErrors, rr.reconcileHAConfigMaps()...)
	if len(reconciliationErrors) > 0 {
		for _, re := range reconciliationErrors {
			rr.Logger.Error(re, "reconcileHA: failed to reconcile configmaps")
		}
		return reconciliationErrors
	}

	return reconciliationErrors
}

func (rr *RedisReconciler) reconcileHAConfigMaps() []error {
	var reconciliationErrors []error

	rr.Logger.Info("reconciling configMaps")

	if err := rr.reconcileHAConfigMap(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	if err := rr.reconcileHAHealthConfigMap(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	return reconciliationErrors
}

// GetHAContainerImage will return the container image for the Redis server in HA mode.
func (rr *RedisReconciler) GetHAContainerImage() string {
	fn := func(cr *argoproj.ArgoCD) (string, string) {
		return cr.Spec.Redis.Image, cr.Spec.Redis.Version
	}
	return argocdcommon.GetContainerImage(fn, rr.Instance, common.RedisHAImageEnvVar, common.ArgoCDDefaultRedisImage, common.ArgoCDDefaultRedisVersionHA)
}

// GetHAResources will return the ResourceRequirements for the Redis container in HA mode
func (rr *RedisReconciler) GetHAResources() corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if rr.Instance.Spec.HA.Resources != nil {
		resources = *rr.Instance.Spec.HA.Resources
	}
	return resources
}

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
			haproxyCfgKey:    rr.GetHAProxyConfig(),
			haproxyScriptKey: rr.GetHAProxyScript(),
			initScriptKey:    rr.GetInitScript(),
			redisConfKey:     rr.GetConf(),
			sentinelConfKey:  rr.GetSentinelConf(),
		},
	}

	desiredCM, err := workloads.RequestConfigMap(cmRequest)
	if err != nil {
		rr.Logger.Error(err, "reconcileHAConfigMap: failed to request configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
		rr.Logger.V(1).Info("reconcileHAConfigMap: one or more mutations could not be applied")
		return err
	}

	namespace, err := cluster.GetNamespace(rr.Instance.Namespace, rr.Client)
	if err != nil {
		rr.Logger.Error(err, "reconcileHAConfigMap: failed to retrieve namespace", "name", rr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := rr.deleteConfigMap(desiredCM.Name, desiredCM.Namespace); err != nil {
			rr.Logger.Error(err, "reconcileHAConfigMap: failed to delete configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
		}
		return err
	}

	_, err = workloads.GetConfigMap(desiredCM.Name, desiredCM.Namespace, rr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			rr.Logger.Error(err, "reconcileHAConfigMap: failed to retrieve configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(rr.Instance, desiredCM, rr.Scheme); err != nil {
			rr.Logger.Error(err, "reconcileHAConfigMap: failed to set owner reference for configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
		}

		if err = workloads.CreateConfigMap(desiredCM, rr.Client); err != nil {
			rr.Logger.Error(err, "reconcileHAConfigMap: failed to create configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
			return err
		}
		rr.Logger.V(0).Info("reconcileHAConfigMap: configMap created", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
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
			livenessScriptKey:         rr.GetLivenessScript(),
			readinessScriptKey:        rr.GetReadinessScript(),
			sentinelLivenessScriptKey: rr.GetSentinelLivenessScript(),
		},
	}

	desiredCM, err := workloads.RequestConfigMap(cmRequest)
	if err != nil {
		rr.Logger.Error(err, "reconcileHAHealthConfigMap: failed to request configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
		rr.Logger.V(1).Info("reconcileHAHealthConfigMap: one or more mutations could not be applied")
		return err
	}

	namespace, err := cluster.GetNamespace(rr.Instance.Namespace, rr.Client)
	if err != nil {
		rr.Logger.Error(err, "reconcileHAHealthConfigMap: failed to retrieve namespace", "name", rr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := rr.deleteConfigMap(desiredCM.Name, desiredCM.Namespace); err != nil {
			rr.Logger.Error(err, "reconcileHAHealthConfigMap: failed to delete configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
		}
		return err
	}

	_, err = workloads.GetConfigMap(desiredCM.Name, desiredCM.Namespace, rr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			rr.Logger.Error(err, "reconcileHAHealthConfigMap: failed to retrieve configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(rr.Instance, desiredCM, rr.Scheme); err != nil {
			rr.Logger.Error(err, "reconcileHAHealthConfigMap: failed to set owner reference for configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
		}

		if err = workloads.CreateConfigMap(desiredCM, rr.Client); err != nil {
			rr.Logger.Error(err, "reconcileHAHealthConfigMap: failed to create configMap", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
			return err
		}
		rr.Logger.V(0).Info("reconcileHAHealthConfigMap: configMap created", "name", desiredCM.Name, "namespace", desiredCM.Namespace)
		return nil
	}

	return nil
}
