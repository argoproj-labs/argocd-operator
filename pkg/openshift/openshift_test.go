package openshift

import (
	"errors"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
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
	testNameMutated       = "mutated-name"
	testReplicasMutated   = int32(4)
	testRouteNameMutated  = "mutated-name"
	testKVP               = map[string]string{
		testKey: testVal,
	}
)

func testMutationFuncFailed(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	return errors.New("test-mutation-error")
}

func testMutationFuncSuccessful(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	switch obj := resource.(type) {
	case *oappsv1.DeploymentConfig:
		obj.Name = testNameMutated
		obj.Spec.Replicas = testReplicasMutated
		return nil
	case *routev1.Route:
		obj.Name = testRouteNameMutated
		return nil
	}

	return errors.New("test-mutation-error")
}
