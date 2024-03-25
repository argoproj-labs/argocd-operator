package appcontroller

import (
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// deleteRoleBinding deletes a RoleBinding with the given name and namespace.
func (acr *AppControllerReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, acr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		acr.Logger.Error(err, "DeleteRoleBinding: failed to delete RoleBinding", "name", name, "namespace", namespace)
		return err
	}
	acr.Logger.Info("RoleBinding deleted", "name", name, "namespace", namespace)
	return nil
}

// DeleteRoleBindings deletes multiple RoleBindings based on the provided list of NamespacedName.
func (acr *AppControllerReconciler) DeleteRoleBindings(roleBindings []types.NamespacedName) error {
	var deletionErr util.MultiError
	for _, roleBinding := range roleBindings {
		deletionErr.Append(acr.deleteRoleBinding(roleBinding.Name, roleBinding.Namespace))
	}
	return deletionErr.ErrOrNil()
}
