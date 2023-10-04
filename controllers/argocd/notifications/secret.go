package notifications

import (
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (nr *NotificationsReconciler) reconcileSecret() error {

	nr.Logger.Info("reconciling secrets")

	secretRequest := workloads.SecretRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NotificationsSecretName,
			Namespace: nr.Instance.Namespace,
			Labels:    resourceLabels,
		},

		Client:    nr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	desiredSecret, err := workloads.RequestSecret(secretRequest)
	if err != nil {
		nr.Logger.Error(err, "reconcileSecret: failed to request secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
		return err
	}

	namespace, err := cluster.GetNamespace(nr.Instance.Namespace, nr.Client)
	if err != nil {
		nr.Logger.Error(err, "reconcileSecret: failed to retrieve namespace", "name", nr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := nr.DeleteSecret(desiredSecret.Namespace); err != nil {
			nr.Logger.Error(err, "reconcileSecret: failed to delete secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
		}
		return err
	}

	existingSecret, err := workloads.GetSecret(desiredSecret.Name, desiredSecret.Namespace, nr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			nr.Logger.Error(err, "reconcileSecret: failed to retrieve secret", "name", existingSecret.Name, "namespace", existingSecret.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(nr.Instance, desiredSecret, nr.Scheme); err != nil {
			nr.Logger.Error(err, "reconcileSecret: failed to set owner reference for secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
		}

		if err = workloads.CreateSecret(desiredSecret, nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileSecret: failed to create secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
			return err
		}
		nr.Logger.V(0).Info("reconcileSecret: secret created", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
		return nil
	}

	return nil
}

func (nr *NotificationsReconciler) DeleteSecret(namespace string) error {
	if err := workloads.DeleteSecret(NotificationsSecretName, namespace, nr.Client); err != nil {
		nr.Logger.Error(err, "DeleteSecret: failed to delete secret", "name", NotificationsSecretName, "namespace", namespace)
		return err
	}
	nr.Logger.V(0).Info("DeleteSecret: secret deleted", "name", NotificationsSecretName, "namespace", namespace)
	return nil
}
