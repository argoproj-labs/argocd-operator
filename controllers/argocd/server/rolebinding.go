package server

import (
	"github.com/argoproj-labs/argocd-operator/common"
	appctrl "github.com/argoproj-labs/argocd-operator/controllers/argocd/appcontroller"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"

	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	amerr "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileRoleBindings will ensure all ArgoCD Server rolebindings are present
func (sr *ServerReconciler) reconcileRoleBindings() error {

	sr.Logger.Info("reconciling roleBinding")

	var reconciliationErrors []error

	// get server service account
	saName := getServiceAccountName(sr.Instance.Name)
	sa, err := permissions.GetServiceAccount(saName, sr.Instance.Namespace, sr.Client)
	if err != nil {
		sr.Logger.Error(err, "reconcileRoleBinding: failed to get serviceaccount", "name", saName, "namespace", sr.Instance.Namespace)
		return err
	}

	// reconcile roleBindings for managed namespaces
	if err = sr.reconcileManagedRoleBindings(sa); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	// get application controller service account
	appControllerName := appctrl.GetAppControllerName(sr.Instance.Name)
	appControllerSA, err := permissions.GetServiceAccount(appControllerName, sr.Instance.Namespace, sr.Client)
	if err != nil {
		sr.Logger.Error(err, "reconcileRoleBinding: failed to get serviceaccount", "name", appControllerName, "namespace", sr.Instance.Namespace)
		reconciliationErrors = append(reconciliationErrors, err)
		return amerr.NewAggregate(reconciliationErrors)
	}

	// reconcile roleBindings for source namespaces
	if err = sr.reconcileSourceRoleBindings(sa, appControllerSA); err != nil {
		reconciliationErrors = append(reconciliationErrors, err)
	}

	return amerr.NewAggregate(reconciliationErrors)
}

// deleteRoleBinding will delete rolebinding with given name.
func (sr *ServerReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, sr.Client); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		sr.Logger.Error(err, "DeleteRoleBinding: failed to delete roleBinding", "name", name, "namespace", namespace)
		return err
	}
	sr.Logger.V(0).Info("DeleteRoleBinding: roleBinding deleted", "name", name, "namespace", namespace)
	return nil
}

// reconcileManagedRoleBindings manages rolebindings in ArgoCD managed namespaces
func (sr *ServerReconciler) reconcileManagedRoleBindings(sa *v1.ServiceAccount) error {
	var reconciliationErrors []error

	for nsName := range sr.ManagedNamespaces {

		rbName := getRoleBindingName(sr.Instance.Name)
		rbLabels := common.DefaultResourceLabels(rbName, sr.Instance.Name, ServerControllerComponent)

		_, err := cluster.GetNamespace(nsName, sr.Client)
		if err != nil {
			if !errors.IsNotFound(err) {
				sr.Logger.Error(err, "reconcileSourceRoleBinding: failed to retrieve namespace", "name", nsName)
				reconciliationErrors = append(reconciliationErrors, err)
			}
			continue
		}

		roleBindingRequest := permissions.RoleBindingRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:        rbName,
				Namespace:   nsName,
				Labels:      rbLabels,
				Annotations: sr.Instance.Annotations,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     common.RoleKind,
				Name:     getRoleName(sr.Instance.Name),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      sa.Name,
					Namespace: sa.Namespace,
				},
			},
		}

		// override default role binding if custom role is set
		if getCustomRoleName() != "" {
			roleBindingRequest.RoleRef = rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     common.ClusterRoleKind,
				Name:     getCustomRoleName(),
			}
		}

		desiredRB := permissions.RequestRoleBinding(roleBindingRequest)

		// rolebinding doesn't exist in the namespace, create it
		existingRB, err := permissions.GetRoleBinding(desiredRB.Name, desiredRB.Namespace, sr.Client)
		if err != nil {
			if !errors.IsNotFound(err) {
				sr.Logger.Error(err, "reconcileManagedRoleBindings: failed to retrieve roleBinding", "name", desiredRB.Name, "namespace", desiredRB.Namespace)
				reconciliationErrors = append(reconciliationErrors, err)
				continue
			}

			// Only set ownerReferences for roles in same namespace as ArgoCD CR
			if sr.Instance.Namespace == desiredRB.Namespace {
				if err = controllerutil.SetControllerReference(sr.Instance, desiredRB, sr.Scheme); err != nil {
					sr.Logger.Error(err, "reconcileManagedRoleBindings: failed to set owner reference for role", "name", desiredRB.Name, "namespace", desiredRB.Namespace)
					reconciliationErrors = append(reconciliationErrors, err)
				}
			}

			if err = permissions.CreateRoleBinding(desiredRB, sr.Client); err != nil {
				sr.Logger.Error(err, "reconcileManagedRoleBindings: failed to create roleBinding", "name", desiredRB.Name, "namespace", desiredRB.Namespace)
				reconciliationErrors = append(reconciliationErrors, err)
				continue
			}
			sr.Logger.V(0).Info("reconcileManagedRoleBindings: roleBinding created", "name", desiredRB.Name, "namespace", desiredRB.Namespace)
			continue
		}

		// difference in existing & desired rolebinding, reset it
		rbChanged := false
		fieldsToCompare := []struct {
			existing, desired interface{}
		}{
			{
				&existingRB.RoleRef,
				&desiredRB.RoleRef,
			},
			{
				&existingRB.Subjects,
				&desiredRB.Subjects,
			},
		}

		for _, field := range fieldsToCompare {
			argocdcommon.UpdateIfChanged(field.existing, field.desired, nil, &rbChanged)
		}

		if rbChanged {
			if err = permissions.UpdateRoleBinding(existingRB, sr.Client); err != nil {
				sr.Logger.Error(err, "reconcileManagedRoleBindings: failed to update roleBinding", "name", existingRB.Name, "namespace", existingRB.Namespace)
				reconciliationErrors = append(reconciliationErrors, err)
				continue
			}
			sr.Logger.V(0).Info("reconcileManagedRoleBindings: roleBinding updated", "name", existingRB.Name, "namespace", existingRB.Namespace)
		}

		// rolebinding found, no changes detected
		continue
	}

	return amerr.NewAggregate(reconciliationErrors)
}

// reconcileSourceRoleBindings manages rolebindings for app in namespaces feature
func (sr *ServerReconciler) reconcileSourceRoleBindings(serverSA, appControllerSA *v1.ServiceAccount) error {
	var reconciliationErrors []error

	for nsName := range sr.SourceNamespaces {

		rbName := getRoleBindingNameForSourceNamespace(sr.Instance.Name, nsName)
		rbLabels := common.DefaultResourceLabels(rbName, sr.Instance.Name, ServerControllerComponent)

		ns, err := cluster.GetNamespace(nsName, sr.Client)
		if err != nil {
			if !errors.IsNotFound(err) {
				sr.Logger.Error(err, "reconcileSourceRoleBinding: failed to retrieve namespace", "name", nsName)
				reconciliationErrors = append(reconciliationErrors, err)
			}
			continue
		}

		// do not reconcile rolebindings for namespaces already containing managed-by label
		// as it already contain rolebindings with permissions to manipulate application resources
		// reconciled during reconcilation of ManagedNamespaces
		if _, ok := ns.Labels[common.ArgoCDArgoprojKeyManagedBy]; ok {
			err := sr.deleteRoleBinding(rbName, nsName)
			if err != nil {
				reconciliationErrors = append(reconciliationErrors, err)
			}
			continue
		}

		roleBindingRequest := permissions.RoleBindingRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:        rbName,
				Namespace:   nsName,
				Labels:      rbLabels,
				Annotations: sr.Instance.Annotations,
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     common.RoleKind,
				Name:     getRoleBindingNameForSourceNamespace(sr.Instance.Name, nsName),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      serverSA.Name,
					Namespace: serverSA.Namespace,
				},
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      appControllerSA.Name,
					Namespace: appControllerSA.Namespace,
				},
			},
		}
		desiredRB := permissions.RequestRoleBinding(roleBindingRequest)

		// rolebinding doesn't exist in the namespace, create it
		existingRB, err := permissions.GetRoleBinding(desiredRB.Name, desiredRB.Namespace, sr.Client)
		if err != nil {
			if !errors.IsNotFound(err) {
				sr.Logger.Error(err, "reconcileSourceRoleBinding: failed to retrieve roleBinding", "name", desiredRB.Name, "namespace", desiredRB.Namespace)
				reconciliationErrors = append(reconciliationErrors, err)
				continue
			}

			if err = permissions.CreateRoleBinding(desiredRB, sr.Client); err != nil {
				sr.Logger.Error(err, "reconcileSourceRoleBinding: failed to create roleBinding", "name", desiredRB.Name, "namespace", desiredRB.Namespace)
				reconciliationErrors = append(reconciliationErrors, err)
				continue
			}
			sr.Logger.V(0).Info("reconcileSourceRoleBinding: roleBinding created", "name", desiredRB.Name, "namespace", desiredRB.Namespace)
			continue
		}

		// difference in existing & desired rolebinding, reset it
		rbChanged := false
		fieldsToCompare := []struct {
			existing, desired interface{}
		}{
			{
				&existingRB.RoleRef,
				&desiredRB.RoleRef,
			},
			{
				&existingRB.Subjects,
				&desiredRB.Subjects,
			},
		}

		for _, field := range fieldsToCompare {
			argocdcommon.UpdateIfChanged(field.existing, field.desired, nil, &rbChanged)
		}

		if rbChanged {
			if err = permissions.UpdateRoleBinding(existingRB, sr.Client); err != nil {
				sr.Logger.Error(err, "reconcileSourceRoleBinding: failed to update roleBinding", "name", existingRB.Name, "namespace", existingRB.Namespace)
				reconciliationErrors = append(reconciliationErrors, err)
				continue
			}
			sr.Logger.V(0).Info("reconcileSourceRoleBinding: roleBinding updated", "name", existingRB.Name, "namespace", existingRB.Namespace)
		}

		// rolebinding found, no changes detected
		continue

	}

	return amerr.NewAggregate(reconciliationErrors)
}

// deleteRoleBindings will delete all ArgoCD Server rolebindings
func (sr *ServerReconciler) deleteRoleBindings(argoCDName, namespace string) error {
	var reconciliationErrors []error

	// delete managed ns rolebindings
	for nsName := range sr.ManagedNamespaces {
		err := sr.deleteRoleBinding(getRoleBindingName(argoCDName), nsName)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
		}
	}

	// delete source ns rolebindings
	for nsName := range sr.SourceNamespaces {
		err := sr.deleteRoleBinding(getRoleBindingNameForSourceNamespace(argoCDName, nsName), nsName)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, err)
		}
	}

	return amerr.NewAggregate(reconciliationErrors)
}
