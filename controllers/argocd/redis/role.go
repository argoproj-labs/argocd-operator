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
	req := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
		Rules:      getPolicyRules(),
		Client:     rr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Instance:   rr.Instance,
	}

	desired, err := permissions.RequestRole(permissions.RoleRequest(req))
	if err != nil {
		return errors.Wrapf(err, "reconcileRole: failed to request role %s", desired.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileRole: failed to set owner reference for role", "name", desired.Name)
	}

	existing, err := permissions.GetRole(desired.Name, desired.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileRole: failed to retrieve role %s", desired.Name)
		}

		if err = permissions.CreateRole(desired, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileRole: failed to create role %s", desired.Name)
		}
		rr.Logger.Info("role created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	changed := false

	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
		{Existing: &existing.Rules, Desired: &desired.Rules, ExtraAction: nil},
	}

	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	if !changed {
		return nil
	}

	if err = permissions.UpdateRole(existing, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileRole: failed to update role %s", existing.Name)
	}

	rr.Logger.Info("role updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (rr *RedisReconciler) reconcileHARole() error {
	req := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(HAResourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component),
		Rules:      getHAPolicyRules(),
		Client:     rr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Instance:   rr.Instance,
	}

	desired, err := permissions.RequestRole(permissions.RoleRequest(req))
	if err != nil {
		return errors.Wrapf(err, "reconcileHARole: failed to request role %s", desired.Name)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconcileHARole: failed to set owner reference for role", "name", desired.Name)
	}

	existing, err := permissions.GetRole(desired.Name, desired.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileHARole: failed to retrieve role %s", desired.Name)
		}

		if err = permissions.CreateRole(desired, rr.Client); err != nil {
			return errors.Wrapf(err, "reconcileHARole: failed to create role %s", desired.Name)
		}
		rr.Logger.Info("role created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	changed := false

	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Annotations, Desired: &desired.Annotations, ExtraAction: nil},
		{Existing: &existing.Rules, Desired: &desired.Rules, ExtraAction: nil},
	}

	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	if !changed {
		return nil
	}

	if err = permissions.UpdateRole(existing, rr.Client); err != nil {
		return errors.Wrapf(err, "reconcileHARole: failed to update role %s", existing.Name)
	}
	rr.Logger.Info("role updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (rr *RedisReconciler) deleteRole(name, namespace string) error {
	if err := permissions.DeleteRole(name, namespace, rr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRole: failed to delete role %s", name)
	}
	rr.Logger.Info("role deleted", "name", name, "namespace", namespace)
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
