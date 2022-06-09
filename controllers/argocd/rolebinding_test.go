package argocd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/argoproj-labs/argocd-operator/common"
)

func TestReconcileArgoCD_reconcileRoleBinding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	p := policyRuleForApplicationController()

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespace(r, "newTestNamespace", a.Namespace))

	workloadIdentifier := "xrb"

	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding := &rbacv1.RoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newTestNamespace"}, roleBinding))

	// update role reference and subject of the rolebinding
	roleBinding.RoleRef.Name = "not-xrb"
	roleBinding.Subjects[0].Name = "not-xrb"
	assert.NoError(t, r.Client.Update(context.TODO(), roleBinding))

	// try reconciling it again and verify if the changes are overwritten
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding = &rbacv1.RoleBinding{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))
}

func TestReconcileArgoCD_reconcileRoleBinding_for_new_namespace(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespace(r, "newTestNamespace", a.Namespace))

	// check no dexServer rolebinding is created for the new namespace with managed-by label
	roleBinding := &rbacv1.RoleBinding{}
	workloadIdentifier := common.ArgoCDDexServerComponent
	expectedDexServerRules := policyRuleForDexServer()
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, expectedDexServerRules, a))
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newTestNamespace"}, roleBinding))

	// check no redisHa rolebinding is created for the new namespace with managed-by label
	workloadIdentifier = common.ArgoCDRedisHAComponent
	expectedRedisHaRules := policyRuleForRedisHa(r.Client)
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, expectedRedisHaRules, a))
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newTestNamespace"}, roleBinding))
}

func TestReconcileArgoCD_reconcileClusterRoleBinding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := "x"
	expectedClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier}}

	assert.NoError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, a))

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	expectedName := fmt.Sprintf("%s-%s-%s", a.Name, a.Namespace, workloadIdentifier)
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))

	// update role reference and subject of the clusterrolebinding
	clusterRoleBinding.RoleRef.Name = "not-x"
	clusterRoleBinding.Subjects[0].Name = "not-x"
	assert.NoError(t, r.Client.Update(context.TODO(), clusterRoleBinding))

	// try reconciling it again and verify if the changes are overwritten
	assert.NoError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, a))

	clusterRoleBinding = &rbacv1.ClusterRoleBinding{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))
}

func TestReconcileArgoCD_reconcileRoleBinding_custom_role(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	p := policyRuleForApplicationController()

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	workloadIdentifier := "argocd-application-controller"
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)

	namespaceWithCustomRole := "namespace-with-custom-role"
	assert.NoError(t, createNamespace(r, namespaceWithCustomRole, a.Namespace))
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	// check if the default rolebindings are created
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, &rbacv1.RoleBinding{}))

	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: namespaceWithCustomRole}, &rbacv1.RoleBinding{}))

	checkForUpdatedRoleRef := func(t *testing.T, roleName, expectedName string) {
		t.Helper()
		expectedRoleRef := rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     roleName,
		}
		roleBinding := &rbacv1.RoleBinding{}
		assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))
		assert.Equal(t, roleBinding.RoleRef, expectedRoleRef)

		assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: namespaceWithCustomRole}, roleBinding))
		assert.Equal(t, roleBinding.RoleRef, expectedRoleRef)
	}

	assert.NoError(t, os.Setenv(common.ArgoCDControllerClusterRoleEnvName, "custom-controller-role"))
	defer os.Unsetenv(common.ArgoCDControllerClusterRoleEnvName)
	assert.NoError(t, r.reconcileRoleBinding(common.ArgoCDApplicationControllerComponent, p, a))

	expectedName = fmt.Sprintf("%s-%s", a.Name, "argocd-application-controller")
	checkForUpdatedRoleRef(t, "custom-controller-role", expectedName)

	assert.NoError(t, os.Setenv(common.ArgoCDServerClusterRoleEnvName, "custom-server-role"))
	defer os.Unsetenv(common.ArgoCDServerClusterRoleEnvName)
	assert.NoError(t, r.reconcileRoleBinding("argocd-server", p, a))

	expectedName = fmt.Sprintf("%s-%s", a.Name, "argocd-server")
	checkForUpdatedRoleRef(t, "custom-server-role", expectedName)
}
