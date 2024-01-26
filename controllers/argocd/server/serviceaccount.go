package server

import (
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileServiceAccount ensures ArgoCD server service account is present
func (sr *ServerReconciler) reconcileServiceAccount() error {

	saReq := permissions.ServiceAccountRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component),
	}

	desiredSa := permissions.RequestServiceAccount(saReq)

	if err := controllerutil.SetControllerReference(sr.Instance, desiredSa, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileServiceAccount: failed to set owner reference for serviceaccount", "name", desiredSa.Name, "namespace", desiredSa.Namespace)
	}

	// service account doesn't exist in the namespace, create it
	_, err := permissions.GetServiceAccount(desiredSa.Name, desiredSa.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileServiceAccount: failed to retrieve serviceaccount %s in namespace %s", desiredSa.Name, desiredSa.Namespace)
		}

		if err = permissions.CreateServiceAccount(desiredSa, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileServiceAccount: failed to create serviceaccount %s in namespace %s", desiredSa.Name, desiredSa.Namespace)
		}

		sr.Logger.V(0).Info("serviceaccount created", "name", desiredSa.Name, "namespace", desiredSa.Namespace)
		return nil
	}

	// serviceaccount exist, do nothing
	return nil
}

// deleteServiceAccount will delete service account with given name.
func (sr *ServerReconciler) deleteServiceAccount(name, namespace string) error {
	if err := permissions.DeleteServiceAccount(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceAccount: failed to delete serviceaccount %s in namespace %s", name, namespace)
	}
	sr.Logger.V(0).Info("serviceaccount deleted", "name", name, "namespace", namespace)
	return nil
}
