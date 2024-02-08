package argocdcommon

import (
	"errors"
	"reflect"

	"github.com/argoproj-labs/argocd-operator/pkg/util"
	appsv1 "k8s.io/api/apps/v1"
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

// FieldCompFnCm is a function type for comparing fields of two ConfigMaps.
type FieldCompFnCm func(*corev1.ConfigMap, *corev1.ConfigMap) []FieldToCompare

// FieldCompFnDeployment is a function type for comparing fields of two Deployments.
type FieldCompFnDeployment func(appsv1.Deployment, appsv1.Deployment) []FieldToCompare

// FieldCompFnSecret is a function type for comparing fields of two Secrets.
type FieldCompFnSecret func(corev1.Secret, corev1.Secret) []FieldToCompare

// FieldCompFnService is a function type for comparing fields of two Services.
type FieldCompFnService func(corev1.Service, corev1.Service) []FieldToCompare

// FieldCompFnIngress is a function type for comparing fields of two Ingresses.
type FieldCompFnIngress func(networkingv1.Ingress, networkingv1.Ingress) []FieldToCompare

// FieldCompFnRole is a function type for comparing fields of two Roles.
type FieldCompFnRole func(rbacv1.Role, rbacv1.Role) []FieldToCompare

// FieldCompFnRoleBinding is a function type for comparing fields of two RoleBindings.
type FieldCompFnRoleBinding func(rbacv1.RoleBinding, rbacv1.RoleBinding) []FieldToCompare

// FieldCompFnStatefulSet is a function type for comparing fields of two StatefulSets.
type FieldCompFnStatefulSet func(appsv1.StatefulSet, appsv1.StatefulSet) []FieldToCompare

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
		return reflect.ValueOf(value).String()
	}

	switch v := value.(type) {
	case string:
		if len(v) > 0 {
			return v
		}
		return defaultValue
	case map[string]string:
		if len(v) > 0 {
			return v
		}
		return defaultValue
	}

	return defaultValue
}
