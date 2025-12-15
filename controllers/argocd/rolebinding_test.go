package argocd

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
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
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	p := policyRuleForApplicationController()

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespace(r, "newTestNamespace", a.Namespace))

	workloadIdentifier := common.ArgoCDApplicationControllerComponent

	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding := &rbacv1.RoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: "newTestNamespace"}, roleBinding))

	// update role reference and subject of the rolebinding
	roleBinding.RoleRef.Name = "not-xrb"
	roleBinding.Subjects[0].Name = "not-xrb"
	assert.NoError(t, r.Update(context.TODO(), roleBinding))

	// try reconciling it again and verify if the changes are overwritten
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	roleBinding = &rbacv1.RoleBinding{}
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))
}

func TestReconcileArgoCD_reconcileRoleBinding_for_new_namespace(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	assert.NoError(t, createNamespace(r, a.Namespace, ""))
	assert.NoError(t, createNamespace(r, "newTestNamespace", a.Namespace))

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

func TestReconcileArgoCD_reconcileRoleBinding_custom_role(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	p := policyRuleForApplicationController()

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	workloadIdentifier := "argocd-application-controller"
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)

	namespaceWithCustomRole := "namespace-with-custom-role"
	assert.NoError(t, createNamespace(r, namespaceWithCustomRole, a.Namespace))
	assert.NoError(t, r.reconcileRoleBinding(workloadIdentifier, p, a))

	// check if the default rolebindings are created
	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, &rbacv1.RoleBinding{}))

	assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: namespaceWithCustomRole}, &rbacv1.RoleBinding{}))

	checkForUpdatedRoleRef := func(t *testing.T, roleName, expectedName string) {
		t.Helper()
		expectedRoleRef := rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     roleName,
		}
		roleBinding := &rbacv1.RoleBinding{}
		assert.NoError(t, r.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, roleBinding))
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
