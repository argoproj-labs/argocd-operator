package permissions

import (
	"errors"

	rbacv1 "k8s.io/api/rbac/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

// common test variables used across permissions tests
var (
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
)

func testMutationFuncSuccessful(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	switch obj := resource.(type) {
	case *rbacv1.Role:
		if obj.Namespace == test.TestNamespace {
			obj.Rules = testRulesMutated
			return nil
		}
	case *rbacv1.ClusterRole:
		obj.Rules = testRulesMutated
		return nil
	}
	return errors.New("test-mutation-error")
}
