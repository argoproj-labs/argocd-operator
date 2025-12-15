package clusterargocd

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/rbac/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

var errMsg = errors.New("this is a test error")

func testDeploymentHook(cr *argoproj.ClusterArgoCD, v interface{}, s string) error {
	switch o := v.(type) {
	case *appsv1.Deployment:
		var replicas int32 = 3
		o.Spec.Replicas = &replicas
	}
	return nil
}

func testClusterRoleHook(cr *argoproj.ClusterArgoCD, v interface{}, s string) error {
	switch o := v.(type) {
	case *v1.ClusterRole:
		o.Rules = append(o.Rules, policyRuleForApplicationController()...)
	}
	return nil
}

func testRoleHook(cr *argoproj.ClusterArgoCD, v interface{}, s string) error {
	switch o := v.(type) {
	case *v1.Role:
		if o.Name == cr.Name+"-"+common.ArgoCDApplicationControllerComponent {
			o.Rules = append(o.Rules, testRules()...)
		}
	}
	return nil
}

func testErrorHook(cr *argoproj.ClusterArgoCD, v interface{}, s string) error {
	return errMsg
}

func TestReconcileArgoCD_testDeploymentHook(t *testing.T) {
	defer resetHooks()()
	a := makeTestClusterArgoCD()

	Register(testDeploymentHook)

	testDeployment := makeTestDeployment()

	assert.NoError(t, applyReconcilerHook(a, testDeployment, ""))
	var expectedReplicas int32 = 3
	assert.Equal(t, &expectedReplicas, testDeployment.Spec.Replicas)
}

func TestReconcileArgoCD_testMultipleHooks(t *testing.T) {
	defer resetHooks()()
	a := makeTestClusterArgoCD()

	testDeployment := makeTestDeployment()
	testClusterRole := makeTestClusterRole()

	Register(testDeploymentHook)
	Register(testClusterRoleHook)

	assert.NoError(t, applyReconcilerHook(a, testDeployment, ""))
	assert.NoError(t, applyReconcilerHook(a, testClusterRole, ""))

	// Verify if testDeploymentHook is executed successfully
	var expectedReplicas int32 = 3
	assert.Equal(t, &expectedReplicas, testDeployment.Spec.Replicas)

	// Verify if testClusterRoleHook is executed successfully
	want := append(makeTestPolicyRules(), policyRuleForApplicationController()...)
	assert.Equal(t, want, testClusterRole.Rules)
}

func TestReconcileArgoCD_hooks_end_upon_error(t *testing.T) {
	defer resetHooks()()
	a := makeTestClusterArgoCD()
	Register(testErrorHook, testClusterRoleHook)

	testClusterRole := makeTestClusterRole()

	assert.Error(t, applyReconcilerHook(a, testClusterRole, ""), "this is a test error")
	assert.Equal(t, makeTestPolicyRules(), testClusterRole.Rules)
}

func resetHooks() func() {
	origDefaultHooksFunc := hooks

	return func() {
		hooks = origDefaultHooksFunc
	}
}
