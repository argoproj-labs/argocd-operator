package clusterargocd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func TestReconcileArgoCD_reconcileRoleBinding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestClusterArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	p := policyRuleForApplicationController()

	assert.NoError(t, createNamespace(r, a.Spec.ControlPlaneNamespace, ""))
	assert.NoError(t, createNamespace(r, "newTestNamespace", a.Spec.ControlPlaneNamespace))

	workloadIdentifier := common.ArgoCDApplicationControllerComponent

	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding := &rbacv1.RoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Spec.ControlPlaneNamespace}, roleBinding))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newTestNamespace"}, roleBinding))

	// update role reference and subject of the rolebinding
	roleBinding.RoleRef.Name = "not-xrb"
	roleBinding.Subjects[0].Name = "not-xrb"
	assert.NoError(t, r.Update(context.TODO(), roleBinding))

	// try reconciling it again and verify if the changes are overwritten
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding = &rbacv1.RoleBinding{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Spec.ControlPlaneNamespace}, roleBinding))
}

func TestReconcileArgoCD_reconcileRoleBinding_for_new_namespace(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestClusterArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Spec.ControlPlaneNamespace, ""))
	assert.NoError(t, createNamespace(r, "newTestNamespace", a.Spec.ControlPlaneNamespace))

	// check no dexServer rolebinding is created for the new namespace with managed-by label
	roleBinding := &rbacv1.RoleBinding{}
	workloadIdentifier := common.ArgoCDDexServerComponent
	expectedDexServerRules := policyRuleForDexServer()
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, expectedDexServerRules, a))
	assert.Error(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newTestNamespace"}, roleBinding))

	// check no redisHa rolebinding is created for the new namespace with managed-by label
	workloadIdentifier = common.ArgoCDRedisHAComponent
	expectedRedisHaRules := policyRuleForRedisHa(r.Client)
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, expectedRedisHaRules, a))
	assert.Error(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newTestNamespace"}, roleBinding))

	// check no redis rolebinding is created for the new namespace with managed-by label
	workloadIdentifier = common.ArgoCDRedisComponent
	expectedRedisRules := policyRuleForRedis(r.Client)
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, expectedRedisRules, a))
	assert.Error(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newTestNamespace"}, roleBinding))
}

// This test validates the behavior of the operator reconciliation when a managed namespace is not properly terminated
// or remains terminating may be because of some resources in the namespace not getting deleted.
func TestReconcileRoleBinding_for_Managed_Teminating_Namespace(t *testing.T) {
	a := makeTestClusterArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Spec.ControlPlaneNamespace, ""))
	assert.NoError(t, createNamespace(r, "managedNS", a.Spec.ControlPlaneNamespace))

	// Verify role bindings are created for the new namespace with managed-by label
	roleBinding := &rbacv1.RoleBinding{}
	workloadIdentifier := common.ArgoCDApplicationControllerComponent
	expectedRules := policyRuleForApplicationController()
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, expectedRules, a))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS"}, roleBinding))

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
	// Verify that operator should not reconcile back to create the roleBindings in terminating ns.
	newNS := &corev1.Namespace{}
	err := r.Get(context.TODO(), types.NamespacedName{Name: "managedNS"}, newNS)
	assert.NoError(t, err)
	err = r.Delete(context.TODO(), newNS)
	assert.NoError(t, err)

	// Verify that the namespace exists and is in terminating state.
	_ = r.Get(context.TODO(), types.NamespacedName{Name: "managedNS"}, newNS)
	assert.NotEqual(t, newNS.DeletionTimestamp, nil)

	err = r.reconcileRoleBinding(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)

	// Verify that the role bindings are deleted
	err = r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS2"}, roleBinding)
	assert.ErrorContains(t, err, "not found")

	// Create another managed namespace
	assert.NoError(t, createNamespace(r, "managedNS2", a.Spec.ControlPlaneNamespace))

	// Check if roleBindings are created for the new namespace as well
	err = r.reconcileRoleBinding(workloadIdentifier, expectedRules, a)
	assert.NoError(t, err)
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "managedNS2"}, roleBinding))
}

func TestReconcileArgoCD_reconcileClusterRoleBinding(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestClusterArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	workloadIdentifier := "x"
	expectedClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier}}

	assert.NoError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, a))

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	expectedName := fmt.Sprintf("%s-%s-%s", a.Name, a.Spec.ControlPlaneNamespace, workloadIdentifier)
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))

	// update role reference and subject of the clusterrolebinding
	clusterRoleBinding.RoleRef.Name = "not-x"
	clusterRoleBinding.Subjects[0].Name = "not-x"
	assert.NoError(t, r.Update(context.TODO(), clusterRoleBinding))

	// try reconciling it again and verify if the changes are overwritten
	assert.NoError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, a))

	clusterRoleBinding = &rbacv1.ClusterRoleBinding{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))
}

func TestReconcileArgoCD_reconcileClusterRoleBinding_disabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestClusterArgoCD()
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	workloadIdentifier := "x"
	expectedClusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier}}

	// Disable creation of default ClusterRole, hence RoleBinding won't be created either.
	a.Spec.DefaultClusterScopedRoleDisabled = true
	err := cl.Update(context.Background(), a)
	assert.NoError(t, err)

	// Reconcile ClusterRoleBinding
	assert.NoError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, a))

	// Ensure default ClusterRoleBinding is not created
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	expectedName := fmt.Sprintf("%s-%s-%s", a.Name, a.Spec.ControlPlaneNamespace, workloadIdentifier)
	err = r.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "not found")

	// Now enable creation of default ClusterRole, hence RoleBinding should be created aw well.
	a.Spec.DefaultClusterScopedRoleDisabled = false
	err = cl.Update(context.Background(), a)
	assert.NoError(t, err)

	// Again reconcile ClusterRoleBinding
	assert.NoError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, a))

	// Ensure default ClusterRoleBinding is created now
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding))

	// Once again disable creation of default ClusterRole
	a.Spec.DefaultClusterScopedRoleDisabled = true
	err = cl.Update(context.Background(), a)
	assert.NoError(t, err)

	// Once again reconcile ClusterRoleBinding
	assert.NoError(t, r.reconcileClusterRoleBinding(workloadIdentifier, expectedClusterRole, a))

	// Ensure default ClusterRoleBinding is deleted again
	err = r.Get(context.TODO(), types.NamespacedName{Name: expectedName}, clusterRoleBinding)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "not found")
}

func TestReconcileArgoCD_reconcileRoleBinding_custom_role(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestClusterArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	p := policyRuleForApplicationController()

	assert.NoError(t, createNamespace(r, a.Spec.ControlPlaneNamespace, ""))

	workloadIdentifier := "argocd-application-controller"
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)

	namespaceWithCustomRole := "namespace-with-custom-role"
	assert.NoError(t, createNamespace(r, namespaceWithCustomRole, a.Spec.ControlPlaneNamespace))
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	// check if the default rolebindings are created
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Spec.ControlPlaneNamespace}, &rbacv1.RoleBinding{}))

	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: namespaceWithCustomRole}, &rbacv1.RoleBinding{}))

	checkForUpdatedRoleRef := func(t *testing.T, roleName, expectedName string) {
		t.Helper()
		expectedRoleRef := rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     roleName,
		}
		roleBinding := &rbacv1.RoleBinding{}
		assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Spec.ControlPlaneNamespace}, roleBinding))
		assert.Equal(t, roleBinding.RoleRef, expectedRoleRef)

		assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: namespaceWithCustomRole}, roleBinding))
		assert.Equal(t, roleBinding.RoleRef, expectedRoleRef)
	}

	t.Setenv(common.ArgoCDControllerClusterRoleEnvName, "custom-controller-role")
	assert.NoError(t, r.reconcileRoleBinding(common.ArgoCDApplicationControllerComponent, p, a))

	expectedName = fmt.Sprintf("%s-%s", a.Name, "argocd-application-controller")
	checkForUpdatedRoleRef(t, "custom-controller-role", expectedName)

	t.Setenv(common.ArgoCDServerClusterRoleEnvName, "custom-server-role")
	assert.NoError(t, r.reconcileRoleBinding("argocd-server", p, a))

	expectedName = fmt.Sprintf("%s-%s", a.Name, "argocd-server")
	checkForUpdatedRoleRef(t, "custom-server-role", expectedName)
}

func TestReconcileArgoCD_reconcileRoleBinding_forSourceNamespaces(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	// Use a long namespace to test the truncation fix
	sourceNamespace := "grp-bk-time-deposit-servicing-activity-topic-streaming-12345678"
	a := makeTestClusterArgoCD()
	a.Spec = argoproj.ClusterArgoCDSpec{
		SourceNamespaces: []string{
			sourceNamespace,
		},
	}

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	p := policyRuleForApplicationController()

	assert.NoError(t, createNamespace(r, a.Spec.ControlPlaneNamespace, ""))
	assert.NoError(t, createNamespaceManagedByClusterArgoCDLabel(r, sourceNamespace, a.Spec.ControlPlaneNamespace))

	workloadIdentifier := common.ArgoCDServerComponent

	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding := &rbacv1.RoleBinding{}
	expectedName := getRoleBindingNameForSourceNamespaces(a.Name, sourceNamespace)

	// Verify the name is truncated to maxLabelLength characters
	assert.LessOrEqual(t, len(expectedName), argoutil.GetMaxLabelLength(), "RoleBinding name should not exceed maxLabelLength")

	// Verify the RoleBinding was created successfully
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: sourceNamespace}, roleBinding))

	// Verify the RoleBinding name is exactly maxLabelLength characters
	assert.Equal(t, argoutil.GetMaxLabelLength(), len(roleBinding.Name), "RoleBinding name should be exactly maxLabelLength characters")
}

func TestTruncateWithHash(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		length      int
		description string
	}{
		{
			name:        "exactly 63 characters - no truncation needed",
			input:       "exactly-sixty-three-characters-long-string-that-is-perfect",
			length:      63,
			description: "Strings exactly at maxLength should be returned unchanged",
		},
		{
			name:        "64 characters - needs truncation",
			input:       "exactly-sixty-four-characters-long-string-that-needs-truncation",
			length:      63,
			description: "Strings longer than maxLength should be truncated with hash",
		},
		{
			name:        "very long string - needs significant truncation",
			input:       "this-is-a-very-long-string-that-will-need-to-be-truncated-significantly-to-fit-within-the-kubernetes-label-limit",
			length:      63,
			description: "Very long strings should be significantly truncated with hash",
		},
		{
			name:        "extremely long string - minimal base with hash",
			input:       "this-is-an-extremely-long-string-that-is-so-long-it-will-need-to-be-completely-replaced-by-a-hash-because-there-is-no-room-for-any-part-of-the-original-string",
			length:      63,
			description: "Extremely long strings should be truncated to fit maxLength with hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := argoutil.TruncateWithHash(tt.input, argoutil.GetMaxLabelLength())

			// Check length constraint
			assert.LessOrEqual(t, len(result), maxLabelLength, "Result should not exceed maxLabelLength")

			// Check that result is deterministic
			result2 := argoutil.TruncateWithHash(tt.input, argoutil.GetMaxLabelLength())
			assert.Equal(t, result, result2, "Function should be deterministic")

			// For short strings, should be unchanged
			if len(tt.input) <= maxLabelLength {
				assert.Equal(t, tt.input, result, "Short strings should not be modified")
			} else {
				// For long strings, should be different and shorter
				assert.NotEqual(t, tt.input, result, "Long strings should be modified")
				assert.LessOrEqual(t, len(result), maxLabelLength, "Result should be within length limit")

				// Should contain hash suffix if truncated
				if len(tt.input) > maxLabelLength {
					assert.Contains(t, result, "-", "Truncated strings should contain hash separator")
				}
			}
		})
	}
}

// TestGetRoleBindingNameForSourceNamespaces tests the updated function with various namespace lengths
func TestGetRoleBindingNameForSourceNamespaces(t *testing.T) {
	tests := []struct {
		name            string
		argocdName      string
		targetNamespace string
		expectedLength  int
	}{
		{
			name:            "short namespace",
			argocdName:      "argocd",
			targetNamespace: "short-ns",
			expectedLength:  16, // "argocd_short-ns"
		},
		{
			name:            "medium namespace",
			argocdName:      "argocd",
			targetNamespace: "medium-length-namespace-name",
			expectedLength:  35, // "argocd_medium-length-namespace-name"
		},
		{
			name:            "long namespace - needs truncation",
			argocdName:      "argocd",
			targetNamespace: "grp-bk-time-deposit-servicing-activity-topic-streaming-12345678",
			expectedLength:  63, // Should be truncated to exactly 63 chars
		},
		{
			name:            "very long namespace - needs significant truncation",
			argocdName:      "argocd",
			targetNamespace: "this-is-an-extremely-long-namespace-name-that-will-definitely-exceed-the-kubernetes-label-limit-and-need-to-be-truncated",
			expectedLength:  63, // Should be truncated to exactly 63 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getRoleBindingNameForSourceNamespaces(tt.argocdName, tt.targetNamespace)

			// Check length constraint
			assert.LessOrEqual(t, len(result), maxLabelLength, "RoleBinding name should not exceed maxLabelLength")

			// Check that result is deterministic
			result2 := getRoleBindingNameForSourceNamespaces(tt.argocdName, tt.targetNamespace)
			assert.Equal(t, result, result2, "Function should be deterministic")

			// For short namespaces, should contain original namespace name
			if len(tt.argocdName)+len(tt.targetNamespace)+1 <= maxLabelLength {
				expected := fmt.Sprintf("%s_%s", tt.argocdName, tt.targetNamespace)
				assert.Equal(t, expected, result, "Short namespaces should not be truncated")
			} else {
				// For long namespaces, should be truncated and contain hash
				assert.LessOrEqual(t, len(result), maxLabelLength, "Long namespaces should be truncated")
				assert.Contains(t, result, tt.argocdName, "Result should contain ArgoCD name")
				assert.Contains(t, result, "-", "Truncated names should contain hash separator")
			}
		})
	}
}

// TestTruncateWithHashUniqueness tests that different inputs produce different hashes
func TestTruncateWithHashUniqueness(t *testing.T) {
	inputs := []string{
		"namespace1",
		"namespace2",
		"very-long-namespace-name-that-will-be-truncated-1",
		"very-long-namespace-name-that-will-be-truncated-2",
		"argocd_grp-bk-time-deposit-servicing-activity-topic-streaming-12345678",
		"argocd_grp-bk-time-deposit-servicing-activity-topic-streaming-87654321",
	}

	results := make(map[string]bool)

	for _, input := range inputs {
		result := argoutil.TruncateWithHash(input, argoutil.GetMaxLabelLength())
		assert.False(t, results[result], "Hash should be unique for different inputs: %s", input)
		results[result] = true

		// Verify length constraint
		assert.LessOrEqual(t, len(result), maxLabelLength, "Result should not exceed maxLabelLength")
	}
}

func Test_newRoleBindingWithNameForApplicationSourceNamespaces(t *testing.T) {
	cr := makeTestClusterArgoCD()
	roleBinding := newRoleBindingWithNameForApplicationSourceNamespaces("test", cr)
	assert.Equal(t, roleBinding.Labels["app.kubernetes.io/name"], "argocd")
}
