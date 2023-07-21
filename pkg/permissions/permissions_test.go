package permissions

import (
	"errors"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// common test variables used across permissions tests
var (
	testName              = "test-name"
	testInstance          = "test-instance"
	testInstanceNamespace = "test-instance-ns"
	testNamespace         = "test-ns"
	testComponent         = "test-component"
	testRules             = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"create",
			},
		},
	}
	testRulesMutated = []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"get",
			},
		},
	}
	testRoleRef = rbacv1.RoleRef{
		Kind:     "Role",
		Name:     testName,
		APIGroup: "rbac.authorization.k8s.io",
	}
	testSubjects = []rbacv1.Subject{
		{
			Kind:      "ServiceAccount",
			Name:      testName,
			Namespace: testNamespace,
		},
	}
	testKVP = map[string]string{
		"test-key": "test-value",
	}
)

func testMutationFuncFailed(cr *v1alpha1.ArgoCD, resource interface{}, client interface{}) error {
	return errors.New("")
}

func testMutationFuncSuccessful(cr *v1alpha1.ArgoCD, resource interface{}, client interface{}) error {
	switch obj := resource.(type) {
	case *rbacv1.Role:
		if obj.Namespace == testNamespace {
			obj.Rules = testRulesMutated
			return nil
		}
	case *rbacv1.ClusterRole:
		obj.Rules = testRulesMutated
		return nil
	}
	return errors.New("")
}
