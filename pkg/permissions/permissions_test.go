package permissions

import (
	"errors"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// common test variables used across permissions tests
var (
	testName              = "test-name"
	testInstance          = "test-instance"
	testInstanceNamespace = "test-instance-ns"
	testNamespace         = "test-ns"
	testComponent         = "test-component"
	testKey               = "test-key"
	testVal               = "test-value"

	testRules = []rbacv1.PolicyRule{
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
			Kind:      rbacv1.ServiceAccountKind,
			Name:      testName,
			Namespace: testNamespace,
		},
	}
	testKVP = map[string]string{
		testKey: testVal,
	}
)

func testMutationFuncFailed(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client) error {
	return errors.New("test-mutation-error")
}

func testMutationFuncSuccessful(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client) error {
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
	return errors.New("test-mutation-error")
}
