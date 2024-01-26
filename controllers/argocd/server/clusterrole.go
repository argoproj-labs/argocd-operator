package server

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	v1 "k8s.io/api/rbac/v1"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (sr *ServerReconciler) reconcileClusterRole() error {

	// ArgoCD instance is not cluster scoped, cleanup any existing clusterrole & exit
	if !sr.ClusterScoped {
		return sr.deleteClusterRole(uniqueResourceName)
	}

	crReq := permissions.ClusterRoleRequest{
		ObjectMeta: argoutil.GetObjMeta(uniqueResourceName, "", sr.Instance.Name, sr.Instance.Namespace, component),
		Rules:     getPolicyRuleForClusterRule(),
		Instance: sr.Instance,
		Client:    sr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	desiredCr, err := permissions.RequestClusterRole(crReq)
	if err != nil {
		return errors.Wrapf(err, "reconcileClusterRole: failed to request clusterrole %s", desiredCr.Name)
	}

	// clusterrole doesn't exist, create it
	existingCr, err := permissions.GetClusterRole(desiredCr.Name, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileClusterRole: failed to retrieve clusterrole %s", desiredCr.Name)
		}

		if err = permissions.CreateClusterRole(desiredCr, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileClusterRole: failed to create clusterrole %s", desiredCr.Name)
		}
		sr.Logger.V(0).Info("clusterrole created", "name", desiredCr.Name)
		return nil
	}

	// difference in existing & desired clusterrole, update it
	changed := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{&existingCr.Rules, &desiredCr.Rules, nil},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
	}

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = permissions.UpdateClusterRole(existingCr, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileClusterRole: failed to update clusterrole %s", existingCr.Name)
	}

	sr.Logger.V(0).Info("clusterrole updated", "name", existingCr.Name)
	return nil
}

// getPolicyRuleForClusterRule returns policy rules for server clusterrole
func getPolicyRuleForClusterRule() []v1.PolicyRule {
	return []v1.PolicyRule{
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

// deleteClusterRole will delete clusterrole with given name.
func (sr *ServerReconciler) deleteClusterRole(name string) error {
	if err := permissions.DeleteClusterRole(name, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteClusterRole: failed to delete clusterrole %s", name)
	}
	sr.Logger.V(0).Info("clusterrole deleted", "name", name)
	return nil
}
