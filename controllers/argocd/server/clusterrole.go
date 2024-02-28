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

	// ArgoCD instance is not cluster scoped, cleanup any existing clusterrole & exit
	if !sr.ClusterScoped {
		return sr.deleteClusterRole(clusterResourceName)
	}

	req := permissions.ClusterRoleRequest{
		ObjectMeta: argoutil.GetObjMeta(clusterResourceName, "", sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getPolicyRuleForClusterRole(),
		Instance:   sr.Instance,
		Client:     sr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	desired, err := permissions.RequestClusterRole(req)
	if err != nil {
		return errors.Wrapf(err, "reconcileClusterRole: failed to request clusterrole %s", desired.Name)
	}

	// clusterrole doesn't exist, create it
	existing, err := permissions.GetClusterRole(desired.Name, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileClusterRole: failed to retrieve clusterrole %s", desired.Name)
		}

		if err = permissions.CreateClusterRole(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileClusterRole: failed to create clusterrole %s", desired.Name)
		}
		sr.Logger.Info("clusterrole created", "name", desired.Name)
		return nil
	}

	// difference in existing & desired clusterrole, update it
	changed := false
	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Rules, Desired: &desired.Rules, ExtraAction: nil},
	}
	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = permissions.UpdateClusterRole(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileClusterRole: failed to update clusterrole %s", existing.Name)
	}

	sr.Logger.Info("clusterrole updated", "name", existing.Name)
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
	}
}
