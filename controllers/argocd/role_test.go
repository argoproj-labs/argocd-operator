package argocd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespace(r, "newNamespaceTest", a.Namespace))

	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// check if roles are created for the new namespace as well
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newNamespaceTest"}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// update reconciledRole policy rules to RedisHa policy rules
	reconciledRole.Rules = policyRuleForRedisHa(r.Client)
	assert.NoError(t, r.Update(context.TODO(), reconciledRole))

	// Check if the RedisHa policy rules are overwritten to Application Controller
	// policy rules by the reconciler
	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

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
	assert.Equal(t, expectedRoleNamespace, dexRoles[0].Namespace)
	// check no redisHa role is created for the new namespace with managed-by label
	workloadIdentifier = common.ArgoCDRedisHAComponent
	expectedRedisHaRules := policyRuleForRedisHa(r.Client)
	redisHaRoles, err := r.reconcileRole(workloadIdentifier, expectedRedisHaRules, a)
	assert.NoError(t, err)
	assert.Equal(t, expectedNumberOfRoles, len(redisHaRoles))
	assert.Equal(t, expectedRoleNamespace, redisHaRoles[0].Namespace)
	// check no redis role is created for the new namespace with managed-by label
	workloadIdentifier = common.ArgoCDRedisComponent
	expectedRedisRules := policyRuleForRedis(r.Client)
	redisRoles, err := r.reconcileRole(workloadIdentifier, expectedRedisRules, a)
	assert.NoError(t, err)
	assert.Equal(t, expectedNumberOfRoles, len(redisRoles))
	assert.Equal(t, expectedRoleNamespace, redisRoles[0].Namespace)
}

func TestReconcileArgoCD_RoleHooks(t *testing.T) {
	defer resetHooks()()
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespace(r, "namespace-custom-role", a.Namespace))

	workloadIdentifier := "argocd-application-controller"
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// check if roles are created for the new namespace as well
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "namespace-custom-role"}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// set the custom role as env variable
	t.Setenv(common.ArgoCDControllerClusterRoleEnvName, "custom-role")

	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	// check if the default cluster roles are removed
	err = r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole)
	if err == nil || !errors.IsNotFound(err) {
		t.Fatal(err)
	}

	err = r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "namespace-custom-role"}, reconciledRole)
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
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	// Create a managed namespace
	assert.NoError(t, createNamespace(r, "managedNS", a.Namespace))

	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	expectedRules := policyRuleForApplicationController()
	_, err := r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	reconciledRole := &v1.Role{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// Check if roles are created for the new namespace as well
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS"}, reconciledRole))
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
	assert.NoError(t, r.Create(
		context.TODO(), configMap))

	// Delete the newNamespaceTest ns.
	// Verify that operator should not reconcile back to create the roles in terminating ns.
	newNS := &corev1.Namespace{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: "managedNS"}, newNS)
	assert.NoError(t, err)
	err = r.Delete(context.TODO(), newNS)
	assert.NoError(t, err)

	// Verify that the namespace exists and is in terminating state.
	_ = r.Get(context.TODO(), types.NamespacedName{Name: "managedNS"}, newNS)
	assert.NotEqual(t, newNS.DeletionTimestamp, nil)

	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	// Verify that the roles are deleted
	err = r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS2"}, reconciledRole)
	assert.ErrorContains(t, err, "not found")

	// Create another managed namespace
	assert.NoError(t, createNamespace(r, "managedNS2", a.Namespace))

	// Check if roles are created for the new namespace as well
	_, err = r.reconcileRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS2"}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)
}

// This test is to verify that Custom and Aggregated ClusterRoles are not allowed together.
func TestReconcileArgoCD_reconcileClusterRole_aggregated_error(t *testing.T) {
	ctx, _, a, cl := setup(t)

	t.Log("Enable both Aggregated and Custom ClusterRoles.")
	a.Spec.AggregatedClusterRoles = true
	a.Spec.DefaultClusterScopedRoleDisabled = true
	assert.NoError(t, cl.Update(ctx, a))

	t.Log("Call function to check.")
	err := verifyInstallationMode(a, true)

	t.Log("Verify response.")
	assert.Error(t, err)
	assert.EqualError(t, err, "custom Cluster Roles and Aggregated Cluster Roles can not be used together")
}

func setup(t *testing.T) (context.Context, *ReconcileArgoCD, *argoproj.ArgoCD, client.Client) {
	logf.SetLogger(ZapLogger(true))
	ctx := context.Background()
	a := makeTestArgoCD()
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	// Set the namespace to be cluster-scoped
	return ctx, r, a, cl
}

// validateAggregatedAdminClusterRole checks that ClusterRole for View permissions has field values configured in aggregated mode
func validateAggregatedViewClusterRole(t *testing.T, ctx context.Context, r *ReconcileArgoCD, clusterRoleName string) {
	// Ensure aggregated ClusterRole for view permission is created
	reconciledClusterRole := &v1.ClusterRole{}
	assert.NoError(t, r.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))

	// Ensure ClusterRole has expected Labels
	assert.EqualValues(t, reconciledClusterRole.Labels[common.ArgoCDAggregateToControllerLabelKey], "true")
	// Ensure ClusterRole has no pre defined Rules
	assert.EqualValues(t, reconciledClusterRole.Rules, policyRuleForApplicationControllerView())
}

// validateAggregatedAdminClusterRole checks that ClusterRole for Admin permissions has field values configured in aggregated mode
func validateAggregatedAdminClusterRole(t *testing.T, ctx context.Context, r *ReconcileArgoCD, a *argoproj.ArgoCD, reconciledClusterRole *v1.ClusterRole, clusterRoleName string) {

	assert.NoError(t, r.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))

	// Ensure ClusterRole has expected AggregationRule
	expectedAggregationRule := &v1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{{
		MatchLabels: map[string]string{common.ArgoCDAggregateToAdminLabelKey: "true", common.ArgoCDKeyManagedBy: a.Name}}}}
	assert.Equal(t, reconciledClusterRole.AggregationRule, expectedAggregationRule)

	// Ensure ClusterRole has expected Labels
	assert.EqualValues(t, reconciledClusterRole.Labels[common.ArgoCDAggregateToControllerLabelKey], "true")
	// Ensure ClusterRole has no pre defined Rules
	assert.EqualValues(t, reconciledClusterRole.Rules, policyRuleForApplicationControllerAdmin())
}

// validateAggregatedControllerClusterRole checks that ClusterRole has field values configured in aggregated mode
func validateAggregatedControllerClusterRole(t *testing.T, ctx context.Context, r *ReconcileArgoCD, a *argoproj.ArgoCD, reconciledClusterRole *v1.ClusterRole, clusterRoleName string) {

	assert.NoError(t, r.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))

	// Ensure ClusterRole has expected AggregationRule
	expectedAggregationRule := &v1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{{
		MatchLabels: map[string]string{common.ArgoCDAggregateToControllerLabelKey: "true", common.ArgoCDKeyManagedBy: a.Name}}}}
	assert.Equal(t, reconciledClusterRole.AggregationRule, expectedAggregationRule)

	// Ensure expected Annotation is added in ClusterRole
	assert.EqualValues(t, reconciledClusterRole.Annotations[common.AutoUpdateAnnotationKey], "true")
	// Ensure now ClusterRole has no pre defined Rules
	assert.Empty(t, reconciledClusterRole.Rules)
}

// validateDefaultClusterRole checks that ClusterRole has field values configured in default mode
func validateDefaultClusterRole(t *testing.T, ctx context.Context, r *ReconcileArgoCD, reconciledClusterRole *v1.ClusterRole, clusterRoleName string) {

	assert.NoError(t, r.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))

	// Ensure ClusterRole does not have AggregationRule
	assert.Empty(t, reconciledClusterRole.AggregationRule)
	// Ensure ClusterRole does not have Annotations
	assert.NotContains(t, reconciledClusterRole.Annotations, common.AutoUpdateAnnotationKey)
	// Ensure ClusterRole has pre defined Rules
	assert.Equal(t, reconciledClusterRole.Rules, policyRuleForApplicationController())
}

// validateCustomClusterRole checks that default ClusterRole is deleted
func validateCustomClusterRole(t *testing.T, ctx context.Context, r *ReconcileArgoCD, clusterRoleName string, reconciledClusterRole *v1.ClusterRole) {
	err := r.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "not found")
}

// enableAggregatedClusterRoles will set fields to create aggregated ClusterRoles
func enableAggregatedClusterRoles(t *testing.T, ctx context.Context, a *argoproj.ArgoCD, cl client.Client) {
	a.Spec.AggregatedClusterRoles = true
	a.Spec.DefaultClusterScopedRoleDisabled = false
	assert.NoError(t, cl.Update(ctx, a))
}

// enableCustomClusterRoles will set fields to create custom ClusterRoles
func enableCustomClusterRoles(t *testing.T, ctx context.Context, a *argoproj.ArgoCD, cl client.Client) {
	a.Spec.DefaultClusterScopedRoleDisabled = true
	a.Spec.AggregatedClusterRoles = false
	assert.NoError(t, cl.Update(ctx, a))
}

// enableDefaultClusterRoles will set fields to create default ClusterRoles
func enableDefaultClusterRoles(t *testing.T, ctx context.Context, a *argoproj.ArgoCD, cl client.Client) {
	a.Spec.DefaultClusterScopedRoleDisabled = false
	a.Spec.AggregatedClusterRoles = false
	assert.NoError(t, cl.Update(ctx, a))
}

func TestReconcileArgoCD_reconcileRole_enable_controller_role(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	componentName := common.ArgoCDApplicationControllerComponent

	_, err := r.reconcileRole(componentName, []v1.PolicyRule{}, a)
	assert.NoError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, componentName)
	reconciledRole := &v1.Role{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))

	flag := false
	a.Spec.Controller.Enabled = &flag

	_, err = r.reconcileRole(componentName, []v1.PolicyRule{}, a)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole)
	assert.Error(t, err)
	assertNotFound(t, err)
}

func TestReconcileArgoCD_reconcileRole_enable_redis_role(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	componentName := common.ArgoCDRedisComponent

	_, err := r.reconcileRole(componentName, []v1.PolicyRule{}, a)
	assert.NoError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, componentName)
	reconciledRole := &v1.Role{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))

	flag := false
	a.Spec.Redis.Enabled = &flag

	_, err = r.reconcileRole(componentName, []v1.PolicyRule{}, a)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole)
	assert.Error(t, err)
	assertNotFound(t, err)
}

func TestReconcileArgoCD_reconcileRole_enable_server_role(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	componentName := common.ArgoCDServerComponent

	_, err := r.reconcileRole(componentName, []v1.PolicyRule{}, a)
	assert.NoError(t, err)

	expectedName := fmt.Sprintf("%s-%s", a.Name, componentName)
	reconciledRole := &v1.Role{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))

	flag := false
	a.Spec.Server.Enabled = &flag

	_, err = r.reconcileRole(componentName, []v1.PolicyRule{}, a)
	assert.NoError(t, err)

	err = r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole)
	assert.Error(t, err)
	assertNotFound(t, err)
}
