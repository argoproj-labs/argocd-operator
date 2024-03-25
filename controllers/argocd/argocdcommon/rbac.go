package argocdcommon

import (
	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetRBACToBeDeleted(ns string, ls labels.Selector, cl client.Client, logger *util.Logger) ([]types.NamespacedName, []types.NamespacedName) {
	roleList, err := permissions.ListRoles(ns, cl, []client.ListOption{
		&client.ListOptions{
			Namespace:     ns,
			LabelSelector: ls,
		},
	})
	if err != nil {
		logger.Error(err, "getResourcesToBeDeleted: failed to list roles wtih label selector", "namespace", ns, "label selector", ls)
	}

	rbList, err := permissions.ListRoleBindings(ns, cl, []client.ListOption{
		&client.ListOptions{
			Namespace:     ns,
			LabelSelector: ls,
		},
	})
	if err != nil {
		logger.Error(err, "getResourcesToBeDeleted: failed to list rolebindings wtih label selector", "namespace", ns, "label selector", ls)
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
