package argocd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileRoleBinding(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	p := policyRuleForApplicationController()

	workloadIdentifier := "xrb"

	assert.NilError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding := &rbacv1.RoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))

	// update role reference and subject of the rolebinding
	roleBinding.RoleRef.Name = "not-xrb"
	roleBinding.Subjects[0].Name = "not-xrb"
	assert.NilError(t, r.client.Update(context.TODO(), roleBinding))

	// try reconciling it again and verify if the changes are overwritten
	assert.NilError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding = &rbacv1.RoleBinding{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))
}

func TestReconcileArgoCD_reconcileRoleBinding_dex_disabled(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	rules := policyRuleForDexServer()
	rb := newRoleBindingWithname(dexServer, a)

	// Dex is enabled, creates a role binding
	assert.NilError(t, r.reconcileRoleBinding(dexServer, rules, a))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: rb.Name, Namespace: a.Namespace}, rb))

	// Disable Dex, deletes the existing role binding
	os.Setenv("DISABLE_DEX", "true")
	defer os.Unsetenv("DISABLE_DEX")

	assert.NilError(t, r.reconcileRoleBinding(dexServer, rules, a))
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: rb.Name, Namespace: a.Namespace}, rb), "not found")
}

func TestReconcileArgoCD_reconcileClusterRoleBinding(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := "x"
	expectedClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier}}
	expectedServiceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier, Namespace: a.Namespace}}

	assert.NilError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, expectedServiceAccount, a))

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	expectedName := fmt.Sprintf("%s-%s-%s", a.Name, a.Namespace, workloadIdentifier)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))

	// update role reference and subject of the clusterrolebinding
	clusterRoleBinding.RoleRef.Name = "not-x"
	clusterRoleBinding.Subjects[0].Name = "not-x"
	assert.NilError(t, r.client.Update(context.TODO(), clusterRoleBinding))

	// try reconciling it again and verify if the changes are overwritten
	assert.NilError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, expectedServiceAccount, a))

	clusterRoleBinding = &rbacv1.ClusterRoleBinding{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))
}

func TestReconcileArgoCD_reconcileRoleBinding_application_controller(t *testing.T) {
	// tests reconciler hook for argocd-application-controller role
	cr := makeTestArgoCD()
	r := makeTestReconciler(t, cr)

	defer resetHooks()()
	Register(testRoleBindingHook)

	roleBinding := newRoleBindingWithname(applicationController, cr)
	assert.NilError(t, r.reconcileRoleBinding(applicationController, policyRuleForApplicationController(), cr))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, roleBinding))
	assert.DeepEqual(t, roleBinding.RoleRef.Name, "test-admin-role")
}
