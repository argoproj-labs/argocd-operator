package server

import (
	"fmt"
	"reflect"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileRoleBinding will ensure ArgoCD Server rolebinding is present
func (sr *ServerReconciler) reconcileRoleBinding() error {

	req := permissions.RoleBindingRequest{
		ObjectMeta: argoutil.GetObjMeta(resourceName, sr.Instance.Namespace, sr.Instance.Name, sr.Instance.Namespace, component, util.EmptyMap(), util.EmptyMap()),
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     common.RoleKind,
			Name:     resourceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      resourceName,
				Namespace: sr.Instance.Namespace,
			},
		},
	}

	// override default role binding if custom role is set
	if getCustomRoleName() != "" {
		req.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     common.ClusterRoleKind,
			Name:     getCustomRoleName(),
		}
	}

	desired := permissions.RequestRoleBinding(req)

	if err := controllerutil.SetControllerReference(sr.Instance, desired, sr.Scheme); err != nil {
		sr.Logger.Error(err, "reconcileRoleBinding: failed to set owner reference for roleBinding", "name", desired.Name, "namespace", desired.Namespace)
	}

	// rolebinding doesn't exist in the namespace, create it
	existing, err := permissions.GetRoleBinding(desired.Name, desired.Namespace, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileRoleBinding: failed to retrieve roleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = permissions.CreateRoleBinding(desired, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileRoleBinding: failed to create roleBinding %s in namespace %s", desired.Name, desired.Namespace)
		}

		sr.Logger.Info("role binding created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// if roleRef differs, we must delete the rolebinding as kubernetes does not allow updation of roleRef
	if !reflect.DeepEqual(existing.RoleRef, desired.RoleRef) {
		if err := sr.deleteRoleBinding(resourceName, sr.Instance.Namespace); err != nil {
			return errors.Wrapf(err, "reconcileRoleBinding: unable to delete obsolete rolebinding %s in namespace %s", existing.Name, existing.Namespace)
		}
		// re-trigger reconciliation to create the deleted rolebinding
		return fmt.Errorf("detected drift in roleRef for rolebinding %s, recreating it", existing.Name)
	}

	// difference in existing & desired rolebinding, update it
	changed := false
	fieldsToCompare := []argocdcommon.FieldToCompare{
		{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
		{Existing: &existing.Subjects, Desired: &desired.Subjects, ExtraAction: nil},
	}
	argocdcommon.UpdateIfChanged(fieldsToCompare, &changed)

	// nothing changed
	if !changed {
		return nil
	}

	if err = permissions.UpdateRoleBinding(existing, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileRoleBinding: failed to update roleBinding %s in namespace %s", existing.Name, existing.Namespace)
	}

	sr.Logger.Info("rolebinding updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

// deleteRoleBinding will delete rolebinding with given name.
func (sr *ServerReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoleBinding: failed to delete role %s in namespace %s", name, namespace)
	}
	sr.Logger.Info("roleBinding deleted", "name", name, "namespace", namespace)
	return nil
}
