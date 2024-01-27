package redis

import (
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rr *RedisReconciler) reconcileServiceAccount() error {

	saReq := permissions.ServiceAccountRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
	}

	desiredSa := permissions.RequestServiceAccount(saReq)

	if err := controllerutil.SetControllerReference(rr.Instance, desiredSa, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileServiceAccount: failed to set owner reference for serviceaccount")
	}

	_, err := permissions.GetServiceAccount(desiredSa.Name, desiredSa.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileServiceAccount: failed to retrieve serviceaccount")
		}

		if err = permissions.CreateServiceAccount(desiredSa, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileServiceAccount: failed to create serviceaccount")
		}
		rr.Logger.Info("serviceaccount created", "name", desiredSa.Name, "namespace", desiredSa.Namespace)
		return nil
	}
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
