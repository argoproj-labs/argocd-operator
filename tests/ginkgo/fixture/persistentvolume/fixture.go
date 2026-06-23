package persistentvolume

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	matcher "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

func HavePhase(phase corev1.PersistentVolumePhase) matcher.GomegaMatcher {
	return fetchPersistentVolume(func(pv *corev1.PersistentVolume) bool {
		GinkgoWriter.Println("PersistentVolume HavePhase:", "expected: ", phase, "actual: ", pv.Status.Phase)
		return pv.Status.Phase == phase
	})
}

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
func fetchPersistentVolume(f func(*corev1.PersistentVolume) bool) matcher.GomegaMatcher {

	return WithTransform(func(pv *corev1.PersistentVolume) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(pv), pv)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(pv)

	}, BeTrue())

}
