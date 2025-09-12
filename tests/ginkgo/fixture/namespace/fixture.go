package namespace

import (
	"context"

	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/ginkgo/v2" //nolint:all
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/gomega" //nolint:all

	matcher "github.com/onsi/gomega/types"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

func HavePhase(expectedPhase corev1.NamespacePhase) matcher.GomegaMatcher {
	return fetchNamespace(func(ns *corev1.Namespace) bool {
		GinkgoWriter.Println("Namespace - HavePhase: Expected:", expectedPhase, "/ Actual:", ns.Status.Phase)
		return ns.Status.Phase == expectedPhase
	})
}

func HaveLabel(key string, value string) matcher.GomegaMatcher {
	return fetchNamespace(func(ns *corev1.Namespace) bool {
		GinkgoWriter.Println("Namespace - HaveLabel: Key:", key, "Expected:", value, "/ Actual:", ns.Labels[key])
		return ns.Labels[key] == value
	})
}

// Update will keep trying to update object until it succeeds, or times out.
func Update(obj *corev1.Namespace, modify func(*corev1.Namespace)) {
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
func fetchNamespace(f func(*corev1.Namespace) bool) matcher.GomegaMatcher {

	return WithTransform(func(depl *corev1.Namespace) bool {

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
