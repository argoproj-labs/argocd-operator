package argocd

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

func TestReconcileArgoCD_reconcileRole(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespace(r, "newNamespaceTest", a.Namespace))

	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// check if roles are created for the new namespace as well
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newNamespaceTest"}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// update reconciledRole policy rules to RedisHa policy rules
	reconciledRole.Rules = policyRuleForRedisHa(r.Client)
	assert.NoError(t, r.Client.Update(context.TODO(), reconciledRole))

	// Check if the RedisHa policy rules are overwritten to Application Controller
	// policy rules by the reconciler
	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)
}
func TestReconcileArgoCD_reconcileRole_for_new_namespace(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespace(r, "newNamespaceTest", a.Namespace))

	// only 1 role for the Argo CD instance namespace will be created
	expectedNumberOfRoles := 1
	// check no dexServer role is created for the new namespace with managed-by label
	workloadIdentifier := common.ArgoCDDexServerComponent
	expectedRoleNamespace := a.Namespace
	expectedDexServerRules := policyRuleForDexServer()
	dexRoles, err := r.reconcileRole(workloadIdentifier, expectedDexServerRules, a)
	assert.NoError(t, err)
	assert.Equal(t, expectedNumberOfRoles, len(dexRoles))
	assert.Equal(t, expectedRoleNamespace, dexRoles[0].ObjectMeta.Namespace)
	// check no redisHa role is created for the new namespace with managed-by label
	workloadIdentifier = common.ArgoCDRedisHAComponent
	expectedRedisHaRules := policyRuleForRedisHa(r.Client)
	redisHaRoles, err := r.reconcileRole(workloadIdentifier, expectedRedisHaRules, a)
	assert.NoError(t, err)
	assert.Equal(t, expectedNumberOfRoles, len(redisHaRoles))
	assert.Equal(t, expectedRoleNamespace, redisHaRoles[0].ObjectMeta.Namespace)
	// check no redis role is created for the new namespace with managed-by label
	workloadIdentifier = common.ArgoCDRedisComponent
	expectedRedisRules := policyRuleForRedis(r.Client)
	redisRoles, err := r.reconcileRole(workloadIdentifier, expectedRedisRules, a)
	assert.NoError(t, err)
	assert.Equal(t, expectedNumberOfRoles, len(redisRoles))
	assert.Equal(t, expectedRoleNamespace, redisRoles[0].ObjectMeta.Namespace)
	// check no grafana role is created for the new namespace with managed-by label
	workloadIdentifier = common.ArgoCDOperatorGrafanaComponent
	expectedGrafanaRules := policyRuleForGrafana(r.Client)
	grafanaRoles, err := r.reconcileRole(workloadIdentifier, expectedGrafanaRules, a)
	assert.NoError(t, err)
	assert.Equal(t, expectedNumberOfRoles, len(grafanaRoles))
	assert.Equal(t, expectedRoleNamespace, grafanaRoles[0].ObjectMeta.Namespace)
}

func TestReconcileArgoCD_reconcileClusterRole(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(workloadIdentifier, a)
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	// cluster role should not be created
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, &v1.ClusterRole{}), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, &v1.ClusterRole{}))
	assert.Contains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, &v1.ClusterRole{}).Error(), "not found")

	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", a.Namespace)
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	reconciledClusterRole := &v1.ClusterRole{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.Equal(t, expectedRules, reconciledClusterRole.Rules)

	// update reconciledRole policy rules to RedisHa policy rules
	reconciledClusterRole.Rules = policyRuleForRedisHa(r.Client)
	assert.NoError(t, r.Client.Update(context.TODO(), reconciledClusterRole))

	// Check if the RedisHa policy rules are overwritten to Application Controller
	// policy rules for cluster role by the reconciler
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.Equal(t, expectedRules, reconciledClusterRole.Rules)

	// Check if the CLuster Role gets deleted
	os.Unsetenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	assert.Contains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole).Error(), "not found")
}

func TestReconcileArgoCD_reconcileRoleForApplicationSourceNamespaces(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	sourceNamespace := "newNamespaceTest"
	a := makeTestArgoCD()
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{
			sourceNamespace,
		},
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespaceManagedByClusterArgoCDLabel(r, sourceNamespace, a.Namespace))

	workloadIdentifier := common.ArgoCDServerComponent
	expectedRules := policyRuleForServerApplicationSourceNamespaces()
	err := r.reconcileRoleForApplicationSourceNamespaces(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	expectedName := getRoleNameForApplicationSourceNamespaces(sourceNamespace, a)
	reconciledRole := &v1.Role{}

	// check if roles are created for the new namespace
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: sourceNamespace}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

}

func TestReconcileArgoCD_RoleHooks(t *testing.T) {
	defer resetHooks()()
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	Register(testRoleHook)

	roles, err := r.reconcileRole(common.ArgoCDApplicationControllerComponent, []v1.PolicyRule{}, a)
	role := roles[0]
	assert.NoError(t, err)
	assert.Equal(t, role.Rules, testRules())

	roles, err = r.reconcileRole("test", []v1.PolicyRule{}, a)
	role = roles[0]
	assert.NoError(t, err)
	assert.Equal(t, role.Rules, []v1.PolicyRule{})
}

func TestReconcileArgoCD_reconcileRole_custom_role(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespace(r, "namespace-custom-role", a.Namespace))

	workloadIdentifier := "argocd-application-controller"
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// check if roles are created for the new namespace as well
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "namespace-custom-role"}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// set the custom role as env variable
	t.Setenv(common.ArgoCDControllerClusterRoleEnvName, "custom-role")

	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

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

// This test validates the behavior of the operator reconciliation when a managed namespace is not properly terminated
// or remains terminating may be because of some resources in the namespace not getting deleted.
func TestReconcileRoles_ManagedTerminatingNamespace(t *testing.T) {

	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	// Create a managed namespace
	assert.NoError(t, createNamespace(r, "managedNS", a.Namespace))

	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// Check if roles are created for the new namespace as well
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS"}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

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
	// Verify that operator should not reconcile back to create the roles in terminating ns.
	newNS := &corev1.Namespace{}
	r.Client.Get(context.TODO(), types.NamespacedName{Namespace: "managedNS", Name: "managedNS"}, newNS)
	r.Client.Delete(context.TODO(), newNS)

	// Verify that the namespace exists and is in terminating state.
	r.Client.Get(context.TODO(), types.NamespacedName{Namespace: "managedNS", Name: "managedNS"}, newNS)
	assert.NotEqual(t, newNS.DeletionTimestamp, nil)

	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	// Verify that the roles are deleted
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS2"}, reconciledRole)
	assert.ErrorContains(t, err, "not found")

	// Create another managed namespace
	assert.NoError(t, createNamespace(r, "managedNS2", a.Namespace))

	// Check if roles are created for the new namespace as well
	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS2"}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)
}
