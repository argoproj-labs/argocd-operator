package applicationset

import (
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (asr *ApplicationSetReconciler) reconcileServiceAccount() error {
	req := permissions.ServiceAccountRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, asr.Instance.Namespace, asr.Instance.Name, asr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
	}

	return asr.reconServiceAccount(req)
}

func (asr *ApplicationSetReconciler) reconServiceAccount(req permissions.ServiceAccountRequest) error {
	desired := permissions.RequestServiceAccount(req)

	if err := controllerutil.SetControllerReference(asr.Instance, desired, asr.Scheme); err != nil {
		asr.Logger.Error(err, "reconServiceAccount: failed to set owner reference for ServiceAccount", "name", desired.Name, "namespace", desired.Namespace)
	}

	_, err := permissions.GetServiceAccount(desired.Name, desired.Namespace, asr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconServiceAccount: failed to retrieve ServiceAccount %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateServiceAccount(desired, asr.Client); err != nil {
			return errors.Wrapf(err, "reconServiceAccount: failed to create ServiceAccount %s in namespace %s", desired.Name, desired.Namespace)
		}
		asr.Logger.Info("service account created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// ServiceAccount found, no update required - nothing to do
	return nil
}

// deleteServiceAccount will delete service account with given name.
func (asr *ApplicationSetReconciler) deleteServiceAccount(name, namespace string) error {
	if err := permissions.DeleteServiceAccount(name, namespace, asr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceAccount: failed to delete serviceaccount %s in namespace %s", name, namespace)
	}
	asr.Logger.Info("service account deleted", "name", name, "namespace", namespace)
	return nil
}
