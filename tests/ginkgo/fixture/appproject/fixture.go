package appproject

import (
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"k8s.io/client-go/util/retry"

	appv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	matcher "github.com/onsi/gomega/types"

	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/ginkgo/v2" //nolint:all
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/gomega" //nolint:all

	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
//
//nolint:unused
func expectedCondition(f func(app *appv1alpha1.AppProject) bool) matcher.GomegaMatcher {

	return WithTransform(func(appProject *appv1alpha1.AppProject) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(appProject), appProject)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(appProject)

	}, BeTrue())

}

// Update will keep trying to update object until it succeeds, or times out.
func Update(obj *appv1alpha1.AppProject, modify func(*appv1alpha1.AppProject)) {
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

// Update will keep trying to update object until it succeeds, or times out.
func UpdateWithError(obj *appv1alpha1.AppProject, modify func(*appv1alpha1.AppProject)) error {
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

	return err
}
