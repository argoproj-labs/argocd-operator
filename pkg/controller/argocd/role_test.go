package argocd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"gotest.tools/assert"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileRole(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := "x"
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)

	// update reconciledRole policy rules to RedisHa policy rules
	reconciledRole.Rules = policyRuleForRedisHa(a)
	assert.NilError(t, r.client.Update(context.TODO(), reconciledRole))

	// Check if the RedisHa policy rules are overwritten to Application Controller
	// policy rules by the reconciler
	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)
}

func TestReconcileArgoCD_reconcileRole_dex_disabled(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	rules := policyRuleForDexServer()
	role := newRole(dexServer, rules, a)

	// Dex is enabled
	_, err := r.reconcileRole(dexServer, rules, a)
	assert.NilError(t, err)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: a.Namespace}, role))
	assert.DeepEqual(t, rules, role.Rules)

	// Disable Dex
	os.Setenv("DISABLE_DEX", "true")
	defer os.Unsetenv("DISABLE_DEX")

	_, err = r.reconcileRole(dexServer, rules, a)
	assert.NilError(t, err)
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: a.Namespace}, role), "not found")
}

func TestReconcileArgoCD_reconcileClusterRole(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(workloadIdentifier, a)
	expectedRules := []v1.PolicyRule{}
	_, err := r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	// cluster role should not be created
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, &v1.ClusterRole{}), "not found")

	expectedRules = testClusterRoleRules()
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, err)

	reconciledClusterRole := &v1.ClusterRole{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.DeepEqual(t, expectedRules, reconciledClusterRole.Rules)

	// update reconciledRole policy rules to RedisHa policy rules
	reconciledClusterRole.Rules = policyRuleForRedisHa(a)
	assert.NilError(t, r.client.Update(context.TODO(), reconciledClusterRole))

	// Check if the RedisHa policy rules are overwritten to Application Controller
	// policy rules for cluster role by the reconciler
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.DeepEqual(t, expectedRules, reconciledClusterRole.Rules)

	// Check if the CLuster Role gets deleted
	_, err = r.reconcileClusterRole(workloadIdentifier, []v1.PolicyRule{}, a)
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole), "not found")
}
