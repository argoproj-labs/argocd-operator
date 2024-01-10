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

// TriggerHARollout deletes HA configmaps and statefulset to be recreated automatically during reconciliation, and triggers rollout for deployments
func (rr *RedisReconciler) TriggerHARollout() []error {
	var rolloutErrors []error

	err := rr.deleteConfigMap(common.ArgoCDRedisHAConfigMapName, rr.Instance.Namespace)
	if err != nil {
		rolloutErrors = append(rolloutErrors, err)
	}

	err = rr.deleteConfigMap(common.ArgoCDRedisHAHealthConfigMapName, rr.Instance.Namespace)
	if err != nil {
		rolloutErrors = append(rolloutErrors, err)
	}

	err = rr.TriggerDeploymentRollout(HAProxyResourceName, rr.Instance.Namespace, TLSCertChangedKey)
	if err != nil {
		rolloutErrors = append(rolloutErrors, err)
	}

	// If we use triggerRollout on the redis stateful set, kubernetes will attempt to restart the  pods
	// one at a time, and the first one to restart (which will be using tls) will hang as it tries to
	// communicate with the existing pods (which are not using tls) to establish which is the master.
	// So instead we delete the stateful set, which will delete all the pods.
	err = rr.deleteStatefulSet(HAServerResourceName, rr.Instance.Namespace)
	if err != nil {
		rolloutErrors = append(rolloutErrors, err)
	}

	return rolloutErrors
}

func (rr *RedisReconciler) DeleteHAResources() error {}
