/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package parallel

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/node"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-118_validate_redis_ssc", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()

		})

		It("verifies that HA pods have expected SSC", func() {

			fixture.EnsureRunningOnOpenShift() // SSC requires OpenShift

			// This test enables HA redis and thus requires at least 3 nodes
			node.ExpectHasAtLeastXNodes(3)

			By("creating basic Argo CD instance with HA enabled and waiting for it to be available")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					HA: argov1beta1api.ArgoCDHASpec{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.HaveRedisStatus("Running"))

			By("verifying argocd-redis-ha pod defaults to expected SCC")
			Eventually(func() bool {
				var podList corev1.PodList
				if err := k8sClient.List(ctx, &podList, &client.ListOptions{Namespace: ns.Name}); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				podFound := false
				for _, pod := range podList.Items {

					if !strings.Contains(pod.Name, "argocd-redis-ha") {
						continue
					}
					podFound = true
					actualSCC := pod.ObjectMeta.Annotations["openshift.io/scc"]

					if actualSCC != "restricted-v2" {
						return false
					}
				}

				// Pass if we found redis pod, and it had the correct SCC
				return podFound

			}).Should(BeTrue())

			By("verifying argocd-redis-ha-haproxy pod defaults to expected SCC")
			Eventually(func() bool {
				var podList corev1.PodList
				if err := k8sClient.List(ctx, &podList, &client.ListOptions{Namespace: ns.Name}); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				podFound := false
				for _, pod := range podList.Items {

					if !strings.Contains(pod.Name, "argocd-redis-ha-haproxy") {
						continue
					}
					podFound = true
					actualSCC := pod.ObjectMeta.Annotations["openshift.io/scc"]

					if actualSCC != "restricted-v2" {
						return false
					}
				}

				// Pass if we found redis pod, and it had the correct SCC
				return podFound

			}).Should(BeTrue())

		})

	})
})
