package argocd

import (
	"context"
	"reflect"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// clusterResourceMapper maps a watch event on a cluster scoped resource back to the
// ArgoCD object that we want to reconcile.
func (r *ArgoCDReconciler) clusterResourceMapper(ctx context.Context, o client.Object) []reconcile.Request {
	namespacedArgoCDObject := client.ObjectKey{}

	for k, v := range o.GetAnnotations() {
		if k == common.ArgoCDArgoprojKeyName {
			namespacedArgoCDObject.Name = v
		} else if k == common.ArgoCDArgoprojKeyNamespace {
			namespacedArgoCDObject.Namespace = v
		}
	}

	var result = []reconcile.Request{}
	if namespacedArgoCDObject.Name != "" && namespacedArgoCDObject.Namespace != "" {
		result = []reconcile.Request{
			{NamespacedName: namespacedArgoCDObject},
		}
	}
	return result
}

// tlsSecretMapper maps a watch event on a secret of type TLS back to the
// ArgoCD object that we want to reconcile.
func (r *ArgoCDReconciler) tlsSecretMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	if !argocdcommon.IsSecretOfInterest(o) {
		return result
	}
	namespacedArgoCDObject := client.ObjectKey{}

	owner, err := argocdcommon.FindSecretOwnerInstance(types.NamespacedName{Name: o.GetName(), Namespace: o.GetNamespace()}, r.Client)
	if err != nil {
		r.Logger.Error(err, "tlsSecretMapper: failed to find secret owner instance")
		return result
	}

	if !reflect.DeepEqual(owner, types.NamespacedName{}) {
		result = []reconcile.Request{
			{NamespacedName: namespacedArgoCDObject},
		}
	}

	return result
}

// clusterSecretMapper maps a watch event on an Argo CD cluster secret back to the
// ArgoCD object that we want to reconcile.
func (r *ArgoCDReconciler) clusterSecretMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	labels := o.GetLabels()
	if v, ok := labels[common.ArgoCDArgoprojKeySecretType]; ok && v == common.ArgoCDSecretTypeCluster {
		argocds := &argoproj.ArgoCDList{}
		if err := r.Client.List(context.TODO(), argocds, &client.ListOptions{Namespace: o.GetNamespace()}); err != nil {
			return result
		}

		if len(argocds.Items) != 1 {
			return result
		}

		argocd := argocds.Items[0]
		namespacedName := client.ObjectKey{
			Name:      argocd.Name,
			Namespace: argocd.Namespace,
		}
		result = []reconcile.Request{
			{NamespacedName: namespacedName},
		}
	}

	return result
}

// applicationSetSCMTLSConfigMapMapper maps a watch event on a configmap with name "argocd-appset-gitlab-scm-tls-certs-cm",
// back to the ArgoCD object that we want to reconcile.
func (r *ArgoCDReconciler) applicationSetSCMTLSConfigMapMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	if o.GetName() == common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName {
		argocds := &argoproj.ArgoCDList{}
		if err := r.Client.List(context.TODO(), argocds, &client.ListOptions{Namespace: o.GetNamespace()}); err != nil {
			return result
		}

		if len(argocds.Items) != 1 {
			return result
		}

		argocd := argocds.Items[0]
		namespacedName := client.ObjectKey{
			Name:      argocd.Name,
			Namespace: argocd.Namespace,
		}
		result = []reconcile.Request{
			{NamespacedName: namespacedName},
		}
	}

	return result
}

// namespaceMapper maps a watch event on a namespace, back to the
// ArgoCD object that we want to reconcile.
func (r *ArgoCDReconciler) namespaceMapper(ctx context.Context, obj client.Object) []reconcile.Request {
	lock.Lock()
	defer lock.Unlock()

	var result = []reconcile.Request{}
	affectedNs := obj.(*corev1.Namespace)

	if affectedNs.GetDeletionTimestamp() != nil {
		// check if terminating namespace contains Argo CD instance to be deleted
		if objs, err := resource.ListObjects(affectedNs.Name, &argoproj.ArgoCDList{}, r.Client, []client.ListOption{}); err == nil {
			if instances, ok := objs.(*argoproj.ArgoCDList); ok {
				if len(instances.Items) > 0 {
					argocd := instances.Items[0]

					// delete instance and trigger reconciliation for resource cleanup
					if err := resource.DeleteObject(argocd.Name, argocd.Namespace, &argocd, r.Client); err != nil {
						r.Logger.Error(err, "namespaceMapper: failed to delete Argo CD instance", "name", argocd.Name)
					}

					namespacedName := client.ObjectKey{
						Name:      argocd.Name,
						Namespace: argocd.Namespace,
					}
					result = []reconcile.Request{
						{NamespacedName: namespacedName},
					}
				}
				// if Argo CD instance is present the namespace won't carry any of the managed-by labels,
				// so we can return here
				return result
			}
		}

		// remove any labels from terminating namespaces if they exist
		delete(affectedNs.Labels, common.ArgoCDArgoprojKeyManagedBy)
		delete(affectedNs.Labels, common.ArgoCDArgoprojKeyAppsManagedBy)
		delete(affectedNs.Labels, common.ArgoCDArgoprojKeyAppSetsManagedBy)

		if err := cluster.UpdateNamespace(affectedNs, r.Client); err != nil {
			r.Logger.Error(err, "namespaceMapper: failed to update terminating namespace", "name", affectedNs.Name)
		}
	}

	newManagingNamespaces := map[string]string{}

	if _, ok := ScheduledForRBACDeletion[affectedNs.Name]; ok {
		// some kind of clean up is required in affected namespace
		resourceDeletionLabelVals := map[string]string{}

		nsOpts := ScheduledForRBACDeletion[affectedNs.Name]

		// consolidate nsOpts by field in maps, to track unique rbac-type label values
		for _, nsOpt := range nsOpts {
			resourceDeletionLabelVals[nsOpt.ResourceDeletionLabelValue] = ""
			newManagingNamespaces[nsOpt.NewManagingNs] = ""
		}

		// delete roles and rolebindings from affected namespace
		err := r.deleteNonControlPlaneResources(affectedNs.Name, util.StringMapKeys(resourceDeletionLabelVals), nsOpts)
		if err != nil {
			r.Logger.Error(err, "namespaceMapper: failed to delete resources", "namespace", affectedNs.Name)
		}

		// delete namespace from previously managing instance's cluster secret by triggering reconciliation of instance
		for _, nsOpt := range nsOpts {
			if nsOpt.ResourceDeletionLabelValue == common.ArgoCDRBACTypeResourceMananagement {
				// get previously managing instance and queue it for reconciliation
				if objs, err := resource.ListObjects(nsOpt.PrevManagingNs, &argoproj.ArgoCDList{}, r.Client, []client.ListOption{}); err == nil {
					if instances, ok := objs.(*argoproj.ArgoCDList); ok {
						if len(instances.Items) > 0 {
							argocd := instances.Items[0]

							namespacedName := client.ObjectKey{
								Name:      argocd.Name,
								Namespace: argocd.Namespace,
							}
							result = append(result, reconcile.Request{
								NamespacedName: namespacedName,
							})
						}
					}
				}
			}
		}
	} else {
		// no clean up required, but we may have a newly managed namespace that needs to be reconciled. Add any managing Namespaces to the set
		// of namespaces to be reconciled
		if val, ok := affectedNs.Labels[common.ArgoCDArgoprojKeyManagedBy]; ok {
			newManagingNamespaces[val] = ""
		}

		if val, ok := affectedNs.Labels[common.ArgoCDArgoprojKeyAppsManagedBy]; ok {
			newManagingNamespaces[val] = ""
		}

		if val, ok := affectedNs.Labels[common.ArgoCDArgoprojKeyAppSetsManagedBy]; ok {
			newManagingNamespaces[val] = ""
		}
	}

	// trigger reconciliation for new managing namespaces so required resources are created for those instances
	for newManagingNs := range newManagingNamespaces {
		if objs, err := resource.ListObjects(newManagingNs, &argoproj.ArgoCDList{}, r.Client, []client.ListOption{}); err == nil {
			if instances, ok := objs.(*argoproj.ArgoCDList); ok {
				if len(instances.Items) > 0 {
					argocd := instances.Items[0]

					namespacedName := client.ObjectKey{
						Name:      argocd.Name,
						Namespace: argocd.Namespace,
					}
					result = append(result, reconcile.Request{
						NamespacedName: namespacedName,
					})
				}
			}
		}
	}

	return result
}

// deleteNonControlPlaneResources deletes roles and rolebindings from a namespace that is either
// 1. no longer managed by an Argo CD instance
// 2. managed by a different Argo CD instance
// 3. About to be deleted
// This includes roles and rolebindings for app-controller, server and appset-controller, for resource management, app management and appset-management
// The roles and rolebindings to be deleted are determined by which labels were removed/modified for the namespace in question
func (r *ArgoCDReconciler) deleteNonControlPlaneResources(ns string, resourceDeletionLabelVals []string, nsOpts []ManagedNsOpts) error {
	rbacTypeReq, err := getRbacTypeReq(resourceDeletionLabelVals)
	if err != nil {
		return err
	}

	appControllerLS, err := getAppControllerLabelSelector(rbacTypeReq)
	if err != nil {
		return err
	}

	appsetControllerLS, err := getAppSetControllerLabelSelector(rbacTypeReq)
	if err != nil {
		return err
	}

	serverControllerLS, err := getServerControllerLabelSelector(rbacTypeReq)
	if err != nil {
		return err
	}

	var deletionErr util.MultiError

	// delete app-controller resources
	roles, rolebindings := argocdcommon.GetRBACToBeDeleted(ns, appControllerLS, r.Client, r.Logger)
	err = r.AppController.DeleteRoles(roles)
	deletionErr.Append(err)

	err = r.AppController.DeleteRoleBindings(rolebindings)
	deletionErr.Append(err)

	// delete appset-controller resources
	roles, rolebindings = argocdcommon.GetRBACToBeDeleted(ns, appsetControllerLS, r.Client, r.Logger)
	err = r.AppsetController.DeleteRoles(roles)
	deletionErr.Append(err)

	err = r.AppsetController.DeleteRoleBindings(rolebindings)
	deletionErr.Append(err)

	// delete server-controller resources
	roles, rolebindings = argocdcommon.GetRBACToBeDeleted(ns, serverControllerLS, r.Client, r.Logger)
	err = r.ServerController.DeleteRoles(roles)
	deletionErr.Append(err)

	err = r.ServerController.DeleteRoleBindings(rolebindings)
	deletionErr.Append(err)

	return deletionErr.ErrOrNil()

}

// getRbacTypeReq returns an rbac type label requirement. The requirement is the existence of the label
// "argocd.argoproj.io/rbac-type", and the value of this key being present in the supplied resourceDeletionLabelValues
func getRbacTypeReq(resourceDeletionLabelValues []string) (*labels.Requirement, error) {
	rbacTypeReq, err := argocdcommon.GetLabelRequirements(common.ArgoCDArgoprojKeyRBACType, selection.In, resourceDeletionLabelValues)
	if err != nil {
		return nil, errors.Wrap(err, "getRbacTypeReq: failed to generate requirement")
	}
	return rbacTypeReq, nil
}

// getAppControllerLabelSelector returns a label selector that looks for resources carrying:
//  1. component = application-controller
//     AND
//  2. rbac-type = one of the elements in the supplied rbacTypeReq
func getAppControllerLabelSelector(rbacTypeReq *labels.Requirement) (labels.Selector, error) {
	appControllerComponentReq, err := argocdcommon.GetLabelRequirements(common.AppK8sKeyComponent, selection.Equals, []string{common.AppControllerComponent})
	if err != nil {
		return nil, errors.Wrap(err, "getAppControllerLabelSelector: failed to generate label selector")
	}
	appControllerLS := argocdcommon.GetLabelSelector(*rbacTypeReq, *appControllerComponentReq)
	return appControllerLS, nil
}

// getAppSetControllerLabelSelector returns a label selector that looks for resources carrying:
//  1. component = applicationset-controller
//     AND
//  2. rbac-type = one of the elements in the supplied rbacTypeReq
func getAppSetControllerLabelSelector(rbacTypeReq *labels.Requirement) (labels.Selector, error) {
	appsetControllerComponentReq, err := argocdcommon.GetLabelRequirements(common.AppK8sKeyComponent, selection.Equals, []string{common.AppSetControllerComponent})
	if err != nil {
		return nil, errors.Wrap(err, "getAppSetControllerLabelSelector: failed to generate label selector")
	}
	appsetControllerLS := argocdcommon.GetLabelSelector(*rbacTypeReq, *appsetControllerComponentReq)
	return appsetControllerLS, nil
}

// getServerControllerLabelSelector returns a label selector that looks for resources carrying:
//  1. component = server
//     AND
//  2. rbac-type = one of the elements in the supplied rbacTypeReq
func getServerControllerLabelSelector(rbacTypeReq *labels.Requirement) (labels.Selector, error) {
	serverControllerComponentReq, err := argocdcommon.GetLabelRequirements(common.AppK8sKeyComponent, selection.Equals, []string{common.ServerComponent})
	if err != nil {
		return nil, errors.Wrap(err, "getServerControllerLabelSelector: failed to generate label selector")
	}
	serverControllerLS := argocdcommon.GetLabelSelector(*rbacTypeReq, *serverControllerComponentReq)
	return serverControllerLS, nil
}
