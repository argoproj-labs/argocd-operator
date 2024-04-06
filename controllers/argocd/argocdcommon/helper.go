package argocdcommon

import (
	"errors"
	"reflect"

	"github.com/argoproj-labs/argocd-operator/pkg/util"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// FieldToCompare contains a field from an existing resource, the same field in the desired state of the resource, and an action to be taken after comparison
type FieldToCompare struct {
	Existing    interface{}
	Desired     interface{}
	ExtraAction func()
}

type UpdateFnCm func(*corev1.ConfigMap, *corev1.ConfigMap, *bool) error

type UpdateFnRole func(*rbacv1.Role, *rbacv1.Role, *bool) error

type UpdateFnClusterRole func(*rbacv1.ClusterRole, *rbacv1.ClusterRole, *bool) error

type UpdateFnRb func(*rbacv1.RoleBinding, *rbacv1.RoleBinding, *bool) error

type UpdateFnCrb func(*rbacv1.ClusterRoleBinding, *rbacv1.ClusterRoleBinding, *bool) error

type UpdateFnSvc func(*corev1.Service, *corev1.Service, *bool) error

type UpdateFnSM func(*monitoringv1.ServiceMonitor, *monitoringv1.ServiceMonitor, *bool) error

type UpdateFnPR func(*monitoringv1.PrometheusRule, *monitoringv1.PrometheusRule, *bool) error

type UpdateFnIngress func(*networkingv1.Ingress, *networkingv1.Ingress, *bool) error

type UpdateFnRoute func(*routev1.Route, *routev1.Route, *bool) error

type UpdateFnSa func(*corev1.ServiceAccount, *corev1.ServiceAccount, *bool) error

type UpdateFnSs func(*appsv1.StatefulSet, *appsv1.StatefulSet, *bool) error

type UpdateFnDep func(*appsv1.Deployment, *appsv1.Deployment, *bool) error

type UpdateFnHPA func(*autoscalingv1.HorizontalPodAutoscaler, *autoscalingv1.HorizontalPodAutoscaler, *bool) error

type UpdateFnSecret func(*corev1.Secret, *corev1.Secret, *bool) error

// UpdateIfChanged accepts a slice of fields to be compared, along with a bool ptr. It compares all the provided fields, updating any fields and setting the bool ptr to true if a drift is detected
func UpdateIfChanged(ftc []FieldToCompare, changed *bool) {
	for _, field := range ftc {
		if util.IsPtr(field.Existing) && util.IsPtr(field.Desired) {
			if !reflect.DeepEqual(field.Existing, field.Desired) {
				reflect.ValueOf(field.Existing).Elem().Set(reflect.ValueOf(field.Desired).Elem())
				if field.ExtraAction != nil {
					field.ExtraAction()
				}
				*changed = true
			}
		}
	}
}

// PartialMatch accepts a slice of fields to be compared, along with a bool ptr. It compares all the provided fields and sets the bool to false if a drift is detected
func PartialMatch(ftc []FieldToCompare, match *bool) {
	for _, field := range ftc {
		if !reflect.DeepEqual(field.Existing, field.Desired) {
			*match = false
		}
	}
}

// IsMergable returns error if any of the extraArgs is already part of the default command Arguments.
func IsMergable(extraArgs []string, cmd []string) error {
	if len(extraArgs) > 0 {
		for _, arg := range extraArgs {
			if len(arg) > 2 && arg[:2] == "--" {
				if ok := util.ContainsString(cmd, arg); ok {
					err := errors.New("duplicate argument error")
					return err
				}
			}
		}
	}
	return nil
}

// GetValueOrDefault returns the value if it's non-empty, otherwise returns the default value.
func GetValueOrDefault(value interface{}, defaultValue interface{}) interface{} {
	if util.IsPtr(value) {
		if reflect.ValueOf(value).IsNil() {
			return defaultValue
		}
		ptVal := reflect.Indirect(reflect.ValueOf(value))

		switch ptVal.Kind() {
		case reflect.String:
			return reflect.Indirect(reflect.ValueOf(value)).String()
		}
	}

	switch v := value.(type) {
	case string:
		if len(v) > 0 {
			return v
		}
		return defaultValue.(string)
	case map[string]string:
		if len(v) > 0 {
			return v
		}
		return defaultValue.(map[string]string)
	}

	return defaultValue
}
