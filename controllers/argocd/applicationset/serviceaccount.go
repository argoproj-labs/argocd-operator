package applicationset

import (
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (asr *ApplicationSetReconciler) reconcileServiceAccount() error {

	asr.Logger.Info("reconciling serviceAccounts")

	serviceAccountRequest := permissions.ServiceAccountRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        resourceName,
			Namespace:   asr.Instance.Namespace,
			Labels:      resourceLabels,
			Annotations: asr.Instance.Annotations,
		},
	}

	desiredServiceAccount := permissions.RequestServiceAccount(serviceAccountRequest)

	namespace, err := cluster.GetNamespace(asr.Instance.Namespace, asr.Client)
	if err != nil {
		asr.Logger.Error(err, "reconcileServiceAccount: failed to retrieve namespace", "name", asr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := asr.deleteServiceAccount(desiredServiceAccount.Name, desiredServiceAccount.Namespace); err != nil {
			asr.Logger.Error(err, "reconcileServiceAccount: failed to delete serviceAccount", "name", desiredServiceAccount.Name, "namespace", desiredServiceAccount.Namespace)
		}
		return err
	}

	_, err = permissions.GetServiceAccount(desiredServiceAccount.Name, desiredServiceAccount.Namespace, asr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			asr.Logger.Error(err, "reconcileServiceAccount: failed to retrieve serviceAccount", "name", desiredServiceAccount.Name, "namespace", desiredServiceAccount.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(asr.Instance, desiredServiceAccount, asr.Scheme); err != nil {
			asr.Logger.Error(err, "reconcileServiceAccount: failed to set owner reference for serviceAccount", "name", desiredServiceAccount.Name, "namespace", desiredServiceAccount.Namespace)
		}

		if err = permissions.CreateServiceAccount(desiredServiceAccount, asr.Client); err != nil {
			asr.Logger.Error(err, "reconcileServiceAccount: failed to create serviceAccount", "name", desiredServiceAccount.Name, "namespace", desiredServiceAccount.Namespace)
			return err
		}
		asr.Logger.V(0).Info("reconcileServiceAccount: serviceAccount created", "name", desiredServiceAccount.Name, "namespace", desiredServiceAccount.Namespace)
		return nil
	}

	return nil
}

func (nr *ApplicationSetReconciler) deleteServiceAccount(name, namespace string) error {
	if err := permissions.DeleteServiceAccount(name, namespace, nr.Client); err != nil {
		nr.Logger.Error(err, "DeleteServiceAccount: failed to delete serviceAccount", "name", name, "namespace", namespace)
		return err
	}
	nr.Logger.V(0).Info("DeleteServiceAccount: serviceAccount deleted", "name", name, "namespace", namespace)
	return nil
}
