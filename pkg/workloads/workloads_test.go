package workloads

import (
	"errors"

	"github.com/argoproj-labs/argocd-operator/api/v1beta1"
	oappsv1 "github.com/openshift/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
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
	testValMutated        = "test-value-mutated"

	testNameMutated     = "mutated-name"
	testReplicasMutated = int32(4)
	testKVP             = map[string]string{
		testKey: testVal,
	}
	testKVPMutated = map[string]string{
		testKey: testValMutated,
	}
)

func testMutationFuncFailed(cr *v1beta1.ArgoCD, resource interface{}, client cntrlClient.Client) error {
	return errors.New("test-mutation-error")
}

func testMutationFuncSuccessful(cr *v1beta1.ArgoCD, resource interface{}, client cntrlClient.Client) error {
	switch obj := resource.(type) {
	case *appsv1.Deployment:
		obj.Name = testNameMutated
		obj.Spec.Replicas = &testReplicasMutated
		return nil
	case *appsv1.StatefulSet:
		obj.Name = testNameMutated
		obj.Spec.Replicas = &testReplicasMutated
		return nil
	case *oappsv1.DeploymentConfig:
		obj.Name = testNameMutated
		obj.Spec.Replicas = testReplicasMutated
		return nil
	case *corev1.Secret:
		obj.Name = testNameMutated
		obj.StringData = testKVPMutated
		return nil
	case *corev1.ConfigMap:
		obj.Name = testNameMutated
		obj.Data = testKVPMutated
		return nil
	case *autoscaling.HorizontalPodAutoscaler:
		obj.Name = testNameMutated
		obj.Spec.MaxReplicas = testReplicasMutated
		return nil
	}
	return errors.New("test-mutation-error")
}
