package workloads

import (
	"errors"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	oappsv1 "github.com/openshift/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
)

// common test variables used across workloads tests
var (
	testName              = "test-name"
	testInstance          = "test-instance"
	testInstanceNamespace = "test-instance-ns"
	testNamespace         = "test-ns"
	testComponent         = "test-component"
	testKey               = "test-key"
	testVal               = "test-value"

	testDeploymentNameMutated              = "mutated-name"
	testStatefulSetNameMutated             = "mutated-name"
	testDeploymentConfigNameMutated        = "mutated-name"
	testSecretNameMutated                  = "mutated-name"
	testConfigMapNameMutated               = "mutated-name"
	testHorizontalPodAutoscalerNameMutated = "mutated-name"
	testKVP                                = map[string]string{
		testKey: testVal,
	}
)

func testMutationFuncFailed(cr *v1alpha1.ArgoCD, resource interface{}, client interface{}) error {
	return errors.New("test-mutation-error")
}

func testMutationFuncSuccessful(cr *v1alpha1.ArgoCD, resource interface{}, client interface{}) error {
	switch obj := resource.(type) {
	case *appsv1.Deployment:
		obj.Name = testDeploymentNameMutated
		return nil
	case *appsv1.StatefulSet:
		obj.Name = testStatefulSetNameMutated
		return nil
	case *oappsv1.DeploymentConfig:
		obj.Name = testDeploymentConfigNameMutated
		return nil
	case *corev1.Secret:
		obj.Name = testSecretNameMutated
		return nil
	case *corev1.ConfigMap:
		obj.Name = testConfigMapNameMutated
		return nil
	case *autoscaling.HorizontalPodAutoscaler:
		obj.Name = testHorizontalPodAutoscalerNameMutated
		return nil
	}
	return errors.New("test-mutation-error")
}
