package argocd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

func TestReconcileArgoCD_reconcileRoleBinding_dex_disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	rules := policyRuleForDexServer()
	rb := newRoleBindingWithname(dexServer, a)

	// Dex is enabled, creates a role binding
	assert.NoError(t, r.reconcileRoleBinding(dexServer, rules, a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: rb.Name, Namespace: a.Namespace}, rb))

	// Disable Dex, deletes the existing role binding
	os.Setenv("DISABLE_DEX", "true")
	defer os.Unsetenv("DISABLE_DEX")

	_, err := r.reconcileRole(dexServer, rules, a)
	assert.NoError(t, err)
	assert.NoError(t, r.reconcileRoleBinding(dexServer, rules, a))
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: rb.Name, Namespace: a.Namespace}, rb), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: rb.Name, Namespace: a.Namespace}, rb))
	assert.Contains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: rb.Name, Namespace: a.Namespace}, rb).Error(), "not found")
}

func TestReconcileArgoCD_reconcileClusterRoleBinding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := "x"
	expectedClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier}}
	expectedServiceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier, Namespace: a.Namespace}}

	assert.NoError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, expectedServiceAccount, a))

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	expectedName := fmt.Sprintf("%s-%s-%s", a.Name, a.Namespace, workloadIdentifier)
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))

	// update role reference and subject of the clusterrolebinding
	clusterRoleBinding.RoleRef.Name = "not-x"
	clusterRoleBinding.Subjects[0].Name = "not-x"
	assert.NoError(t, r.Client.Update(context.TODO(), clusterRoleBinding))

	// try reconciling it again and verify if the changes are overwritten
	assert.NoError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, expectedServiceAccount, a))

	clusterRoleBinding = &rbacv1.ClusterRoleBinding{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))
}

// This test validates the behavior of the operator reconciliation when a managed namespace is not properly terminated
// or remains terminating may be because of some resources in the namespace not getting deleted.
func TestReconcileRoleBinding_for_Managed_Teminating_Namespace(t *testing.T) {
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespace(r, "managedNS", a.Namespace))

	// Verify role bindings are created for the new namespace with managed-by label
	roleBinding := &rbacv1.RoleBinding{}
	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	expectedRules := policyRuleForApplicationController()
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, expectedRules, a))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS"}, roleBinding))

	// Create a configmap with an invalid finalizer in the "managedNS".
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dummy",
			Namespace: "managedNS",
			Finalizers: []string{
				"nonexistent.finalizer/dummy",
			},
		},
	}
	assert.NoError(t, r.Client.Create(
		context.TODO(), configMap))

	// Delete the newNamespaceTest ns.
	// Verify that operator should not reconcile back to create the roleBindings in terminating ns.
	newNS := &corev1.Namespace{}
	r.Client.Get(context.TODO(), types.NamespacedName{Namespace: "managedNS", Name: "managedNS"}, newNS)
	r.Client.Delete(context.TODO(), newNS)

	// Verify that the namespace exists and is in terminating state.
	r.Client.Get(context.TODO(), types.NamespacedName{Namespace: "managedNS", Name: "managedNS"}, newNS)
	assert.NotEqual(t, newNS.DeletionTimestamp, nil)

	err := r.reconcileRoleBinding(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	// Verify that the role bindings are deleted
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS2"}, roleBinding)
	assert.EqualError(t, err, "rolebindings.rbac.authorization.k8s.io \"argocd-argocd-application-controller\" not found")

	// Create another managed namespace
	assert.NoError(t, createNamespace(r, "managedNS2", a.Namespace))

	// Check if roleBindings are created for the new namespace as well
	err = r.reconcileRoleBinding(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS2"}, roleBinding))
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
	assert.NoError(t, r.reconcileRoleBinding(applicationController, p, a))

	expectedName = fmt.Sprintf("%s-%s", a.Name, "argocd-application-controller")
	checkForUpdatedRoleRef(t, "custom-controller-role", expectedName)

	assert.NoError(t, os.Setenv(common.ArgoCDServerClusterRoleEnvName, "custom-server-role"))
	defer os.Unsetenv(common.ArgoCDServerClusterRoleEnvName)
	assert.NoError(t, r.reconcileRoleBinding("argocd-server", p, a))

	expectedName = fmt.Sprintf("%s-%s", a.Name, "argocd-server")
	checkForUpdatedRoleRef(t, "custom-server-role", expectedName)
}
