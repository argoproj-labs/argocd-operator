package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rsr *RepoServerReconciler) reconcileServiceAccount() error {

	req := permissions.ServiceAccountRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rsr.Instance.Namespace, rsr.Instance.Name, rsr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
	}

	desired := permissions.RequestServiceAccount(req)

	if err := controllerutil.SetControllerReference(rsr.Instance, desired, rsr.Scheme); err != nil {
		rsr.Logger.Error(err, "reconcileServiceAccount: failed to set owner reference for serviceaccount")
	}

	_, err := permissions.GetServiceAccount(desired.Name, desired.Namespace, rsr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileServiceAccount: failed to retrieve serviceaccount")
		}

		if err = permissions.CreateServiceAccount(desired, rsr.Client); err != nil {
			return errors.Wrapf(err, "reconcileServiceAccount: failed to create serviceaccount")
		}
		rsr.Logger.Info("serviceaccount created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}
	return nil
}

func (rsr *RepoServerReconciler) deleteServiceAccount(name, namespace string) error {
	if err := permissions.DeleteServiceAccount(name, namespace, rsr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceAccount: failed to delete serviceaccount %s in namespace %s", name, namespace)
	}
	rsr.Logger.Info("service account deleted", "name", name, "namespace", namespace)
	return nil
}
