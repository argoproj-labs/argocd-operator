package argocd

import (
	"context"
	"fmt"
	"strings"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	applicationController = "argocd-application-controller"
	server                = "argocd-server"
	redisHa               = "argocd-redis-ha"
	dexServer             = "argocd-dex-server"
)

// newRole returns a new Role instance.
func newRole(name string, cr *argoprojv1a1.ArgoCD) *v1.Role {
	return &v1.Role{

		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    labelsForCluster(cr),
		},
	}
}

func generateResourceName(argoComponentName string, cr *argoprojv1a1.ArgoCD) string {
	return cr.Name + "-" + argoComponentName
}

// newRoleWithName creates a new Role with the given name for the given ArgoCD.
func newRoleWithName(name string, cr *argoprojv1a1.ArgoCD) *v1.Role {
	sa := newRole(name, cr)
	sa.Name = fmt.Sprintf("%s-%s", cr.Name, name)

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}

func (r *ReconcileArgoCD) getClusterRole(name string) (*v1.ClusterRole, error) {
	clusterRoleList := &v1.ClusterRoleList{}
	err := r.client.List(context.TODO(), clusterRoleList)
	if err != nil {
		return nil, err
	}

	for _, clusterRole := range clusterRoleList.Items {
		if strings.Contains(clusterRole.Name, name) {
			return &clusterRole, nil
		}
	}

	return nil, fmt.Errorf("Unable to find ClusterRole: %s", name)
}

func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.Role, error) {

	role := newRoleWithName(name, cr)
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, role) //rbacClient.Roles(cr.Namespace).Get(context.TODO(), role.Name, metav1.GetOptions{})
	roleExists := true

	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("Failed to reconcile the role for the service account associated with %s : %s", name, err)
		}
		roleExists = false
		role = newRoleWithName(name, cr)
	}

	role.Rules = policyRules

	controllerutil.SetControllerReference(cr, role, r.scheme)
	if roleExists {
		err = r.client.Update(context.TODO(), role)
	} else {
		err = r.client.Create(context.TODO(), role)
	}
	return role, err
}
