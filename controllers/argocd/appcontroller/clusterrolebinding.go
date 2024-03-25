package appcontroller

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (acr *AppControllerReconciler) reconcileClusterRoleBinding() error {
	req := permissions.ClusterRoleBindingRequest{
		ObjectMeta: argoutil.GetObjMeta(clusterResourceName, "", acr.Instance.Name, acr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     common.ClusterRoleKind,
			Name:     clusterResourceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      resourceName,
				Namespace: acr.Instance.Namespace,
			},
		},
	}

	ignoreDrift := false
	updateFn := func(existing, desired *rbacv1.ClusterRoleBinding, changed *bool) error {
		// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
		if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
			acr.Logger.Debug("detected drift in roleRef for clusterrolebinding", "name", existing.Name, "namespace", existing.Namespace)
			if err := acr.deleteClusterRoleBinding(resourceName); err != nil {
				return errors.Wrapf(err, "reconcileClusterRoleBinding: unable to delete obsolete rolebinding %s", existing.Name)
			}
			return nil
		}

		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Subjects, Desired: &desired.Subjects, ExtraAction: nil},
		}

		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return acr.reconClusterRoleBinding(req, argocdcommon.UpdateFnCrb(updateFn), ignoreDrift)
}

func (acr *AppControllerReconciler) reconClusterRoleBinding(req permissions.ClusterRoleBindingRequest, updateFn interface{}, ignoreDrift bool) error {
	desired := permissions.RequestClusterRoleBinding(req)

	existing, err := permissions.GetClusterRoleBinding(desired.Name, acr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconClusterRoleBinding: failed to retrieve ClusterRoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateClusterRoleBinding(desired, acr.Client); err != nil {
			return errors.Wrapf(err, "reconClusterRoleBinding: failed to create ClusterRoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}
		acr.Logger.Info("cluster role binding created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// ClusterRoleBinding found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnCrb); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconClusterRoleBinding: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = permissions.UpdateClusterRoleBinding(existing, acr.Client); err != nil {
		return errors.Wrapf(err, "reconClusterRoleBinding: failed to update ClusterRoleBinding %s", existing.Name)
	}

	acr.Logger.Info("cluster role binding updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteClusterRoleBinding deletes a ClusterRoleBinding with the given name and namespace.
func (acr *AppControllerReconciler) deleteClusterRoleBinding(name string) error {
	if err := permissions.DeleteClusterRoleBinding(name, acr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		acr.Logger.Error(err, "DeleteClusterRoleBinding: failed to delete ClusterRoleBinding", "name", name)
		return err
	}
	acr.Logger.Info("ClusterRoleBinding deleted", "name", name)
	return nil
}
