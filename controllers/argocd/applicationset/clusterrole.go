package applicationset

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (asr *ApplicationSetReconciler) reconcileClusterRole() error {
	req := permissions.ClusterRoleRequest{
		ObjectMeta: argoutil.GetObjMeta(clusterResourceName, "", asr.Instance.Name, asr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getPolicyRuleForClusterRole(),
		Instance:   asr.Instance,
		Client:     asr.Client,
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
	return asr.reconClusterRole(req, argocdcommon.UpdateFnClusterRole(updateFn), ignoreDrift)
}

func (asr *ApplicationSetReconciler) reconClusterRole(req permissions.ClusterRoleRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := permissions.RequestClusterRole(req)
	if err != nil {
		asr.Logger.Debug("reconClusterRole: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconClusterRole: failed to request ClusterRole %s", desired.Name)
	}

	existing, err := permissions.GetClusterRole(desired.Name, asr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconClusterRole: failed to retrieve ClusterRole %s", desired.Name)
		}

		if err = permissions.CreateClusterRole(desired, asr.Client); err != nil {
			return errors.Wrapf(err, "reconClusterRole: failed to create ClusterRole %s", desired.Name)
		}
		asr.Logger.Info("cluster role created", "name", desired.Name)
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

	if err = permissions.UpdateClusterRole(existing, asr.Client); err != nil {
		return errors.Wrapf(err, "reconClusterRole: failed to update ClusterRole %s", existing.Name)
	}

	asr.Logger.Info("cluster role updated", "name", existing.Name)
	return nil
}

// deleteClusterRole will delete clusterrole with given name.
func (asr *ApplicationSetReconciler) deleteClusterRole(name string) error {
	if err := permissions.DeleteClusterRole(name, asr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteClusterRole: failed to delete clusterrole %s", name)
	}
	asr.Logger.Info("clusterrole deleted", "name", name)
	return nil
}

// getPolicyRuleForClusterRole returns policy rules for appset clusterrole
func getPolicyRuleForClusterRole() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		// ApplicationSet
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applications",
				"applicationsets",
			},
			Verbs: []string{
				"list",
				"watch",
			},
		},
		// Secrets
		{
			APIGroups: []string{""},
			Resources: []string{
				"secrets",
			},
			Verbs: []string{
				"list",
				"watch",
			},
		},
	}
}
