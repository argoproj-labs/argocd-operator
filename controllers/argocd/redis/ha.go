package redis

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

var (
	HAProxyResourceName  string
	HAResourceName       string
	HAServerResourceName string
)

func (rr *RedisReconciler) reconcileHA() error {

	// reconcile ha role
	if err := rr.reconcileHARole(); err != nil {
		return err
	}

	// reconcile serviceaccount
	if err := rr.reconcileServiceAccount(); err != nil {
		return err
	}

	// reconcile rolebinding
	if err := rr.reconcileRoleBinding(); err != nil {
		return err
	}

	// reconcile ha configmaps
	if err := rr.reconcileHAConfigMaps(); err != nil {
		return err
	}

	// reconcile ha services
	if err := rr.reconcileHAServices(); err != nil {
		return err
	}

	// reconcile haproxy Deployment
	if err := rr.reconcileHAProxyDeployment(); err != nil {
		return err
	}

	// reconcile ha statefulset
	if err := rr.reconcileHAStatefulSet(); err != nil {
		return err
	}

	// reconcile TLS secret
	if err := rr.reconcileTLSSecret(); err != nil {
		return err
	}
	return nil
}

func (rr *RedisReconciler) reconcileHAConfigMaps() error {
	var reconErrs util.MultiError

	err := rr.reconcileHAConfigMap()
	reconErrs.Append(err)

	err = rr.reconcileHAHealthConfigMap()
	reconErrs.Append(err)

	return reconErrs.ErrOrNil()
}

func (rr *RedisReconciler) reconcileHAServices() error {
	var reconErrs util.MultiError

	err := rr.reconcileHAMasterService()
	reconErrs.Append(err)

	err = rr.reconcileHAProxyService()
	reconErrs.Append(err)

	err = rr.reconcileHAAnnounceServices()
	reconErrs.Append(err)

	return reconErrs.ErrOrNil()
}

// TriggerHARollout deletes HA configmaps and statefulset to be recreated automatically during reconciliation, and triggers rollout for deployments
func (rr *RedisReconciler) TriggerHARollout(key string) error {
	var rolloutErrs util.MultiError

	// delete and recreate HA config maps as part of rollout
	if err := rr.deleteConfigMap(common.ArgoCDRedisHAConfigMapName, rr.Instance.Namespace); err != nil {
		rolloutErrs.Append(errors.Wrapf(err, "TriggerHARollout"))
	}

	if err := rr.deleteConfigMap(common.ArgoCDRedisHAHealthConfigMapName, rr.Instance.Namespace); err != nil {
		rolloutErrs.Append(errors.Wrapf(err, "TriggerHARollout"))
	}

	if err := rr.reconcileHAConfigMaps(); err != nil {
		rolloutErrs.Append(errors.Wrapf(err, "TriggerHARollout"))
	}

	// rollout deployment
	if err := rr.TriggerDeploymentRollout(HAProxyResourceName, rr.Instance.Namespace, key); err != nil {
		rolloutErrs.Append(errors.Wrapf(err, "TriggerHARollout"))
	}

	// If we use triggerRollout on the redis stateful set, kubernetes will attempt to restart the  pods
	// one at a time, and the first one to restart (which will be using tls) will hang as it tries to
	// communicate with the existing pods (which are not using tls) to establish which is the master.
	// So instead we delete the stateful set, which will delete all the pods.
	if err := rr.deleteStatefulSet(HAServerResourceName, rr.Instance.Namespace); err != nil {
		rolloutErrs.Append(errors.Wrapf(err, "TriggerHARollout"))
	}

	return rolloutErrs.ErrOrNil()
}

func (rr *RedisReconciler) DeleteHAResources() error {
	var deletionErr util.MultiError

	// delete statefulset
	err := rr.deleteStatefulSet(HAServerResourceName, rr.Instance.Namespace)
	deletionErr.Append(err)

	// delete deployment
	err = rr.deleteDeployment(HAProxyResourceName, rr.Instance.Namespace)
	deletionErr.Append(err)

	// delete services
	err = rr.deleteHAServices()
	deletionErr.Append(err)

	// delete configmaps
	err = rr.deleteHAConfigmaps()
	deletionErr.Append(err)

	// delete role
	err = rr.deleteRole(HAResourceName, rr.Instance.Namespace)
	deletionErr.Append(err)

	return deletionErr.ErrOrNil()
}

func (rr *RedisReconciler) deleteHAServices() error {
	var delErrs util.MultiError

	err := rr.deleteService(HAResourceName, rr.Instance.Namespace)
	delErrs.Append(err)

	err = rr.deleteService(HAProxyResourceName, rr.Instance.Namespace)
	delErrs.Append(err)

	for i := int32(0); i < common.DefaultRedisHAReplicas; i++ {
		svcName := argoutil.GenerateResourceName(rr.Instance.Name, fmt.Sprintf("%s-%d", common.RedisHAAnnouceSuffix, i))
		err := rr.deleteService(svcName, rr.Instance.Namespace)
		delErrs.Append(err)
	}

	return delErrs.ErrOrNil()
}

func (rr *RedisReconciler) deleteHAConfigmaps() error {
	var delErrs util.MultiError

	err := rr.deleteConfigMap(common.ArgoCDRedisHAConfigMapName, rr.Instance.Namespace)
	delErrs.Append(err)

	err = rr.deleteConfigMap(common.ArgoCDRedisHAHealthConfigMapName, rr.Instance.Namespace)
	delErrs.Append(err)

	return delErrs.ErrOrNil()
}
