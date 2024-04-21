package notifications

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

func (nr *NotificationsReconciler) reconcileRole() error {
	req := permissions.RoleRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, nr.Instance.Namespace, nr.Instance.Name, nr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getPolicyRules(),
		Client:     nr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
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
	return nr.reconRole(req, argocdcommon.UpdateFnRole(updateFn), ignoreDrift)
}

func (nr *NotificationsReconciler) reconRole(req permissions.RoleRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := permissions.RequestRole(req)
	if err != nil {
		nr.Logger.Debug("reconRole: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconRole: failed to request Role %s in namespace %s", desired.Name, desired.Namespace)
	}

	if desired.Namespace == nr.Instance.Namespace {
		if err = controllerutil.SetControllerReference(nr.Instance, desired, nr.Scheme); err != nil {
			nr.Logger.Error(err, "reconRole: failed to set owner reference for Role", "name", desired.Name, "namespace", desired.Namespace)
		}
	}

	existing, err := permissions.GetRole(desired.Name, desired.Namespace, nr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconRole: failed to retrieve Role %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateRole(desired, nr.Client); err != nil {
			return errors.Wrapf(err, "reconRole: failed to create Role %s in namespace %s", desired.Name, desired.Namespace)
		}
		nr.Logger.Info("role created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = permissions.UpdateRole(existing, nr.Client); err != nil {
		return errors.Wrapf(err, "reconRole: failed to update Role %s", existing.Name)
	}

	nr.Logger.Info("role updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (nr *NotificationsReconciler) deleteRole(name, namespace string) error {
	if err := permissions.DeleteRole(name, namespace, nr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		nr.Logger.Error(err, "DeleteRole: failed to delete role", "name", name, "namespace", namespace)
		return err
	}
	nr.Logger.Info("role deleted", "name", name, "namespace", namespace)
	return nil
}

func getPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
				"appprojects",
			},
			Verbs: []string{
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"configmaps",
				"secrets",
			},
			Verbs: []string{
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			ResourceNames: []string{
				"argocd-notifications-cm",
			},
			Resources: []string{
				"configmaps",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			ResourceNames: []string{
				"argocd-notifications-secret",
			},
			Resources: []string{
				"secrets",
			},
			Verbs: []string{
				"get",
			},
		},
	}
}
