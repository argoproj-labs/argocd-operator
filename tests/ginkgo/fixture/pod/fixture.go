package pod

import (
	"context"
	"regexp"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/ginkgo/v2" //nolint:all
	//lint:ignore ST1001 "This is a common practice in Gomega tests for readability."
	. "github.com/onsi/gomega" //nolint:all
	matcher "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetPodByNameRegexp(k8sClient client.Client, nameRegexp *regexp.Regexp, options ...client.ListOption) *corev1.Pod {
	var pods []corev1.Pod
	list := &corev1.PodList{}
	err := k8sClient.List(context.Background(), list, options...)
	Expect(err).To(BeNil())
	for _, pod := range list.Items {
		if nameRegexp.MatchString(pod.Name) {
			pods = append(pods, pod)
		}
	}

	Expect(pods).Should(HaveLen(1), "expected a single pod matching "+nameRegexp.String())

	return &pods[0]
}

func GetSpecInitContainerByName(name string, pod corev1.Pod) *corev1.Container {

	for idx := range pod.Spec.InitContainers {

		container := pod.Spec.InitContainers[idx]
		if container.Name == name {
			return &container
		}
	}

	return nil
}

func GetSpecContainerByName(name string, pod corev1.Pod) *corev1.Container {

	for idx := range pod.Spec.Containers {

		container := pod.Spec.Containers[idx]
		if container.Name == name {
			return &container
		}
	}

	return nil
}

func HavePhase(phase corev1.PodPhase) matcher.GomegaMatcher {
	return fetchPod(func(pod *corev1.Pod) bool {
		GinkgoWriter.Println("Pod HavePhase:", "expected: ", phase, "actual: ", pod.Status.Phase)
		return pod.Status.Phase == phase
	})
}

// This is intentionally NOT exported, for now. Create another function in this file/package that calls this function, and export that.
func fetchPod(f func(*corev1.Pod) bool) matcher.GomegaMatcher {

	return WithTransform(func(pod *corev1.Pod) bool {

		k8sClient, _, err := utils.GetE2ETestKubeClientWithError()
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(pod), pod)
		if err != nil {
			GinkgoWriter.Println(err)
			return false
		}

		return f(pod)

	}, BeTrue())

}
