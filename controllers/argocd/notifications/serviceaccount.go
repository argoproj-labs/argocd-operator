package notifications

import (
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileServiceAccount ensures ArgoCD server service account is present
func (nr *NotificationsReconciler) reconcileServiceAccount() error {
	req := permissions.ServiceAccountRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, nr.Instance.Namespace, nr.Instance.Name, nr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
	}

	return nr.reconServiceAccount(req)
}

func (nr *NotificationsReconciler) reconServiceAccount(req permissions.ServiceAccountRequest) error {
	desired := permissions.RequestServiceAccount(req)

	if err := controllerutil.SetControllerReference(nr.Instance, desired, nr.Scheme); err != nil {
		nr.Logger.Error(err, "reconServiceAccount: failed to set owner reference for ServiceAccount", "name", desired.Name, "namespace", desired.Namespace)
	}

	_, err := permissions.GetServiceAccount(desired.Name, desired.Namespace, nr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconServiceAccount: failed to retrieve ServiceAccount %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateServiceAccount(desired, nr.Client); err != nil {
			return errors.Wrapf(err, "reconServiceAccount: failed to create ServiceAccount %s in namespace %s", desired.Name, desired.Namespace)
		}
		nr.Logger.Info("service account created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// ServiceAccount found, no update required - nothing to do
	return nil
}

// deleteServiceAccount will delete service account with given name.
func (nr *NotificationsReconciler) deleteServiceAccount(name, namespace string) error {
	if err := permissions.DeleteServiceAccount(name, namespace, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceAccount: failed to delete serviceaccount %s in namespace %s", name, namespace)
	}
	nr.Logger.Info("service account deleted", "name", name, "namespace", namespace)
	return nil
}
