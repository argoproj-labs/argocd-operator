package server

import (
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (sr *ServerReconciler) deleteRole(name, namespace string) error {
	if err := permissions.DeleteRole(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		sr.Logger.Error(err, "DeleteRole: failed to delete role", "name", name, "namespace", namespace)
		return err
	}
	sr.Logger.Info("role deleted", "name", name, "namespace", namespace)
	return nil
}

func (sr *ServerReconciler) DeleteRoles(roles []types.NamespacedName) error {
	var deletionErr util.MultiError
	for _, role := range roles {
		deletionErr.Append(sr.deleteRole(role.Name, role.Namespace))
	}
	return deletionErr.ErrOrNil()
}
