package notificationsconfiguration

import (
	"context"
	"fmt"
	"reflect"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	ArgoCDNotificationsConfigMap = "argocd-notifications-cm"
)

// reconcileNotificationsConfigmap will ensure that the notifications configuration is updated
func (r *NotificationsConfigurationReconciler) reconcileNotificationsConfigmap(cr *v1alpha1.NotificationsConfiguration) error {

	NotificationsConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ArgoCDNotificationsConfigMap,
			Namespace: cr.Namespace,
		},
	}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, NotificationsConfigMap.Name, NotificationsConfigMap); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the configmap %s : %s", NotificationsConfigMap.Name, err)
		}

		if err := controllerutil.SetControllerReference(cr, NotificationsConfigMap, r.Scheme); err != nil {
			return err
		}

		err := r.Client.Create(context.TODO(), NotificationsConfigMap)
		if err != nil {
			return err
		}
	}

	// Verify if Notifications Configmap data is up to date with NotificationsConfiguration CR data
	expectedConfiguration := make(map[string]string)

	for k, v := range cr.Spec.Triggers {
		expectedConfiguration[k] = v
	}

	for k, v := range cr.Spec.Templates {
		expectedConfiguration[k] = v
	}

	for k, v := range cr.Spec.Services {
		expectedConfiguration[k] = v
	}

	for k, v := range cr.Spec.Subscriptions {
		expectedConfiguration[k] = v
	}

	if cr.Spec.Context != nil {
		expectedConfiguration["context"] = mapToString(cr.Spec.Context)
	}

	if !reflect.DeepEqual(expectedConfiguration, NotificationsConfigMap.Data) {
		NotificationsConfigMap.Data = expectedConfiguration
		err := r.Client.Update(context.TODO(), NotificationsConfigMap)
		if err != nil {
			return err
		}
	}

	// Do nothing
	return nil
}
func mapToString(m map[string]string) string {
	result := ""
	for key, value := range m {
		result += fmt.Sprintf("%s: %s\n", key, value)
	}
	return result
}
