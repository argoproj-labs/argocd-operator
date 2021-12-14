package argocd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"gotest.tools/assert"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/argoproj-labs/argocd-operator/common"
)

func TestReconcileArgoCD_reconcileRole(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	assert.NilError(t, createNamespace(r, a.Namespace, ""))
	assert.NilError(t, createNamespace(r, "newNamespaceTest", a.Namespace))

	workloadIdentifier := "x"
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)

	// check if roles are created for the new namespace as well
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newNamespaceTest"}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)

	// update reconciledRole policy rules to RedisHa policy rules
	reconciledRole.Rules = policyRuleForRedisHa(a)
	assert.NilError(t, r.Client.Update(context.TODO(), reconciledRole))

	// Check if the RedisHa policy rules are overwritten to Application Controller
	// policy rules by the reconciler
	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)
}

func TestReconcileArgoCD_reconcileRole_dex_disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	assert.NilError(t, createNamespace(r, a.Namespace, ""))

	rules := policyRuleForDexServer()
	role := newRole(dexServer, rules, a)

	// Dex is enabled
	_, err := r.reconcileRole(dexServer, rules, a)
	assert.NilError(t, err)
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: a.Namespace}, role))
	assert.DeepEqual(t, rules, role.Rules)

	// Disable Dex
	os.Setenv("DISABLE_DEX", "true")
	defer os.Unsetenv("DISABLE_DEX")

	_, err = r.reconcileRole(dexServer, rules, a)
	assert.NilError(t, err)
	assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: a.Namespace}, role), "not found")
}

func TestReconcileArgoCD_reconcileClusterRole(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(workloadIdentifier, a)
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	// cluster role should not be created
	assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, &v1.ClusterRole{}), "not found")

	os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", a.Namespace)
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	reconciledClusterRole := &v1.ClusterRole{}
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.DeepEqual(t, expectedRules, reconciledClusterRole.Rules)

	// update reconciledRole policy rules to RedisHa policy rules
	reconciledClusterRole.Rules = policyRuleForRedisHa(a)
	assert.NilError(t, r.Client.Update(context.TODO(), reconciledClusterRole))

	// Check if the RedisHa policy rules are overwritten to Application Controller
	// policy rules for cluster role by the reconciler
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.DeepEqual(t, expectedRules, reconciledClusterRole.Rules)

	// Check if the CLuster Role gets deleted
	os.Unsetenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole), "not found")
}

func TestReconcileArgoCD_RoleHooks(t *testing.T) {
	defer resetHooks()()
	a := makeTestArgoCD()
	r := makeTestReconciler(t)
	assert.NilError(t, createNamespace(r, a.Namespace, ""))
	Register(testRoleHook)

	roles, err := r.reconcileRole(applicationController, []v1.PolicyRule{}, a)
	role := roles[0]
	assert.NilError(t, err)
	assert.DeepEqual(t, role.Rules, testRules())

	roles, err = r.reconcileRole("test", []v1.PolicyRule{}, a)
	role = roles[0]
	assert.NilError(t, err)
	assert.DeepEqual(t, role.Rules, []v1.PolicyRule{})
}

func TestReconcileArgoCD_reconcileRole_custom_role(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	assert.NilError(t, createNamespace(r, a.Namespace, ""))
	assert.NilError(t, createNamespace(r, "namespace-custom-role", a.Namespace))

	workloadIdentifier := "argocd-application-controller"
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)

	// check if roles are created for the new namespace as well
	assert.NilError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "namespace-custom-role"}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)

	// set the custom role as env variable
	assert.NilError(t, os.Setenv(common.ArgoCDControllerClusterRoleEnvName, "custom-role"))
	defer os.Unsetenv(common.ArgoCDControllerClusterRoleEnvName)

	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	// check if the default cluster roles are removed
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole)
	if err == nil || !errors.IsNotFound(err) {
		t.Fatal(err)
	}

	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "namespace-custom-role"}, reconciledRole)
	if err == nil || !errors.IsNotFound(err) {
		t.Fatal(err)
	}
}
