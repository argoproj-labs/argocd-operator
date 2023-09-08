package networking

import (
	"errors"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// common test variables used across workloads tests
var (
	testName              = "test-name"
	testInstance          = "test-instance"
	testInstanceNamespace = "test-instance-ns"
	testNamespace         = "test-ns"
	testComponent         = "test-component"
	testApplicationName   = "test-application-name"
	testKey               = "test-key"
	testVal               = "test-value"

	testServiceNameMutated = "mutated-name"
	testRouteNameMutated   = "mutated-name"
	testIngressNameMutated = "mutated-name"
	testKVP                = map[string]string{
		testKey: testVal,
	}
)

func testMutationFuncFailed(cr *v1alpha1.ArgoCD, resource interface{}, client cntrlClient.Client) error {
	return errors.New("test-mutation-error")
}

func testMutationFuncSuccessful(cr *v1alpha1.ArgoCD, resource interface{}, client cntrlClient.Client) error {
	switch obj := resource.(type) {
	case *corev1.Service:
		obj.Name = testServiceNameMutated
		return nil
	case *routev1.Route:
		obj.Name = testRouteNameMutated
		return nil
	case *networkingv1.Ingress:
		obj.Name = testIngressNameMutated
		return nil
	}
	return errors.New("test-mutation-error")
}
