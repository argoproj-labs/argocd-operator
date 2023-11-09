package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileServiceAccount ensures ArgoCD server service account is present
func (sr *ServerReconciler) reconcileServiceAccount() error {
	sr.Logger.V(0).Info("reconciling serviceAccount")

	saName := getServiceAccountName(sr.Instance.Name)
	saLabels := common.DefaultLabels(saName, sr.Instance.Name, ServerControllerComponent)

	saRequest := permissions.ServiceAccountRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        saName,
			Namespace:   sr.Instance.Namespace,
			Labels:      saLabels,
			Annotations: sr.Instance.Annotations,
		},
	}

	desiredSA := permissions.RequestServiceAccount(saRequest)

	// service account doesn't exist in the namespace, create it
	_, err := permissions.GetServiceAccount(desiredSA.Name, desiredSA.Namespace, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileServiceAccount: failed to retrieve serviceAccount", "name", desiredSA.Name, "namespace", desiredSA.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(sr.Instance, desiredSA, sr.Scheme); err != nil {
			sr.Logger.Error(err, "reconcileServiceAccount: failed to set owner reference for serviceAccount", "name", desiredSA.Name, "namespace", desiredSA.Namespace)
		}

		if err = permissions.CreateServiceAccount(desiredSA, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileServiceAccount: failed to create serviceAccount", "name", desiredSA.Name, "namespace", desiredSA.Namespace)
			return err
		}
		
		sr.Logger.V(0).Info("reconcileServiceAccount: serviceAccount created", "name", desiredSA.Name, "namespace", desiredSA.Namespace)
		return nil
	}

	return nil
}

// deleteServiceAccount will delete service account with given name.
func (sr *ServerReconciler) deleteServiceAccount(name, namespace string) error {
	if err := permissions.DeleteServiceAccount(name, namespace, sr.Client); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		sr.Logger.Error(err, "deleteServiceAccount: failed to delete serviceAccount", "name", name, "namespace", namespace)
		return err
	}
	sr.Logger.V(0).Info("deleteServiceAccount: serviceAccount deleted", "name", name, "namespace", namespace)
	return nil
}