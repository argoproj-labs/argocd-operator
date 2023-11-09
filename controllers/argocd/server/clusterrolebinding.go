package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (sr *ServerReconciler) reconcileClusterRoleBinding() error {

	crbName := getClusterRoleBindingName(sr.Instance.Name, sr.Instance.Namespace)
	crbLables := common.DefaultLabels(crbName, sr.Instance.Name, ServerControllerComponent)

	// ArgoCD instance is not cluster scoped, cleanup any existing cluster rolebindings & exit
	if !sr.ClusterScoped {
		return sr.deleteClusterRoleBinding(crbName)
	}

	sr.Logger.Info("reconciling clusterRoleBinding")

	// get server service account
	saName := getServiceAccountName(sr.Instance.Name)
	saNS := sr.Instance.Namespace
	sa, err := permissions.GetServiceAccount(saName, saNS, sr.Client)
	if err != nil {
		sr.Logger.Error(err, "reconcileClusterRoleBinding: failed to get serviceaccount", "name", saName, "namespace", saNS)
		return err
	}

	crbRequest := permissions.ClusterRoleBindingRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:        crbName,
			Labels:      crbLables,
			Annotations: sr.Instance.Annotations,
		},
	}

	crbRequest.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}
	crbRequest.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     common.ClusterRoleKind,
		Name:     getClusterRoleName(sr.Instance.Name, sr.Instance.Namespace),
	}


	desiredCRB := permissions.RequestClusterRoleBinding(crbRequest)

	// cluster rolebinding doesn't exist in the namespace, create it
	existingCRB, err := permissions.GetClusterRoleBinding(desiredCRB.Name, sr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			sr.Logger.Error(err, "reconcileClusterRoleBinding: failed to retrieve cluster rolebinding", "name", desiredCRB.Name, "namespace", desiredCRB.Namespace)
			return err
		}

		if err = permissions.CreateClusterRoleBinding(desiredCRB, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileClusterRoleBinding: failed to create cluster rolebinding", "name", desiredCRB.Name, "namespace", desiredCRB.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileClusterRoleBinding: cluster rolebinding created", "name", desiredCRB.Name, "namespace", desiredCRB.Namespace)
		return nil
	}

	// difference in existing & desired cluster rolebinding, reset it
	rbChanged := false
	fieldsToCompare := []struct {
		existing, desired interface{}
	}{
		{
			&existingCRB.RoleRef,
			&desiredCRB.RoleRef,
		},
		{
			&existingCRB.Subjects,
			&desiredCRB.Subjects,
		},
	}

	for _, field := range fieldsToCompare {
		argocdcommon.UpdateIfChanged(field.existing, field.desired, nil, &rbChanged)
	}

	if rbChanged {
		if err = permissions.UpdateClusterRoleBinding(existingCRB, sr.Client); err != nil {
			sr.Logger.Error(err, "reconcileClusterRoleBinding: failed to update cluster rolebinding", "name", existingCRB.Name, "namespace", existingCRB.Namespace)
			return err
		}
		sr.Logger.V(0).Info("reconcileClusterRoleBinding: cluster rolebinding updated", "name", existingCRB.Name, "namespace", existingCRB.Namespace)
	}
	
	// cluster rolebinding found, no changes detected
	return nil
}

// deleteClusterRole will delete cluster rolebinding with given name.
func (sr *ServerReconciler) deleteClusterRoleBinding(name string) error {
	if err := permissions.DeleteClusterRoleBinding(name, sr.Client); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		sr.Logger.Error(err, "reconcileClusterRoleBinding: failed to delete role", "name", name)
		return err
	}
	sr.Logger.V(0).Info("reconcileClusterRoleBinding: role deleted", "name", name)
	return nil
}