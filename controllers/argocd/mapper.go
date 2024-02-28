package argocd

import (
	"context"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
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
			if objs, err := resource.ListObjects(affectedNs.Name, &argoproj.ArgoCDList{}, r.Client, []client.ListOption{
				&client.ListOptions{
					Namespace: nsOpt.PrevManagingNs,
				},
			}); err == nil {
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
	for key, _ := range newManagingNamespaces {
		if objs, err := resource.ListObjects(affectedNs.Name, &argoproj.ArgoCDList{}, r.Client, []client.ListOption{
			&client.ListOptions{
				Namespace: key,
			},
		}); err == nil {
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
	roles, rolebindings := r.getResourcesToBeDeleted(ns, appControllerLS)
	err = r.AppController.DeleteRoles(roles)
	deletionErr.Append(err)

	err = r.AppController.DeleteRoleBindings(rolebindings)
	deletionErr.Append(err)

	// delete appset-controller resources
	roles, rolebindings = r.getResourcesToBeDeleted(ns, appsetControllerLS)
	err = r.AppsetController.DeleteRoles(roles)
	deletionErr.Append(err)

	err = r.AppsetController.DeleteRoleBindings(rolebindings)
	deletionErr.Append(err)

	// delete server-controller resources
	roles, rolebindings = r.getResourcesToBeDeleted(ns, serverControllerLS)
	err = r.ServerController.DeleteRoles(roles)
	deletionErr.Append(err)

	err = r.ServerController.DeleteRoleBindings(rolebindings)
	deletionErr.Append(err)

	return deletionErr.ErrOrNil()

}

func (r *ArgoCDReconciler) getResourcesToBeDeleted(ns string, ls labels.Selector) ([]types.NamespacedName, []types.NamespacedName) {
	roleList, err := permissions.ListRoles(ns, r.Client, []client.ListOption{
		&client.ListOptions{
			Namespace:     ns,
			LabelSelector: ls,
		},
	})
	if err != nil {
		r.Logger.Error(err, "deleteNonControlPlaneResources: failed to list app-controller roles", "namespace", ns)
	}

	rbList, err := permissions.ListRoleBindings(ns, r.Client, []client.ListOption{
		&client.ListOptions{
			Namespace:     ns,
			LabelSelector: ls,
		},
	})
	if err != nil {
		r.Logger.Error(err, "deleteNonControlPlaneResources: failed to list app-controller rolebindings", "namespace", ns)
	}

	roles := []types.NamespacedName{}
	rolebindings := []types.NamespacedName{}

	for _, r := range roleList.Items {
		roles = append(roles, types.NamespacedName{Name: r.Name, Namespace: r.Namespace})
	}

	for _, r := range rbList.Items {
		rolebindings = append(rolebindings, types.NamespacedName{Name: r.Name, Namespace: r.Namespace})
	}

	return roles, rolebindings
}

// getRbacTypeReq returns an rbac type label requirement. The requirement is the existence of the label
// "argocd.argoproj.io/rbac-type", and the value of this key being present in the supplied resourceDeletionLabelValues
func getRbacTypeReq(resourceDeletionLabelValues []string) (*labels.Requirement, error) {
	rbacTypeReq, err := labels.NewRequirement(common.ArgoCDArgoprojKeyRBACType, selection.In, resourceDeletionLabelValues)
	if err != nil {
		return nil, errors.Wrap(err, "getRbacTypeReq: failed to generate label selector")
	}

	return rbacTypeReq, nil
}

// getAppControllerLabelSelector returns a label selector that looks for resources carrying:
//  1. component = application-controller
//     AND
//  2. rbac-type = one of the elements in the supplied rbacTypeReq
func getAppControllerLabelSelector(rbacTypeReq *labels.Requirement) (labels.Selector, error) {
	appControllerComponentReq, err := labels.NewRequirement(common.AppK8sKeyComponent, selection.Equals, []string{common.AppControllerComponent})
	if err != nil {
		return nil, errors.Wrap(err, "getAppControllerLabelSelector: failed to generate label selector")
	}

	return labels.NewSelector().Add(*appControllerComponentReq, *rbacTypeReq), nil
}

// getAppSetControllerLabelSelector returns a label selector that looks for resources carrying:
//  1. component = applicationset-controller
//     AND
//  2. rbac-type = one of the elements in the supplied rbacTypeReq
func getAppSetControllerLabelSelector(rbacTypeReq *labels.Requirement) (labels.Selector, error) {
	appsetControllerComponentReq, err := labels.NewRequirement(common.AppK8sKeyComponent, selection.Equals, []string{common.AppSetControllerComponent})
	if err != nil {
		return nil, errors.Wrap(err, "getAppSetControllerLabelSelector: failed to generate label selector")
	}

	return labels.NewSelector().Add(*appsetControllerComponentReq, *rbacTypeReq), nil
}

// getServerControllerLabelSelector returns a label selector that looks for resources carrying:
//  1. component = server
//     AND
//  2. rbac-type = one of the elements in the supplied rbacTypeReq
func getServerControllerLabelSelector(rbacTypeReq *labels.Requirement) (labels.Selector, error) {
	serverControllerComponentReq, err := labels.NewRequirement(common.AppK8sKeyComponent, selection.Equals, []string{common.ServerComponent})
	if err != nil {
		return nil, errors.Wrap(err, "getServerControllerLabelSelector: failed to generate label selector")
	}

	return labels.NewSelector().Add(*serverControllerComponentReq, *rbacTypeReq), nil
}
