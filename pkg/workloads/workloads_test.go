package workloads

import (
	"errors"

	oappsv1 "github.com/openshift/api/apps/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

// common test variables used across workloads tests
var (
	testReplicasMutated = int32(4)
)

func testMutationFuncSuccessful(cr *argoproj.ArgoCD, resource interface{}, client cntrlClient.Client, args ...interface{}) error {
	switch obj := resource.(type) {
	case *appsv1.Deployment:
		obj.Name = test.TestNameMutated
		obj.Spec.Replicas = &testReplicasMutated
		return nil
	case *appsv1.StatefulSet:
		obj.Name = test.TestNameMutated
		obj.Spec.Replicas = &testReplicasMutated
		return nil
	case *oappsv1.DeploymentConfig:
		obj.Name = test.TestNameMutated
		obj.Spec.Replicas = testReplicasMutated
		return nil
	case *corev1.Secret:
		obj.Name = test.TestNameMutated
		obj.StringData = test.TestKVPMutated
		return nil
	case *corev1.ConfigMap:
		obj.Name = test.TestNameMutated
		obj.Data = test.TestKVPMutated
		return nil
	case *autoscaling.HorizontalPodAutoscaler:
		obj.Name = test.TestNameMutated
		obj.Spec.MaxReplicas = testReplicasMutated
		return nil
	}
	return errors.New("test-mutation-error")
}
