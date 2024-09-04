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

func TestReconcileArgoCD_reconcileClusterRole_custom_cluster_role(t *testing.T) {
	ctx, r, a, cl := setup(t)
	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(workloadIdentifier, a)
	expectedRules := policyRuleForApplicationController()
	reconciledClusterRole := &v1.ClusterRole{}

	t.Log("Mode 1: Enable custom ClusterRole")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole")
	_, err := r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 1: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))

	t.Log("Mode 2: Enable default ClusterRole")
	enableDefaultClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole")
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify default ClusterRole is created.")
	validateDefaultClusterRole(t, ctx, r, reconciledClusterRole, clusterRoleName)

	t.Log("Mode 1: Enable custom ClusterRole")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole")
	_, err = r.reconcileClusterRole(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)
}

func TestReconcileArgoCD_reconcileRoleForApplicationSourceNamespaces(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	sourceNamespace := "newNamespaceTest"
	a := makeTestArgoCD()
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{
			sourceNamespace,
		},
		ApplicationSet: &argoproj.ArgoCDApplicationSet{
			SourceNamespaces: []string{"tmp"},
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

	workloadIdentifier := common.ArgoCDServerComponent + "-custom"
	expectedRules := policyRuleForServerApplicationSourceNamespaces()
	err := r.reconcileRoleForApplicationSourceNamespaces(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	expectedName := getRoleNameForApplicationSourceNamespaces(sourceNamespace, a)
	reconciledRole := &v1.Role{}

	// check if roles are created for the new namespace
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: sourceNamespace}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// check if appset rules are added for server-role when new appset namespace is added
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{
			sourceNamespace,
		},
		ApplicationSet: &argoproj.ArgoCDApplicationSet{
			SourceNamespaces: []string{"tmp", sourceNamespace},
		},
	}
	err = r.reconcileRoleForApplicationSourceNamespaces(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)
	reconciledRole = &v1.Role{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: sourceNamespace}, reconciledRole))
	assert.Equal(t, append(expectedRules, policyRuleForServerApplicationSetSourceNamespaces()...), reconciledRole.Rules)

	// check if appset rules are removed for server-role when appset namespace is removed from the list
	a.Spec = argoproj.ArgoCDSpec{
		SourceNamespaces: []string{
			sourceNamespace,
		},
		ApplicationSet: &argoproj.ArgoCDApplicationSet{
			SourceNamespaces: []string{"tmp"},
		},
	}
	err = r.reconcileRoleForApplicationSourceNamespaces(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)
	reconciledRole = &v1.Role{}
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
	assert.EqualError(t, err, "Custom Cluster Roles and Aggregated Cluster Roles can not be used together.")
}

// This test is to verify that base aggregated ClusterRole is created.
func TestReconcileArgoCD_reconcileClusterRole_aggregated_controller(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponent
	reconciledClusterRole := &v1.ClusterRole{}

	t.Log("Enable aggregated ClusterRole")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRuleForApplicationController(), a)

	t.Log("Verify response.")
	assert.NoError(t, err)
	validateAggregatedControllerClusterRole(t, ctx, r, a, reconciledClusterRole, GenerateUniqueResourceName(componentName, a))
}

// This test is to verify that aggregated ClusterRole for view permission is created.
func TestReconcileArgoCD_reconcileClusterRole_aggregated_view(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponentView
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	t.Log("Enable aggregated ClusterRole")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRuleForApplicationControllerView(), a)

	t.Log("Verify response.")
	assert.NoError(t, err)
	validateAggregatedViewClusterRole(t, ctx, r, clusterRoleName)

	t.Log("Change ClusterRole fields.")
	reconciledClusterRole := &v1.ClusterRole{}
	assert.NoError(t, r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	reconciledClusterRole.Labels = map[string]string{"test": "test"}
	assert.NoError(t, r.Client.Update(ctx, reconciledClusterRole))

	t.Log("Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRuleForApplicationControllerView(), a)

	t.Log("Verify changes are reverted.")
	assert.NoError(t, err)
	validateAggregatedViewClusterRole(t, ctx, r, clusterRoleName)
}

// This test is to verify that aggregated ClusterRole for admin permission is created.
func TestReconcileArgoCD_reconcileClusterRole_aggregated_admin(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponentAdmin
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	reconciledClusterRole := &v1.ClusterRole{}

	t.Log("Enable aggregated ClusterRole")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRuleForApplicationControllerAdmin(), a)

	t.Log("Verify response.")
	assert.NoError(t, err)
	validateAggregatedAdminClusterRole(t, ctx, r, a, reconciledClusterRole, clusterRoleName)

	t.Log("Change ClusterRole fields.")
	assert.NoError(t, r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))
	reconciledClusterRole.Labels = map[string]string{"test": "test"}
	reconciledClusterRole.AggregationRule = &v1.AggregationRule{}
	assert.NoError(t, r.Client.Update(ctx, reconciledClusterRole))

	t.Log("Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRuleForApplicationControllerAdmin(), a)

	t.Log("Verify changes are reverted.")
	assert.NoError(t, err)
	validateAggregatedAdminClusterRole(t, ctx, r, a, reconciledClusterRole, clusterRoleName)
}

// This test is to verify the scenario for View permission when user is switching between aggregated and default ClusterRoles.
func TestReconcileArgoCD_reconcileClusterRole_view_aggregated_and_default(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponentView
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationControllerView()
	reconciledClusterRole := &v1.ClusterRole{}

	//------- Aggregated Mode -------
	t.Log("Mode 1: Enable aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify aggregated ClusterRole for View permission is created.")
	validateAggregatedViewClusterRole(t, ctx, r, clusterRoleName)

	//------- Default Mode -------
	t.Log("Mode 2: Switch to default ClusterRole.")
	enableDefaultClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify aggregated ClusterRole for View permission is deleted now.")
	err = r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "not found")

	//------- Aggregated Mode -------
	t.Log("Mode 1: Switch back to aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify aggregated ClusterRole for View permission is created.")
	validateAggregatedViewClusterRole(t, ctx, r, clusterRoleName)
}

// This test is to verify the scenario for Admin permission when user is switching between aggregated and default ClusterRoles.
func TestReconcileArgoCD_reconcileClusterRole_admin_aggregated_and_default(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponentAdmin
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationControllerAdmin()
	reconciledClusterRole := &v1.ClusterRole{}

	//------- Aggregated Mode -------
	t.Log("Mode 1: Enable aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify aggregated ClusterRole for Admin permission is created.")
	validateAggregatedAdminClusterRole(t, ctx, r, a, reconciledClusterRole, GenerateUniqueResourceName(componentName, a))

	//------- Default Mode -------
	t.Log("Mode 2: Switch to default ClusterRole.")
	enableDefaultClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify aggregated ClusterRole for Admin permission is deleted now.")
	err = r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "not found")

	//------- Aggregated Mode -------
	t.Log("Mode 1: Switch back to aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify aggregated ClusterRole for View permission is created.")
	validateAggregatedAdminClusterRole(t, ctx, r, a, reconciledClusterRole, GenerateUniqueResourceName(componentName, a))
}

// This test is to verify the scenario for View permission when user is switching between aggregated and custom ClusterRoles.
func TestReconcileArgoCD_reconcileClusterRole_view_aggregated_and_custom(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponentView
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationControllerView()
	reconciledClusterRole := &v1.ClusterRole{}

	//------- Aggregated Mode -------
	t.Log("Mode 1: Enable aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify aggregated ClusterRole for View permission is created.")
	validateAggregatedViewClusterRole(t, ctx, r, clusterRoleName)

	//------- Custom Mode -------
	t.Log("Mode 2: Switch to custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 2: Verify aggregated ClusterRole for View permission is deleted now.")
	err = r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "not found")

	t.Log("Mode 2: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))

	//------- Aggregated Mode -------
	t.Log("Mode 1: Switch back to aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify aggregated ClusterRole is created.")
	validateAggregatedViewClusterRole(t, ctx, r, clusterRoleName)
}

// This test is to verify the scenario for Admin permission when user is switching between aggregated and custom ClusterRoles.
func TestReconcileArgoCD_reconcileClusterRole_admin_aggregated_and_custom(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponentAdmin
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationControllerAdmin()
	reconciledClusterRole := &v1.ClusterRole{}

	//------- Aggregated Mode -------
	t.Log("Mode 1: Enable aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify aggregated ClusterRole for Admin permission is created.")
	validateAggregatedAdminClusterRole(t, ctx, r, a, reconciledClusterRole, GenerateUniqueResourceName(componentName, a))

	//------- Custom Mode -------
	t.Log("Mode 2: Switch to custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 2: Verify aggregated ClusterRole for Admin permission is deleted now.")
	err = r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "not found")

	t.Log("Mode 2: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))

	//------- Aggregated Mode -------
	t.Log("Mode 1: Switch back to aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify aggregated ClusterRole for Admin permission is created.")
	validateAggregatedAdminClusterRole(t, ctx, r, a, reconciledClusterRole, GenerateUniqueResourceName(componentName, a))
}

// This test is to verify the scenario when user is switching between default and aggregated ClusterRoles.
func TestReconcileArgoCD_reconcileClusterRole_default_and_aggregated(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationController()
	reconciledClusterRole := &v1.ClusterRole{}

	//----- Default Mode -----
	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify default ClusterRole is created.")
	validateDefaultClusterRole(t, ctx, r, reconciledClusterRole, clusterRoleName)

	//----- Aggregated Mode -----
	t.Log("Mode 2: Switch to aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify aggregated ClusterRole is created.")
	validateAggregatedControllerClusterRole(t, ctx, r, a, reconciledClusterRole, clusterRoleName)

	//----- Default Mode -----
	t.Log("Mode 1: Switch back to default ClusterRole.")
	enableDefaultClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify default ClusterRole is created.")
	validateDefaultClusterRole(t, ctx, r, reconciledClusterRole, clusterRoleName)
}

// This test is to verify the scenario when user is switching between default and custom ClusterRoles.
func TestReconcileArgoCD_reconcileClusterRole_default_and_custom(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationController()
	reconciledClusterRole := &v1.ClusterRole{}

	//----- Default Mode -----
	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify default ClusterRole is created.")
	validateDefaultClusterRole(t, ctx, r, reconciledClusterRole, clusterRoleName)

	//----- Custom Mode -----
	t.Log("Mode 2: Switch to custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 2: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))

	//----- Default Mode -----
	t.Log("Mode 1: Switch back to default ClusterRole.")
	enableDefaultClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify default ClusterRole is created.")
	validateDefaultClusterRole(t, ctx, r, reconciledClusterRole, clusterRoleName)
}

// This test is to verify the scenario when user is switching between custom and aggregated ClusterRoles.
func TestReconcileArgoCD_reconcileClusterRole_custom_and_aggregated(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationController()
	reconciledClusterRole := &v1.ClusterRole{}

	//----- Custom Mode -----
	t.Log("Mode 1: Enable custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 1: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))

	//----- Aggregated Mode -----
	t.Log("Mode 2: Switch to aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify aggregated ClusterRole is created.")
	validateAggregatedControllerClusterRole(t, ctx, r, a, reconciledClusterRole, clusterRoleName)

	//----- Custom Mode -----
	t.Log("Mode 1: Switch back to custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)
}

// This test is to verify the scenario when user is switching from default to custom ClusterRole and then to aggregated ClusterRole.
func TestReconcileArgoCD_reconcileClusterRole_default_to_custom_to_aggregated(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationController()
	reconciledClusterRole := &v1.ClusterRole{}

	//------- Default Mode -------
	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify default ClusterRole is created.")
	validateDefaultClusterRole(t, ctx, r, reconciledClusterRole, clusterRoleName)

	//------- Custom Mode -------
	t.Log("Mode 2: Switch to custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 2: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))

	//------- Aggregated Mode -------
	t.Log("Mode 3: Switch to aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 3: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 3: Verify aggregated ClusterRole is created.")
	validateAggregatedControllerClusterRole(t, ctx, r, a, reconciledClusterRole, clusterRoleName)
}

// This test is to verify the scenario when user is switching from default to aggregated ClusterRole and then to custom ClusterRole.
func TestReconcileArgoCD_reconcileClusterRole_default_to_aggregated_to_custom(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationController()
	reconciledClusterRole := &v1.ClusterRole{}

	//------- Default Mode -------
	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify default ClusterRole is created.")
	validateDefaultClusterRole(t, ctx, r, reconciledClusterRole, clusterRoleName)

	//------- Aggregated Mode -------
	t.Log("Mode 2: Switch to aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify aggregated ClusterRole is created.")
	validateAggregatedControllerClusterRole(t, ctx, r, a, reconciledClusterRole, clusterRoleName)

	//------- Custom Mode -------
	t.Log("Mode 3: Switch to custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 3: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 3: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 3: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))
}

// This test is to verify the scenario when user is switching from custom to default ClusterRole and then to aggregated ClusterRole.
func TestReconcileArgoCD_reconcileClusterRole_custom_to_default_to_aggregated(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationController()
	reconciledClusterRole := &v1.ClusterRole{}

	//------- Custom Mode -------
	t.Log("Mode 1: Enable custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 1: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))

	//------- Default Mode -------
	t.Log("Mode 2: Switch to default ClusterRole.")
	enableDefaultClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	//------- Aggregated Mode -------
	t.Log("Mode 3: Switch to aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 3: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 3: Verify aggregated ClusterRole is created.")
	validateAggregatedControllerClusterRole(t, ctx, r, a, reconciledClusterRole, clusterRoleName)
}

// This test is to verify the scenario when user is switching from custom to aggregated ClusterRole and then to default ClusterRole.
func TestReconcileArgoCD_reconcileClusterRole_custom_to_aggregated_to_default(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationController()
	reconciledClusterRole := &v1.ClusterRole{}

	//------- Custom Mode -------
	t.Log("Mode 1: Enable custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 1: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))

	//------- Aggregated Mode -------
	t.Log("Mode 2: Switch to aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify aggregated ClusterRole is created.")
	validateAggregatedControllerClusterRole(t, ctx, r, a, reconciledClusterRole, clusterRoleName)

	//------- Default Mode -------
	t.Log("Mode 3: Switch to default ClusterRole.")
	enableDefaultClusterRoles(t, ctx, a, cl)

	t.Log("Mode 3: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 3: Verify default ClusterRole is created.")
	validateDefaultClusterRole(t, ctx, r, reconciledClusterRole, clusterRoleName)
}

// This test is to verify the scenario when user is switching from aggregated to default ClusterRole and then to custom ClusterRole.
func TestReconcileArgoCD_reconcileClusterRole_aggregated_to_default_to_custom(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationController()
	reconciledClusterRole := &v1.ClusterRole{}

	//------- Aggregated Mode -------
	t.Log("Mode 1: Enable aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify aggregated ClusterRole is created.")
	validateAggregatedControllerClusterRole(t, ctx, r, a, reconciledClusterRole, clusterRoleName)

	//------- Default Mode -------
	t.Log("Mode 2: Switch to default ClusterRole.")
	enableDefaultClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify default ClusterRole is created.")
	validateDefaultClusterRole(t, ctx, r, reconciledClusterRole, clusterRoleName)

	//------- Custom Mode -------
	t.Log("Mode 3: Switch to custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 3: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 3: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 3: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))
}

// This test is to verify the scenario when user is switching from aggregated to custom ClusterRole and then to default ClusterRole.
func TestReconcileArgoCD_reconcileClusterRole_aggregated_to_custom_to_default(t *testing.T) {
	ctx, r, a, cl := setup(t)
	componentName := common.ArgoCDApplicationControllerComponent
	clusterRoleName := GenerateUniqueResourceName(componentName, a)
	policyRules := policyRuleForApplicationController()
	reconciledClusterRole := &v1.ClusterRole{}

	//------- Aggregated Mode -------
	t.Log("Mode 1: Enable aggregated ClusterRole.")
	enableAggregatedClusterRoles(t, ctx, a, cl)

	t.Log("Mode 1: Reconcile ClusterRole.")
	_, err := r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 1: Verify aggregated ClusterRole is created.")
	validateAggregatedControllerClusterRole(t, ctx, r, a, reconciledClusterRole, clusterRoleName)

	//------- Custom Mode -------
	t.Log("Mode 2: Switch to custom ClusterRole.")
	enableCustomClusterRoles(t, ctx, a, cl)

	t.Log("Mode 2: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 2: Verify custom ClusterRole is allowed.")
	validateCustomClusterRole(t, ctx, r, clusterRoleName, reconciledClusterRole)

	t.Log("Mode 2: Create a custom ClusterRole.")
	customClusterRole := newClusterRole(clusterRoleName, []v1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"get"}}}, a)
	assert.NoError(t, r.Client.Create(ctx, customClusterRole))

	//------- Default Mode -------
	t.Log("Mode 3: Switch to default ClusterRole.")
	enableDefaultClusterRoles(t, ctx, a, cl)

	t.Log("Mode 3: Reconcile ClusterRole.")
	_, err = r.reconcileClusterRole(componentName, policyRules, a)
	assert.NoError(t, err)

	t.Log("Mode 3: Verify default ClusterRole is created.")
	validateDefaultClusterRole(t, ctx, r, reconciledClusterRole, clusterRoleName)
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
	r := makeTestReconciler(cl, sch)
	// Set the namespace to be cluster-scoped
	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", a.Namespace)
	return ctx, r, a, cl
}

// validateAggregatedAdminClusterRole checks that ClusterRole for View permissions has field values configured in aggregated mode
func validateAggregatedViewClusterRole(t *testing.T, ctx context.Context, r *ReconcileArgoCD, clusterRoleName string) {
	// Ensure aggregated ClusterRole for view permission is created
	reconciledClusterRole := &v1.ClusterRole{}
	assert.NoError(t, r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))

	// Ensure ClusterRole has expected Labels
	assert.EqualValues(t, reconciledClusterRole.Labels[common.ArgoCDAggregateToControllerLabelKey], "true")
	// Ensure ClusterRole has no pre defined Rules
	assert.EqualValues(t, reconciledClusterRole.Rules, policyRuleForApplicationControllerView())
}

// validateAggregatedAdminClusterRole checks that ClusterRole for Admin permissions has field values configured in aggregated mode
func validateAggregatedAdminClusterRole(t *testing.T, ctx context.Context, r *ReconcileArgoCD, a *argoproj.ArgoCD, reconciledClusterRole *v1.ClusterRole, clusterRoleName string) {

	assert.NoError(t, r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))

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

	assert.NoError(t, r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))

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

	assert.NoError(t, r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole))

	// Ensure ClusterRole does not have AggregationRule
	assert.Empty(t, reconciledClusterRole.AggregationRule)
	// Ensure ClusterRole does not have Annotations
	assert.NotContains(t, reconciledClusterRole.Annotations, common.AutoUpdateAnnotationKey)
	// Ensure ClusterRole has pre defined Rules
	assert.Equal(t, reconciledClusterRole.Rules, policyRuleForApplicationController())
}

// validateCustomClusterRole checks that default ClusterRole is deleted
func validateCustomClusterRole(t *testing.T, ctx context.Context, r *ReconcileArgoCD, clusterRoleName string, reconciledClusterRole *v1.ClusterRole) {
	err := r.Client.Get(ctx, types.NamespacedName{Name: clusterRoleName}, reconciledClusterRole)
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
