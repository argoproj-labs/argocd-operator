package redis

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	corev1 "k8s.io/api/core/v1"
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
