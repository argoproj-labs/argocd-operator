package server

import (
	"reflect"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (sr *ServerReconciler) reconcileClusterRoleBinding() error {
	req := permissions.ClusterRoleBindingRequest{
		ObjectMeta: argoutil.GetObjMeta(clusterResourceName, "", sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     common.ClusterRoleKind,
			Name:     clusterResourceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      resourceName,
				Namespace: sr.Instance.Namespace,
			},
		},
	}

	ignoreDrift := false
	updateFn := func(existing, desired *rbacv1.ClusterRoleBinding, changed *bool) error {
		// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
		if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
			sr.Logger.Debug("detected drift in roleRef for clusterrolebinding", "name", existing.Name, "namespace", existing.Namespace)
			if err := sr.deleteClusterRoleBinding(resourceName); err != nil {
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
	return sr.reconClusterRoleBinding(req, argocdcommon.UpdateFnCrb(updateFn), ignoreDrift)
}

func (sr *ServerReconciler) reconClusterRoleBinding(req permissions.ClusterRoleBindingRequest, updateFn interface{}, ignoreDrift bool) error {
	desired := permissions.RequestClusterRoleBinding(req)

	existing, err := permissions.GetClusterRoleBinding(desired.Name, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconClusterRoleBinding: failed to retrieve ClusterRoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateClusterRoleBinding(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconClusterRoleBinding: failed to create ClusterRoleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}
		sr.Logger.Info("cluster role binding created", "name", desired.Name, "namespace", desired.Namespace)
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

	if err = permissions.UpdateClusterRoleBinding(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconClusterRoleBinding: failed to update ClusterRoleBinding %s", existing.Name)
	}

	sr.Logger.Info("cluster role binding updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteClusterRoleBinding will delete clusterrolebinding with given name.
func (sr *ServerReconciler) deleteClusterRoleBinding(name string) error {
	if err := permissions.DeleteClusterRoleBinding(name, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteClusterRoleBinding: failed to delete clusterrolebinding %s", name)
	}
	sr.Logger.Info("clusterrolebinding deleted", "name", name)
	return nil
}
