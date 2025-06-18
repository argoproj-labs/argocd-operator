package notificationsconfiguration

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
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
		argoutil.AddWatchedByOperatorLabel(&NotificationsConfigMap.ObjectMeta)
		argoutil.LogResourceCreation(log, NotificationsConfigMap)
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

	// check context separately as converting context map to string produce different string due to random serialization of map value
	changed := checkIfContextChanged(cr, NotificationsConfigMap)

	for k := range expectedConfiguration {
		if !reflect.DeepEqual(expectedConfiguration[k], NotificationsConfigMap.Data[k]) && k != "context" {
			changed = true
		}
	}

	if changed {
		NotificationsConfigMap.Data = expectedConfiguration
		argoutil.LogResourceUpdate(log, NotificationsConfigMap, "updating config map data")
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

// checkIfContextChanged checks if context value in NotificationConfiguration and notificationConfigMap context have same value
// return true if there is difference, and false if no changes observed
func checkIfContextChanged(cr *v1alpha1.NotificationsConfiguration, notificationConfigMap *corev1.ConfigMap) bool {
	cmContext := strings.Split(strings.TrimSuffix(notificationConfigMap.Data["context"], "\n"), "\n")
	if len(cmContext) == len(cr.Spec.Context) {
		// Create a map for quick lookups
		stringMap := make(map[string]bool)
		for _, item := range cmContext {
			stringMap[item] = true
		}

		// Check for each item in array1
		for key, value := range cr.Spec.Context {
			if !stringMap[fmt.Sprintf("%s: %s", key, value)] {
				return true
			}
		}
	} else {
		return true
	}
	return false
}
