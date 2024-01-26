package server

import (
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

// reconcileRoleBindings will ensure all ArgoCD Server rolebindings are present
func (sr *ServerReconciler) reconcileRoleBindings() error {
	var reconErrs util.MultiError

	// reconcile roleBindings for managed namespaces
	if err := sr.reconcileManagedRoleBindings(); err != nil {
		reconErrs.Append(err)
	}

	// reconcile roleBindings for source namespaces
	if err := sr.reconcileSourceRoleBindings(); err != nil {
		reconErrs.Append(err)
	}

	return reconErrs.ErrOrNil()
}

// deleteRoleBinding will delete rolebinding with given name.
func (sr *ServerReconciler) deleteRoleBinding(name, namespace string) error {
	if err := permissions.DeleteRoleBinding(name, namespace, sr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteRoleBinding: failed to delete role %s in namespace %s", name, namespace)
	}
	sr.Logger.V(0).Info("rolebinding deleted", "name", name, "namespace", namespace)
	return nil
}

// reconcileManagedRoleBindings manages rolebindings in ArgoCD managed namespaces
func (sr *ServerReconciler) reconcileManagedRoleBindings() error {
	var reconErrs util.MultiError

	for nsName := range sr.ManagedNamespaces {

		rbReq := permissions.RoleBindingRequest{
			ObjectMeta: argoutil.GetObjMeta(resourceName, nsName, sr.Instance.Name, sr.Instance.Namespace, component),
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     common.RoleKind,
				Name:     resourceName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      resourceName,	// argocd server sa
					Namespace: sr.Instance.Namespace,
				},
			},
		}

		// override default role binding if custom role is set
		if getCustomRoleName() != "" {
			rbReq.RoleRef = rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     common.ClusterRoleKind,
				Name:     getCustomRoleName(),
			}
		}

		// for non control-plane namespace, add special resource management label to rolebinding
		if nsName != sr.Instance.Namespace {
			if len(rbReq.ObjectMeta.Labels) == 0 {
				rbReq.ObjectMeta.Labels = make(map[string]string)
			}
			rbReq.ObjectMeta.Labels[common.ArgoCDKeyRBACType] = common.ArgoCDRBACTypeResourceMananagement
		}

		desiredRb := permissions.RequestRoleBinding(rbReq)

		// Only set ownerReferences for roles in same namespace as ArgoCD CR
		if sr.Instance.Namespace == desiredRb.Namespace {
			if err := controllerutil.SetControllerReference(sr.Instance, desiredRb, sr.Scheme); err != nil {
				sr.Logger.Error(err, "reconcileManagedRoleBindings: failed to set owner reference for rolebinding", "name", desiredRb.Name, "namespace", desiredRb.Namespace)
			}
		}

		// rolebinding doesn't exist in the namespace, create it
		existingRb, err := permissions.GetRoleBinding(desiredRb.Name, desiredRb.Namespace, sr.Client)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				reconErrs.Append(errors.Wrapf(err, "reconcileManagedRoleBindings: failed to retrieve rolebinding %s in namespace %s", desiredRb.Name, desiredRb.Namespace))
				continue
			}

			if err = permissions.CreateRoleBinding(desiredRb, sr.Client); err != nil {
				reconErrs.Append(errors.Wrapf(err, "reconcileManagedRoleBindings: failed to create rolebinding %s in namespace %s", desiredRb.Name, desiredRb.Namespace))
				continue
			}

			sr.Logger.V(0).Info("rolebinding created", "name", desiredRb.Name, "namespace", desiredRb.Namespace)
			continue
		}

		// difference in existing & desired rolebinding, update it
		changed := false

		fieldsToCompare := []struct {
			existing, desired interface{}
			extraAction       func()
		}{
			{
				&existingRb.RoleRef,
				&desiredRb.RoleRef,
				nil,
			},
			{
				&existingRb.Subjects,
				&desiredRb.Subjects,
				nil,
			},
		}

		for _, field := range fieldsToCompare {
			argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
		}
	
		// nothing changed
		if !changed {
			continue
		}
	
		if err = permissions.UpdateRoleBinding(existingRb, sr.Client); err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileManagedRoleBindings: failed to update rolebinding %s in namespace %s", existingRb.Name, existingRb.Namespace))
			continue
		}

		sr.Logger.V(0).Info("rolebinding updated", "name", existingRb.Name, "namespace", existingRb.Namespace)
		continue
	}

	return reconErrs.ErrOrNil()
}

// reconcileSourceRoleBindings manages rolebindings for app in namespaces feature
func (sr *ServerReconciler) reconcileSourceRoleBindings() error {
	var reconErrs util.MultiError

	for nsName := range sr.SourceNamespaces {

		rbReq := permissions.RoleBindingRequest{
			ObjectMeta: argoutil.GetObjMeta(uniqueResourceName, nsName, sr.Instance.Name, sr.Instance.Namespace, component),
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     common.RoleKind,
				Name:     uniqueResourceName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      resourceName,	// argocd server sa
					Namespace: sr.Instance.Namespace,
				},
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      appcontrollerResourceName,	// argocd appcontroller sa
					Namespace: sr.Instance.Namespace,
				},
			},
		}

		// for non control-plane namespace, add special app management label to rolebinding
		if nsName != sr.Instance.Namespace {
			if len(rbReq.ObjectMeta.Labels) == 0 {
				rbReq.ObjectMeta.Labels = make(map[string]string)
			}
			rbReq.ObjectMeta.Labels[common.ArgoCDKeyRBACType] = common.ArgoCDRBACTypeAppManagement
		}

		desiredRb := permissions.RequestRoleBinding(rbReq)

		// rolebinding doesn't exist in the namespace, create it
		existingRb, err := permissions.GetRoleBinding(desiredRb.Name, desiredRb.Namespace, sr.Client)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				reconErrs.Append(errors.Wrapf(err, "reconcileSourceRoleBindings: failed to retrieve rolebinding %s in namespace %s", desiredRb.Name, desiredRb.Namespace))
                continue
			}

			if err = permissions.CreateRoleBinding(desiredRb, sr.Client); err != nil {
				reconErrs.Append(errors.Wrapf(err, "reconcileSourceRoleBindings: failed to create rolebinding %s in namespace %s", desiredRb.Name, desiredRb.Namespace))
                continue
			}

			sr.Logger.V(0).Info("rolebinding created", "name", desiredRb.Name, "namespace", desiredRb.Namespace)
			continue
		}

		// difference in existing & desired rolebinding, update it
		changed := false

		fieldsToCompare := []struct {
			existing, desired interface{}
			extraAction       func()
		}{
			{
				&existingRb.RoleRef,
				&desiredRb.RoleRef,
				nil,
			},
			{
				&existingRb.Subjects,
				&desiredRb.Subjects,
				nil,
			},
		}

		for _, field := range fieldsToCompare {
			argocdcommon.UpdateIfChanged(field.existing, field.desired, field.extraAction, &changed)
		}
	
		// nothing changed
		if !changed {
			continue
		}
	
		if err = permissions.UpdateRoleBinding(existingRb, sr.Client); err != nil {
			reconErrs.Append(errors.Wrapf(err, "reconcileSourceRoleBindings: failed to update rolebinding %s in namespace %s", existingRb.Name, existingRb.Namespace))
			continue
		}

		sr.Logger.V(0).Info("rolebinding updated", "name", existingRb.Name, "namespace", existingRb.Namespace)
		continue
	}

	return reconErrs.ErrOrNil()
}

// deleteRoleBindings will delete all ArgoCD Server rolebindings
func (sr *ServerReconciler) deleteRoleBindings(mngNsRoleName, srcNsRoleName string) error {
	var reconErrs util.MultiError

	// delete managed ns rolebindings
	for nsName := range sr.ManagedNamespaces {
		err := sr.deleteRoleBinding(mngNsRoleName, nsName)
		if err != nil {
			reconErrs.Append(err)
		}
	}

	// delete source ns rolebindings
	for nsName := range sr.SourceNamespaces {
		err := sr.deleteRoleBinding(srcNsRoleName, nsName)
		if err != nil {
			reconErrs.Append(err)
		}
	}

	return reconErrs.ErrOrNil()
}
