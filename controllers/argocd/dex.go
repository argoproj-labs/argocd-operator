package argocd

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

// reconcileDexResources consolidates all dex resources reconciliation calls. It serves as the single place to trigger both creation
// and deletion of dex resources based on the specified configuration of dex
func (r *ReconcileArgoCD) reconcileDexResources(cr *argoproj.ArgoCD) error {
	if _, err := r.reconcileRole(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		log.Error(err, "error reconciling dex role")
	}

	if err := r.reconcileRoleBinding(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		log.Error(err, "error reconciling dex rolebinding")
	}

	if err := r.reconcileServiceAccountPermissions(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		return err
	}

	// specialized handling for dex
	if err := r.reconcileDexServiceAccount(cr); err != nil {
		log.Error(err, "error reconciling dex serviceaccount")
	}

	// Reconcile dex config in argocd-cm, create dex config in argocd-cm if required (right after dex is enabled)
	if err := r.reconcileArgoConfigMap(cr); err != nil {
		log.Error(err, "error reconciling argocd-cm configmap")
	}

	if err := r.reconcileDexService(cr); err != nil {
		log.Error(err, "error reconciling dex service")
	}

	if err := r.reconcileDexDeployment(cr); err != nil {
		log.Error(err, "error reconciling dex deployment")
	}

	if err := r.reconcileStatusSSO(cr); err != nil {
		log.Error(err, "error reconciling dex status")
	}

	return nil
}

// The code to create/delete notifications resources is written within the reconciliation logic itself. However, these functions must be called
// in the right order depending on whether resources are getting created or deleted. During creation we must create the role and sa first.
// RoleBinding and deployment are dependent on these resouces. During deletion the order is reversed.
// Deployment and RoleBinding must be deleted before the role and sa. deleteDexResources will only be called during
// delete events, so we don't need to worry about duplicate, recurring reconciliation calls
func (r *ReconcileArgoCD) deleteDexResources(cr *argoproj.ArgoCD) error {
	sa := &corev1.ServiceAccount{}
	role := &rbacv1.Role{}

	if err := argoutil.FetchObject(r.Client, cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDDexServerComponent), sa); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	if err := argoutil.FetchObject(r.Client, cr.Namespace, fmt.Sprintf("%s-%s", cr.Name, common.ArgoCDDexServerComponent), role); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}

	if err := r.reconcileDexDeployment(cr); err != nil {
		log.Error(err, "error reconciling dex deployment")
	}

	if err := r.reconcileDexService(cr); err != nil {
		log.Error(err, "error reconciling dex service")
	}

	// Reconcile dex config in argocd-cm (right after dex is disabled)
	// this is required for a one time trigger of reconcileDexConfiguration directly in case of a dex deletion event,
	// since reconcileArgoConfigMap won't call reconcileDexConfiguration once dex has been disabled (to avoid reconciling on
	// dexconfig unnecessarily when it isn't enabled)
	cm := newConfigMapWithName(common.ArgoCDConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.reconcileDexConfiguration(cm, cr); err != nil {
			log.Error(err, "error reconciling dex configuration in configmap")
		}
	}

	if err := r.reconcileRoleBinding(common.ArgoCDDexServerComponent, policyRuleForDexServer(), cr); err != nil {
		log.Error(err, "error reconciling dex rolebinding")
	}

	if err := r.reconcileStatusSSO(cr); err != nil {
		log.Error(err, "error reconciling dex status")
	}

	return nil
}
