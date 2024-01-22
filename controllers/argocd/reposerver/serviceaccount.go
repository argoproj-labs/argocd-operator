package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rsr *RepoServerReconciler) reconcileServiceAccount() error {

	saReq := permissions.ServiceAccountRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rsr.Instance.Namespace, rsr.Instance.Name, rsr.Instance.Namespace, component),
	}

	desiredSa := permissions.RequestServiceAccount(saReq)

	if err := controllerutil.SetControllerReference(rsr.Instance, desiredSa, rsr.Scheme); err != nil {
		rsr.Logger.Error(err, "reconcileServiceAccount: failed to set owner reference for serviceaccount")
	}

	_, err := permissions.GetServiceAccount(desiredSa.Name, desiredSa.Namespace, rsr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileServiceAccount: failed to retrieve serviceaccount")
		}

		if err = permissions.CreateServiceAccount(desiredSa, rsr.Client); err != nil {
			return errors.Wrapf(err, "reconcileServiceAccount: failed to create serviceaccount")
		}
		rsr.Logger.V(0).Info("serviceaccount created", "name", desiredSa.Name, "namespace", desiredSa.Namespace)
		return nil
	}
	return nil
}

func (rsr *RepoServerReconciler) deleteServiceAccount(name, namespace string) error {
	if err := permissions.DeleteServiceAccount(name, namespace, rsr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceAccount: failed to delete service account %s", name)
	}
	rsr.Logger.V(0).Info("service account deleted", "name", name, "namespace", namespace)
	return nil
}
