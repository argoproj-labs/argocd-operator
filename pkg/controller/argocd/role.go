package argocd

import (
	"context"
	"fmt"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	applicationController = "argocd-application-controller"
	server                = "argocd-server"
	redisHa               = "argocd-redis-ha"
	dexServer             = "argocd-dex-server"
)

// newRole returns a new ServiceAccount instance.
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

// newRoleWithName creates a new ServiceAccount with the given name for the given ArgCD.
func newRoleWithName(name string, cr *argoprojv1a1.ArgoCD) *v1.Role {
	sa := newRole(name, cr)
	sa.Name = fmt.Sprintf("%s-%s", cr.Name, name)

	lbls := sa.ObjectMeta.Labels
	lbls[common.ArgoCDKeyName] = name
	sa.ObjectMeta.Labels = lbls

	return sa
}

func (r *ReconcileArgoCD) getClusterRole(name string) (*v1.ClusterRole, error) {
	rbacClient := r.kc.RbacV1()
	return rbacClient.ClusterRoles().Get(context.TODO(), name, metav1.GetOptions{})
}

// reconcileRole
func (r *ReconcileArgoCD) reconcileRole(name string, policyRules []v1.PolicyRule, cr *argoprojv1a1.ArgoCD) (*v1.Role, error) {
	rbacClient := r.kc.RbacV1()

	role := newRoleWithName(name, cr)
	role, err := rbacClient.Roles(cr.Namespace).Get(context.TODO(), role.Name, metav1.GetOptions{})
	roleExists := true
	if err != nil {
		if errors.IsNotFound(err) {
			roleExists = false
			role = newRoleWithName(name, cr)
		} else {
			return nil, err
		}
	}

	role.Rules = policyRules

	controllerutil.SetControllerReference(cr, role, r.scheme)
	if roleExists {
		_, err = rbacClient.Roles(cr.Namespace).Update(context.TODO(), role, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		role, err = rbacClient.Roles(cr.Namespace).Create(context.TODO(), role, metav1.CreateOptions{})
	}
	return role, err
}
