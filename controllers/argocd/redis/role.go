package redis

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rr *RedisReconciler) reconcileRole() error {
	roleReq := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
		Rules:      getPolicyRules(),
		Client:     rr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Instance:   rr.Instance,
	}

	desiredRole, err := permissions.RequestRole(permissions.RoleRequest(roleReq))
	if err != nil {
		return errors.Wrapf(err, "reconcileRole: failed to request role %s", desiredRole.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredRole, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileRole: failed to set owner reference for role", "name", desiredRole.Name)
	}

	existingRole, err := permissions.GetRole(desiredRole.Name, desiredRole.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileRole: failed to retrieve role %s", desiredRole.Name)
		}

		if err = permissions.CreateRole(desiredRole, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileRole: failed to create role %s", desiredRole.Name)
		}
		rr.Logger.V(0).Info("role created", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
		return nil
	}

	roleChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingRole.Rules, &desiredRole.Rules, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &roleChanged)
	}

	if !roleChanged {
		return nil
	}

	if err = permissions.UpdateRole(existingRole, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileRole: failed to update role %s", existingRole.Name)
	}

	rr.Logger.V(0).Info("role updated", "name", existingRole.Name, "namespace", existingRole.Namespace)
	return nil
}

func (rr *RedisReconciler) reconcileHARole() error {
	roleReq := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(HAResourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
		Rules:      getHAPolicyRules(),
		Client:     rr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Instance:   rr.Instance,
	}

	desiredRole, err := permissions.RequestRole(permissions.RoleRequest(roleReq))
	if err != nil {
		return errors.Wrapf(err, "reconcileHARole: failed to request role %s", desiredRole.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desiredRole, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileHARole: failed to set owner reference for role", "name", desiredRole.Name)
	}

	existingRole, err := permissions.GetRole(desiredRole.Name, desiredRole.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileHARole: failed to retrieve role %s", desiredRole.Name)
		}

		if err = permissions.CreateRole(desiredRole, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileHARole: failed to create role %s", desiredRole.Name)
		}
		rr.Logger.V(0).Info("role created", "name", desiredRole.Name, "namespace", desiredRole.Namespace)
		return nil
	}

	roleChanged := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingRole.Rules, &desiredRole.Rules, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &roleChanged)
	}

	if !roleChanged {
		return nil
	}

	if err = permissions.UpdateRole(existingRole, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileHARole: failed to update role %s", existingRole.Name)
	}
	rr.Logger.V(0).Info("role updated", "name", existingRole.Name, "namespace", existingRole.Namespace)
	return nil
}

func (rr *RedisReconciler) deleteRole(name, namespace string) error {
	if err := permissions.DeleteRole(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRole: failed to delete role %s", name)
	}
	rr.Logger.V(0).Info("role deleted", "name", name, "namespace", namespace)
	return nil
}

func getPolicyRules() []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{}
	return rules
}

func getHAPolicyRules() []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"endpoints",
			},
			Verbs: []string{
				"get",
			},
		},
	}
	return rules
}
