package server

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (sr *ServerReconciler) reconcileClusterRole() error {
	req := permissions.ClusterRoleRequest{
		ObjectMeta: argoutil.GetObjMeta(clusterResourceName, "", sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getPolicyRuleForClusterRole(),
		Instance:   sr.Instance,
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	ignoreDrift := false
	updateFn := func(existing, desired *rbacv1.ClusterRole, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Rules, Desired: &desired.Rules, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return sr.reconClusterRole(req, argocdcommon.UpdateFnClusterRole(updateFn), ignoreDrift)
}

func (sr *ServerReconciler) reconClusterRole(req permissions.ClusterRoleRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := permissions.RequestClusterRole(req)
	if err != nil {
		sr.Logger.Debug("reconClusterRole: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconClusterRole: failed to request ClusterRole %s", desired.Name)
	}

	existing, err := permissions.GetClusterRole(desired.Name, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconClusterRole: failed to retrieve ClusterRole %s", desired.Name)
		}

		if err = permissions.CreateClusterRole(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconClusterRole: failed to create ClusterRole %s", desired.Name)
		}
		sr.Logger.Info("cluster role created", "name", desired.Name)
		return nil
	}

	// ClusterRole found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnClusterRole); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconClusterRole: failed to execute update function for %s", existing.Name)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = permissions.UpdateClusterRole(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconClusterRole: failed to update ClusterRole %s", existing.Name)
	}

	sr.Logger.Info("cluster role updated", "name", existing.Name)
	return nil
}

// deleteClusterRole will delete clusterrole with given name.
func (sr *ServerReconciler) deleteClusterRole(name string) error {
	if err := permissions.DeleteClusterRole(name, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteClusterRole: failed to delete clusterrole %s", name)
	}
	sr.Logger.Info("clusterrole deleted", "name", name)
	return nil
}

// getPolicyRuleForClusterRole returns policy rules for server clusterrole
func getPolicyRuleForClusterRole() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"delete",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
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
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"list",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
				"cronjobs",
				"cronjobs/finalizers",
			},
			Verbs: []string{
				"create",
				"update",
			},
		},
	}
}
