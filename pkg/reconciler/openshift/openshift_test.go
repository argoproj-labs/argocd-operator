package openshift

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gotest.tools/assert"
)

func TestReconcileArgoCD_reconcileApplicableClusterRole(t *testing.T) {

	a := makeTestArgoCDForClusterConfig()
	testClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.Name + "-" + testApplicationController,
			Namespace: a.Namespace,
		},
		Rules: makeTestPolicyRules(),
	}
	assert.NilError(t, reconcilerHook(a, testClusterRole))

	want := append(makeTestPolicyRules(), policyRulesForClusterConfig()...)
	assert.DeepEqual(t, want, testClusterRole.Rules)
}

func TestReconcileArgoCD_reconcileNotApplicableClusterRole(t *testing.T) {

	a := makeTestArgoCDForClusterConfig()
	testClusterRole := makeTestClusterRole()

	assert.NilError(t, reconcilerHook(a, testClusterRole))
	assert.DeepEqual(t, makeTestPolicyRules(), testClusterRole.Rules)
}

func TestReconcileArgoCD_reconcileMultipleClusterRoles(t *testing.T) {

	a := makeTestArgoCDForClusterConfig()
	testApplicableClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a.Name + "-" + testApplicationController,
			Namespace: a.Namespace,
		},
		Rules: makeTestPolicyRules(),
	}

	testNotApplicableClusterRole := makeTestClusterRole()

	assert.NilError(t, reconcilerHook(a, testApplicableClusterRole))
	want := append(makeTestPolicyRules(), policyRulesForClusterConfig()...)
	assert.DeepEqual(t, want, testApplicableClusterRole.Rules)

	assert.NilError(t, reconcilerHook(a, testNotApplicableClusterRole))
	assert.DeepEqual(t, makeTestPolicyRules(), testNotApplicableClusterRole.Rules)
}

func TestReconcileArgoCD_testDeployment(t *testing.T) {

	a := makeTestArgoCDForClusterConfig()

	testDeployment := makeTestDeployment()
	// reconcilerHook should not error on a Deployment resource
	assert.NilError(t, reconcilerHook(a, testDeployment))
}

func TestAllowedNamespaces(t *testing.T) {

	argocdNamespace := testNamespace
	allowedNamespaceList := []string{"foo", "bar", testNamespace}
	assert.DeepEqual(t, true, allowedNamespace(argocdNamespace, allowedNamespaceList))

	allowedNamespaceList = []string{"*"}
	assert.DeepEqual(t, true, allowedNamespace(argocdNamespace, allowedNamespaceList))

	allowedNamespaceList = []string{"foo", "bar"}
	assert.DeepEqual(t, false, allowedNamespace(argocdNamespace, allowedNamespaceList))
}
