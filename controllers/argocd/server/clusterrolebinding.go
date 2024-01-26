package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	v1 "k8s.io/api/rbac/v1"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func (sr *ServerReconciler) reconcileClusterRoleBinding() error {

	// ArgoCD instance is not cluster scoped, cleanup any existing clusterrolebindings & exit
	if !sr.ClusterScoped {
		return sr.deleteClusterRoleBinding(uniqueResourceName)
	}

	crbReq := permissions.ClusterRoleBindingRequest{
		ObjectMeta: argoutil.GetObjMeta(uniqueResourceName, "", sr.Instance.Name, sr.Instance.Namespace, component),
		Subjects: []v1.Subject{
			{
				Kind:      v1.ServiceAccountKind,
				Name:      resourceName,	// argocd server sa
				Namespace: sr.Instance.Namespace,
			},
		},
		RoleRef: v1.RoleRef{
			APIGroup: v1.GroupName,
			Kind:     common.ClusterRoleKind,
			Name:     uniqueResourceName,	// clusterrole as same name as clusterrolebinding
		},
	}

	desiredCrb := permissions.RequestClusterRoleBinding(crbReq)

	// clusterrolebinding doesn't exist in the namespace, create it
	existingCrb, err := permissions.GetClusterRoleBinding(desiredCrb.Name, sr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileClusterRoleBinding: failed to retrieve clusterrolebinding %s", desiredCrb.Name)
		}

		if err = permissions.CreateClusterRoleBinding(desiredCrb, sr.Client); err != nil {
			return errors.Wrapf(err, "reconcileClusterRoleBinding: failed to create clusterrolebinding %s", desiredCrb.Name)
		}

		sr.Logger.V(0).Info("clusterrolebinding created", "name", desiredCrb.Name, "namespace", desiredCrb.Namespace)
		return nil
	}

	// difference in existing & desired clusterrolebinding, update it
	changed := false

	fieldsToCompare := []struct {
		existing, desired interface{}
		extraAction       func()
	}{
		{
			&existingCrb.RoleRef,
			&desiredCrb.RoleRef,
			nil,
		},
		{
			&existingCrb.Subjects,
			&desiredCrb.Subjects,
			nil,
		},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
	}

	// nothing changed, exit reconciliation
	if !changed {
		return nil
	}

	if err = permissions.UpdateClusterRoleBinding(existingCrb, sr.Client); err != nil {
		return errors.Wrapf(err, "reconcileClusterRoleBinding: failed to update clusterrolebinding %s", existingCrb.Name)
	}

	sr.Logger.V(0).Info("clusterrolebinding updated", "name", existingCrb.Name)
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
	sr.Logger.V(0).Info("clusterrolebinding deleted", "name", name)
	return nil
}
