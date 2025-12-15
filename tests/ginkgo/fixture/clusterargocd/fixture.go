package clusterargocd

import (
	"context"
	"time"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/ginkgo/v2" //nolint:all
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/gomega" //nolint:all

	matcher "github.com/onsi/gomega/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Update will update a ClusterArgoCD CR. Update will keep trying to update object until it succeeds, or times out.
func Update(obj *argov1beta1api.ClusterArgoCD, modify func(*argov1beta1api.ClusterArgoCD)) {
	k8sClient, _ := utils.GetE2ETestKubeClient()

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Retrieve the latest version of the object
		err := k8sClient.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)
		if err != nil {
			return err
		}

		modify(obj)

		// Attempt to update the object
		return k8sClient.Update(context.Background(), obj)
	})
	Expect(err).ToNot(HaveOccurred())

	// After we update ClusterArgoCD CR, we should wait a few moments for the operator to reconcile the change.
	time.Sleep(7 * time.Second)
}

// BeAvailable waits for ClusterArgoCD instance to have .status.phase of 'Available'
func BeAvailable() matcher.GomegaMatcher {
	return BeAvailableWithCustomSleepTime(10 * time.Second)
}

// In most cases, you should probably just use 'BeAvailable'.
func BeAvailableWithCustomSleepTime(sleepTime time.Duration) matcher.GomegaMatcher {

	// Wait X seconds to allow operator to reconcile the CR, before we start checking if it's ready
	time.Sleep(sleepTime)

	return fetchClusterArgoCD(func(argocd *argov1beta1api.ClusterArgoCD) bool {

		if argocd.Status.Phase != "Available" {
			GinkgoWriter.Println("ClusterArgoCD status is not yet Available")
			return false
		}
		GinkgoWriter.Println("ClusterArgoCD status is now", argocd.Status.Phase)

		return true
	})
}

func HaveServerStatus(status string) matcher.GomegaMatcher {
	return fetchClusterArgoCD(func(argocd *argov1beta1api.ClusterArgoCD) bool {
		GinkgoWriter.Println("HaveServerStatus:", "expected:", status, "/ actual:", argocd.Status.Server)
		return argocd.Status.Server == status
	})
}

func HaveApplicationSetControllerStatus(status string) matcher.GomegaMatcher {
	return fetchClusterArgoCD(func(argocd *argov1beta1api.ClusterArgoCD) bool {
		GinkgoWriter.Println("HaveApplicationSetControllerStatus:", "expected:", status, "/ actual:", argocd.Status.ApplicationSetController)
		return argocd.Status.ApplicationSetController == status
	})
}

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
func fetchClusterArgoCD(f func(*argov1beta1api.ClusterArgoCD) bool) matcher.GomegaMatcher {

	return WithTransform(func(argocd *argov1beta1api.ClusterArgoCD) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(argocd), argocd)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(argocd)

	}, BeTrue())

}
