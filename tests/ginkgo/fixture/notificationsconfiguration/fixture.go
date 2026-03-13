package notificationsconfiguration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	matcher "github.com/onsi/gomega/types"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1alpha1api "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

// Update will keep trying to update object until it succeeds, or times out.
func Update(obj *argov1alpha1api.NotificationsConfiguration, modify func(*argov1alpha1api.NotificationsConfiguration)) {
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
}

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
//
//nolint:unused
func fetchNotificationsConfiguration(f func(*argov1alpha1api.NotificationsConfiguration) bool) matcher.GomegaMatcher {

	return WithTransform(func(depl *argov1alpha1api.NotificationsConfiguration) bool {

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
