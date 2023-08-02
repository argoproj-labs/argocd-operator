package workloads

import (
	"errors"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	// "github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	appsv1 "k8s.io/api/apps/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// common test variables used across workloads tests
var (
	testName     = "test-name"
	testInstance = "test-instance"
	// testInstanceNamespace = "test-instance-ns"
	testNamespace = "test-ns"
	testComponent = "test-component"
	testKey       = "test-key"
	testVal       = "test-value"

	testDeploymentNameMutated = "mutated-name"
	testKVP                   = map[string]string{
		testKey: testVal,
	}
)

func testMutationFuncFailed(cr *v1alpha1.ArgoCD, resource interface{}, client interface{}) error {
	return errors.New("test-mutation-error")
}

func testMutationFuncSuccessful(cr *v1alpha1.ArgoCD, resource interface{}, client interface{}) error {
	obj := resource.(*appsv1.Deployment)
	obj.Name = testDeploymentNameMutated
	return nil
}
