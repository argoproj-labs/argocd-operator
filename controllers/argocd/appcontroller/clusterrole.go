package appcontroller

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (acr *AppControllerReconciler) reconcileClusterRole() error {
	req := permissions.ClusterRoleRequest{
		ObjectMeta: argoutil.GetObjMeta(clusterResourceName, "", acr.Instance.Name, acr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		Rules:      getClusterPolicyRules(),
		Client:     acr.Client,
		Mutations:  []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
		Instance:   acr.Instance,
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
	return acr.reconClusterRole(req, argocdcommon.UpdateFnClusterRole(updateFn), ignoreDrift)
}

func (acr *AppControllerReconciler) reconClusterRole(req permissions.ClusterRoleRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := permissions.RequestClusterRole(req)
	if err != nil {
		acr.Logger.Debug("reconClusterRole: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconClusterRole: failed to request ClusterRole %s", desired.Name)
	}

	existing, err := permissions.GetClusterRole(desired.Name, acr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconClusterRole: failed to retrieve ClusterRole %s", desired.Name)
		}

		if err = permissions.CreateClusterRole(desired, acr.Client); err != nil {
			return errors.Wrapf(err, "reconClusterRole: failed to create ClusterRole %s", desired.Name)
		}
		acr.Logger.Info("cluster role created", "name", desired.Name)
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

	if err = permissions.UpdateClusterRole(existing, acr.Client); err != nil {
		return errors.Wrapf(err, "reconClusterRole: failed to update ClusterRole %s", existing.Name)
	}

	acr.Logger.Info("cluster role updated", "name", existing.Name)
	return nil
}

func (acr *AppControllerReconciler) deleteClusterRole(name string) error {
	if err := permissions.DeleteClusterRole(name, acr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		acr.Logger.Error(err, "DeleteClusterRole: failed to delete cluster role", "name", name)
		return err
	}
	acr.Logger.Info("cluster role deleted", "name", name)
	return nil
}

func (acr *AppControllerReconciler) DeleteClusterRoles(clusterRoles []types.NamespacedName) error {
	var deletionErr util.MultiError
	for _, clusterRole := range clusterRoles {
		deletionErr.Append(acr.deleteClusterRole(clusterRole.Name))
	}
	return deletionErr.ErrOrNil()
}

// based off https://github.com/argoproj/argo-cd/blob/master/manifests/install.yaml#L20959
func getClusterPolicyRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			NonResourceURLs: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
	}
}
