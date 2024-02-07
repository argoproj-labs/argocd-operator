package notifications

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (nr *NotificationsReconciler) reconcileSecret() error {

	nr.Logger.Info("reconciling secrets")

	secretRequest := workloads.SecretRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NotificationsSecretName,
			Namespace: nr.Instance.Namespace,
			Labels:    resourceLabels,
		},

		Client:    nr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	desiredSecret, err := workloads.RequestSecret(secretRequest)
	if err != nil {
		nr.Logger.Error(err, "reconcileSecret: failed to request secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
		nr.Logger.Debug("reconcileSecret: one or more mutations could not be applied")
		return err
	}

	namespace, err := cluster.GetNamespace(nr.Instance.Namespace, nr.Client)
	if err != nil {
		nr.Logger.Error(err, "reconcileSecret: failed to retrieve namespace", "name", nr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := nr.deleteSecret(desiredSecret.Namespace); err != nil {
			nr.Logger.Error(err, "reconcileSecret: failed to delete secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
		}
		return err
	}

	_, err = workloads.GetSecret(desiredSecret.Name, desiredSecret.Namespace, nr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			nr.Logger.Error(err, "reconcileSecret: failed to retrieve secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(nr.Instance, desiredSecret, nr.Scheme); err != nil {
			nr.Logger.Error(err, "reconcileSecret: failed to set owner reference for secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
		}

		if err = workloads.CreateSecret(desiredSecret, nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileSecret: failed to create secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
			return err
		}
		nr.Logger.Info("secret created", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
		return nil
	}

	return nil
}

func (nr *NotificationsReconciler) deleteSecret(namespace string) error {
	if err := workloads.DeleteSecret(common.NotificationsSecretName, namespace, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		nr.Logger.Error(err, "DeleteSecret: failed to delete secret", "name", common.NotificationsSecretName, "namespace", namespace)
		return err
	}
	nr.Logger.Info("secret deleted", "name", common.NotificationsSecretName, "namespace", namespace)
	return nil
}
