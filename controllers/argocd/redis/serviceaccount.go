package redis

import (
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rr *RedisReconciler) reconcileServiceAccount() error {
	req := permissions.ServiceAccountRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
	}
	ignoreDrift := true
	return rr.reconServiceAccount(req, nil, ignoreDrift)
}

func (rr *RedisReconciler) reconServiceAccount(req permissions.ServiceAccountRequest, updateFn interface{}, ignoreDrift bool) error {
	desired := permissions.RequestServiceAccount(req)

	if err := controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconServiceAccount: failed to set owner reference for ServiceAccount", "name", desired.Name, "namespace", desired.Namespace)
	}

	_, err := permissions.GetServiceAccount(desired.Name, desired.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconServiceAccount: failed to retrieve ServiceAccount %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateServiceAccount(desired, rr.Client); err != nil {
			return errors.Wrapf(err, "reconServiceAccount: failed to create ServiceAccount %s in namespace %s", desired.Name, desired.Namespace)
		}
		rr.Logger.Info("service account created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// ServiceAccount found, no update required - nothing to do
	return nil
}

func (rr *RedisReconciler) deleteServiceAccount(name, namespace string) error {
	if err := permissions.DeleteServiceAccount(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceAccount: failed to delete service account %s", name)
	}
	rr.Logger.Info("service account deleted", "name", name, "namespace", namespace)
	return nil
}
