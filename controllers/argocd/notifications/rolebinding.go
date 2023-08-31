package notifications

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (nr *NotificationsReconciler) reconcileRoleBinding() error {

	nr.Logger.Info("reconciling roleBindings")

	name := argoutil.GenerateUniqueResourceName(nr.Instance.Name, nr.Instance.Namespace, ArgoCDNotificationsControllerComponent)
	sa, err := permissions.GetServiceAccount(name, nr.Instance.Namespace, *nr.Client)

	if err != nil {
		nr.Logger.Error(err, "reconsileRoleBinding: failed to get serviceaccount", "name", name, "namespace", nr.Instance.Namespace)
		return err
	}

	roleBindingRequest := permissions.RoleBindingRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   nr.Instance.Namespace,
			Labels:      nr.Instance.Labels,
			Annotations: nr.Instance.Annotations,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     RoleKind,
			Name:     name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
	}

	desiredRoleBinding := permissions.RequestRoleBinding(roleBindingRequest)

	namespace, err := cluster.GetNamespace(nr.Instance.Namespace, *nr.Client)
	if err != nil {
		nr.Logger.Error(err, "reconcileRole: failed to retrieve namespace", "name", nr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := nr.DeleteRole(desiredRoleBinding.Name, desiredRoleBinding.Namespace); err != nil {
			nr.Logger.Error(err, "reconcileRoleBinding: failed to delete roleBinding", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
		}
		return err
	}

	existingRoleBinding, err := permissions.GetRoleBinding(desiredRoleBinding.Name, desiredRoleBinding.Namespace, *nr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			nr.Logger.Error(err, "reconcileRoleBinding: failed to retrieve roleBinding", "name", existingRoleBinding.Name, "namespace", existingRoleBinding.Namespace)
			return err
		}

		if err = controllerutil.SetControllerReference(nr.Instance, desiredRoleBinding, nr.Scheme); err != nil {
			nr.Logger.Error(err, "reconcileRole: failed to set owner reference for role", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
		}

		if err = permissions.CreateRoleBinding(desiredRoleBinding, *nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileRoleBinding: failed to create roleBinding", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
			return err
		}
		nr.Logger.V(0).Info("reconcileRoleBinding: roleBinding created", "name", desiredRoleBinding.Name, "namespace", desiredRoleBinding.Namespace)
		return nil
	}

	if !reflect.DeepEqual(existingRoleBinding.RoleRef, desiredRoleBinding.RoleRef) ||
		!reflect.DeepEqual(existingRoleBinding.Subjects, desiredRoleBinding.Subjects) {
		existingRoleBinding.RoleRef = desiredRoleBinding.RoleRef
		existingRoleBinding.Subjects = desiredRoleBinding.Subjects
		if err = permissions.UpdateRoleBinding(existingRoleBinding, *nr.Client); err != nil {
			nr.Logger.Error(err, "reconcileRoleBinding: failed to update roleBinding", "name", existingRoleBinding.Name, "namespace", existingRoleBinding.Namespace)
			return err
		}
	}

	nr.Logger.V(0).Info("reconcileRoleBinding: roleBinding updated", "name", existingRoleBinding.Name, "namespace", existingRoleBinding.Namespace)

	return nil
}

func (nr *NotificationsReconciler) DeleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, *nr.Client); err != nil {
		nr.Logger.Error(err, "DeleteRole: failed to delete roleBinding", "name", name, "namespace", namespace)
		return err
	}
	nr.Logger.V(0).Info("DeleteRoleBinding: roleBinding deleted", "name", name, "namespace", namespace)
	return nil
}
