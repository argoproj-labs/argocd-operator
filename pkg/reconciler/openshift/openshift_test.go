package openshift

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gotest.tools/assert"
)

func TestReconcileArgoCD_reconcileApplicableClusterRole(t *testing.T) {

	setClusterConfigNamespaces()
	defer unSetClusterConfigNamespaces()

	a := makeTestArgoCDForClusterConfig()
	testClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.Name + "-" + testApplicationController,
		},
		Rules: makeTestPolicyRules(),
	}
	assert.NilError(t, reconcilerHook(a, testClusterRole))

	want := append(makeTestPolicyRules(), policyRulesForClusterConfig()...)
	assert.DeepEqual(t, want, testClusterRole.Rules)
}

func TestReconcileArgoCD_reconcileNotApplicableClusterRole(t *testing.T) {

	setClusterConfigNamespaces()
	defer unSetClusterConfigNamespaces()

	a := makeTestArgoCDForClusterConfig()
	testClusterRole := makeTestClusterRole()

	assert.NilError(t, reconcilerHook(a, testClusterRole))
	assert.DeepEqual(t, makeTestPolicyRules(), testClusterRole.Rules)
}

func TestReconcileArgoCD_reconcileMultipleClusterRoles(t *testing.T) {

	setClusterConfigNamespaces()
	defer unSetClusterConfigNamespaces()

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

	setClusterConfigNamespaces()
	defer unSetClusterConfigNamespaces()

	a := makeTestArgoCDForClusterConfig()
	testDeployment := makeTestDeployment()
	// reconcilerHook should not error on a Deployment resource
	assert.NilError(t, reconcilerHook(a, testDeployment))
}

func TestReconcileArgoCD_notInClusterConfigNamespaces(t *testing.T) {

	setClusterConfigNamespaces()
	defer unSetClusterConfigNamespaces()

	a := makeTestArgoCD()
	testClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: a.Name + "-" + testApplicationController,
		},
		Rules: makeTestPolicyRules(),
	}
	assert.NilError(t, reconcilerHook(a, testClusterRole))

	want := makeTestPolicyRules()
	assert.DeepEqual(t, want, testClusterRole.Rules)
}

func TestAllowedNamespaces(t *testing.T) {

	argocdNamespace := testNamespace
	clusterConfigNamespaces := "foo,bar,argocd"
	assert.DeepEqual(t, true, allowedNamespace(argocdNamespace, clusterConfigNamespaces))

	clusterConfigNamespaces = "foo, bar, argocd"
	assert.DeepEqual(t, true, allowedNamespace(argocdNamespace, clusterConfigNamespaces))

	clusterConfigNamespaces = "*"
	assert.DeepEqual(t, true, allowedNamespace(argocdNamespace, clusterConfigNamespaces))

	clusterConfigNamespaces = "foo,bar"
	assert.DeepEqual(t, false, allowedNamespace(argocdNamespace, clusterConfigNamespaces))
}

func TestReconcileArgoCD_reconcileRedisDeployment(t *testing.T) {
	a := makeTestArgoCD()
	testDeployment := makeTestDeployment()

	testDeployment.ObjectMeta.Name = a.Name + "-" + "redis"
	want := append([]string{"redis-server"}, testDeployment.Spec.Template.Spec.Containers[0].Args ...)

	assert.NilError(t, reconcilerHook(a, testDeployment))
	assert.DeepEqual(t, testDeployment.Spec.Template.Spec.Containers[0].Args, want)

	testDeployment.ObjectMeta.Name = a.Name + "-" + "not-redis"
	want = testDeployment.Spec.Template.Spec.Containers[0].Args

	assert.NilError(t, reconcilerHook(a, testDeployment))
	assert.DeepEqual(t, testDeployment.Spec.Template.Spec.Containers[0].Args, want)
}
