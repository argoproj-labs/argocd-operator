package notifications

import (
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"

	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (nr *NotificationsReconciler) reconcileServiceAccount() error {

	nr.Logger.Info("reconciling serviceAccounts")

	serviceAccountRequest := permissions.ServiceAccountRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceName,
			Namespace:   nr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: nr.Instance.Annotations,
		},
	}

	desiredServiceAccount := permissions.RequestServiceAccount(serviceAccountRequest)

	namespace, err := cluster.GetNamespace(nr.Instance.Namespace, nr.Client)
	if err != nil {
		nr.Logger.Error(err, "reconcileServiceAccount: failed to retrieve namespace", "name", nr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := nr.deleteServiceAccount(desiredServiceAccount.Name, desiredServiceAccount.Namespace); err != nil {
			nr.Logger.Error(err, "reconcileServiceAccount: failed to delete serviceAccount", "name", desiredServiceAccount.Name, "namespace", desiredServiceAccount.Namespace)
		}
		return err
	}

	_, err = permissions.GetServiceAccount(desiredServiceAccount.Name, desiredServiceAccount.Namespace, nr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			nr.Logger.Error(err, "reconcileServiceAccount: failed to retrieve serviceAccount", "name", desiredServiceAccount.Name, "namespace", desiredServiceAccount.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(nr.Instance, desiredServiceAccount, nr.Scheme); err != nil {
			nr.Logger.Error(err, "reconcileServiceAccount: failed to set owner reference for serviceAccount", "name", desiredServiceAccount.Name, "namespace", desiredServiceAccount.Namespace)
		}

		if err = permissions.CreateServiceAccount(desiredServiceAccount, nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileServiceAccount: failed to create serviceAccount", "name", desiredServiceAccount.Name, "namespace", desiredServiceAccount.Namespace)
			return err
		}
		nr.Logger.Info("serviceAccount created", "name", desiredServiceAccount.Name, "namespace", desiredServiceAccount.Namespace)
		return nil
	}

	return nil
}

func (nr *NotificationsReconciler) deleteServiceAccount(name, namespace string) error {
	if err := permissions.DeleteServiceAccount(name, namespace, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		nr.Logger.Error(err, "DeleteServiceAccount: failed to delete serviceAccount", "name", name, "namespace", namespace)
		return err
	}
	nr.Logger.Info("serviceAccount deleted", "name", name, "namespace", namespace)
	return nil
}
