package redis

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rr *RedisReconciler) reconcileRole() error {
	req := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getPolicyRules(),
		Client:     rr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Instance:   rr.Instance,
	}

	ignoreDrift := false
	updateFn := func(existing, desired *rbacv1.Role, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},

			{Existing: &existing.Rules, Desired: &desired.Rules, ExtraAction: nil},
		}

		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return rr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
}

func (rr *RedisReconciler) reconcileHARole() error {
	req := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(HAResourceName, rr.Instance.Namespace, rr.Instance.Name, rr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getHAPolicyRules(),
		Client:     rr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Instance:   rr.Instance,
	}

	ignoreDrift := false
	updateFn := func(existing, desired *rbacv1.Role, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},

			{Existing: &existing.Rules, Desired: &desired.Rules, ExtraAction: nil},
		}

		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return rr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
}

func (rr *RedisReconciler) reconRole(req permissions.RoleRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := permissions.RequestRole(req)
	if err != nil {
		rr.Logger.Debug("reconRole: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconRole: failed to request Role %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(rr.Instance, desired, rr.Scheme); err != nil {
		rr.Logger.Error(err, "reconRole: failed to set owner reference for Role", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := permissions.GetRole(desired.Name, desired.Namespace, rr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRole: failed to retrieve Role %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateRole(desired, rr.Client); err != nil {
			return errors.Wrapf(err, "reconRole: failed to create Role %s in namespace %s", desired.Name, desired.Namespace)
		}
		rr.Logger.Info("role created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// Role found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnRole); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconRole: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = permissions.UpdateRole(existing, rr.Client); err != nil {
		return errors.Wrapf(err, "reconRole: failed to update Role %s", existing.Name)
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
