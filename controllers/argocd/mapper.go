package argocd

import (
	"context"

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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

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

		// remove any labels from terminating namespaces
		delete(affectedNs.Labels, common.ArgoCDArgoprojKeyManagedBy)
		delete(affectedNs.Labels, common.ArgoCDArgoprojKeyAppsManagedBy)
		delete(affectedNs.Labels, common.ArgoCDArgoprojKeyAppSetsManagedBy)

		if err := cluster.UpdateNamespace(affectedNs, r.Client); err != nil {
			r.Logger.Error(err, "namespaceMapper: failed to update terminating namespace", "name", affectedNs.Name)
		}
	}

	if _, ok := ScheduledForRBACDeletion[affectedNs.Name]; !ok {
		// namespace does not need any resource deletion. nothing to do
		return result
	}

	nsOpts := ScheduledForRBACDeletion[affectedNs.Name]

	resourceDeletionLabelVals := map[string]string{}
	newManagingNamespaces := map[string]string{}

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

	// trigger reconciliation for new managing namespaces so required resources are created for those instances
	for newManagingNs, _ := range newManagingNamespaces {
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
