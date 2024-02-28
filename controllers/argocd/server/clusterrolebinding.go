package server

import (
	"fmt"
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

	// ArgoCD instance is not cluster scoped, cleanup any existing clusterrolebindings & exit
	if !sr.ClusterScoped {
		return sr.deleteClusterRoleBinding(clusterResourceName)
	}

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

	desired := permissions.RequestClusterRoleBinding(req)

	// clusterrolebinding doesn't exist in the namespace, create it
	existing, err := permissions.GetClusterRoleBinding(desired.Name, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileClusterRoleBinding: failed to retrieve clusterrolebinding %s", desired.Name)
		}

		if err = permissions.CreateClusterRoleBinding(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileClusterRoleBinding: failed to create clusterrolebinding %s", desired.Name)
		}

		sr.Logger.Info("clusterrolebinding created", "name", desired.Name)
		return nil
	}

	// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
	if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
		if err := sr.deleteClusterRoleBinding(resourceName); err != nil {
			return errors.Wrapf(err, "reconcileClusterRoleBinding: unable to delete obsolete clusterrolebinding %s", existing.Name)
		}
		// re-trigger reconciliation to create the deleted clusterrolebinding
		return fmt.Errorf("detected drift in roleRef for clusterrolebinding %s, recreating it", existing.Name)
	}

	// difference in existing & desired clusterrolebinding, update it
	changed := false
	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Subjects, Desired: &desired.Subjects, ExtraAction: nil},
	}
	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = permissions.UpdateClusterRoleBinding(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileClusterRoleBinding: failed to update clusterrolebinding %s", existing.Name)
	}

	sr.Logger.Info("clusterrolebinding updated", "name", existing.Name)
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
