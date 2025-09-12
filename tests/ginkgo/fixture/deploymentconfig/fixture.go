package deploymentconfig

import (
	"context"

	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/ginkgo/v2" //nolint:all

	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/gomega" //nolint:all

	matcher "github.com/onsi/gomega/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	openshiftappsv1 "github.com/openshift/api/apps/v1"
)

func HaveReplicas(replicas int) matcher.GomegaMatcher {
	return fetchDeploymentConfig(func(depl *openshiftappsv1.DeploymentConfig) bool {
		GinkgoWriter.Println("DeploymentConfig - HaveReplicas:", "expected: ", replicas, "actual: ", depl.Status.Replicas)
		return int(depl.Status.Replicas) == replicas && depl.Generation == depl.Status.ObservedGeneration
	})
}

func HaveReadyReplicas(readyReplicas int) matcher.GomegaMatcher {
	return fetchDeploymentConfig(func(depl *openshiftappsv1.DeploymentConfig) bool {
		GinkgoWriter.Println("DeploymentConfig - HaveReadyReplicas:", "expected: ", readyReplicas, "actual: ", depl.Status.ReadyReplicas)
		return int(depl.Status.ReadyReplicas) == readyReplicas && depl.Generation == depl.Status.ObservedGeneration
	})
}

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
func fetchDeploymentConfig(f func(*openshiftappsv1.DeploymentConfig) bool) matcher.GomegaMatcher {

	return WithTransform(func(depl *openshiftappsv1.DeploymentConfig) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(depl), depl)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(depl)

	}, BeTrue())

}
