package redis

import (
	"github.com/pkg/errors"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	corev1 "k8s.io/api/core/v1"
)

var (
	HAProxyResourceName  string
	HAResourceName       string
	HAServerResourceName string
)

func (rr *RedisReconciler) reconcileHA() []error {
	var reconciliationErrors []error

	HAResourceName = argoutil.GenerateResourceName(rr.Instance.Name, common.RedisHASuffix)
	HAServerResourceName = argoutil.GenerateResourceName(rr.Instance.Name, common.RedisHAServerSuffix)
	HAProxyResourceName = argoutil.GenerateResourceName(rr.Instance.Name, common.RedisHAProxySuffix)

	// reconcile ha role
	if err := rr.reconcileHARole(); err != nil {
		reconciliationErrors = append(reconciliationErrors, errors.Wrapf(err, "reconcileHA: failed to reconcile role"))
	}

	// reconcile ha configmaps
	if errs := rr.reconcileHAConfigMaps(); len(errs) > 0 {
		for _, err := range reconciliationErrors {
			rr.Logger.Error(err, "reconcileHA")
		}
		reconciliationErrors = append(reconciliationErrors, errors.New("reconcileHA: failed to reconcile config maps"))
	}

	// reconcile ha services
	if errs := rr.reconcileHAServices(); len(errs) > 0 {
		for _, err := range reconciliationErrors {
			rr.Logger.Error(err, "reconcileHA")
		}
		reconciliationErrors = append(reconciliationErrors, errors.New("reconcileHA: failed to reconcile services"))
	}

	return reconciliationErrors
}

func (rr *RedisReconciler) reconcileHAConfigMaps() []error {
	var reconciliationErrors []error
	if err := rr.reconcileHAConfigMap(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	if err := rr.reconcileHAHealthConfigMap(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}
	return reconciliationErrors
}

func (rr *RedisReconciler) reconcileHAServices() []error {
	var reconciliationErrors []error
	if err := rr.reconcileHAMasterService(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	if err := rr.reconcileHAProxyService(); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	if errs := rr.reconcileHAAnnourceServices(); len(errs) > 0 {
		for _, err := range errs {
			rr.Logger.Error(err, "reconcileHAServices")
		}
		reconciliationErrors = append(reconciliationErrors, errors.New("reconcileHAServices: failed to reconcile HA annouce services"))
	}
	return reconciliationErrors
}

// TriggerHARollout deletes HA configmaps and statefulset to be recreated automatically during reconciliation, and triggers rollout for deployments
func (rr *RedisReconciler) TriggerHARollout(key string) []error {
	var rolloutErrors []error

	// delete and recreate HA config maps as part of rollout
	err := rr.deleteConfigMap(common.ArgoCDRedisHAConfigMapName, rr.Instance.Namespace)
	if err != nil {
		rolloutErrors = append(rolloutErrors, err)
	}

	err = rr.deleteConfigMap(common.ArgoCDRedisHAHealthConfigMapName, rr.Instance.Namespace)
	if err != nil {
		rolloutErrors = append(rolloutErrors, err)
	}

	errs := rr.reconcileHAConfigMaps()
	if len(errs) > 0 {
		for _, err := range errs {
			rr.Logger.Error(err, "TriggerHARollout")
		}
		rolloutErrors = append(rolloutErrors, errors.New("TriggerHARollout: failed to reconcile ha config maps"))
	}

	// rollout deployment
	err = rr.TriggerDeploymentRollout(HAProxyResourceName, rr.Instance.Namespace, key)
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
