package notifications

import (
	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ConfigurationInstanceName = "default-notifications-configuration"
)

func (nr *NotificationsReconciler) reconcileConfigurationCR() error {
	defaultNotificationsConfigurationCR := &v1alpha1.NotificationsConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigurationInstanceName,
			Namespace: nr.Instance.Namespace,
		},
		Spec: v1alpha1.NotificationsConfigurationSpec{
			Context:   getDefaultNotificationsContext(),
			Triggers:  getDefaultNotificationsTriggers(),
			Templates: getDefaultNotificationsTemplates(),
		},
	}

	if _, err := resource.GetObject(ConfigurationInstanceName, nr.Instance.Namespace, defaultNotificationsConfigurationCR, nr.Client); err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileConfigurationCR: failed to retrieve notificationsConfigufation instance %s", defaultNotificationsConfigurationCR.GetName())
		}

		if err := resource.CreateObject(defaultNotificationsConfigurationCR, nr.Client); err != nil {
			return errors.Wrapf(err, "reconcileConfigurationCR: failed to create notificationsConfigufation instance %s", ConfigurationInstanceName)
		}
	}
	return nil
}

func (nr *NotificationsReconciler) deleteConfigurationCR() error {
	defaultNotificationsConfigurationCR := &v1alpha1.NotificationsConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigurationInstanceName,
			Namespace: nr.Instance.Namespace,
		},
	}

	if err := resource.DeleteObject(defaultNotificationsConfigurationCR.Name, defaultNotificationsConfigurationCR.Namespace, defaultNotificationsConfigurationCR, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteConfigurationCR: failed to delete configuration")
	}
	return nil
}
