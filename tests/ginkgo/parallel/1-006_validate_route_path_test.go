/*
Copyright 2026.

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-006_validate_route_path", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that path parameter can be configured on server Route", func() {

			fixture.EnsureRunningOnOpenShift()

			ns, nsCleanup := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer nsCleanup()

			By("creating Argo CD with server route path configured")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
							Path:    "/argocd",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying server route exists with correct path")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD, "2m", "5s").Should(argocdFixture.HaveServerStatus("Running"))

			serverRoute := &routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "example-server", Namespace: ns.Name}}
			Eventually(serverRoute).Should(k8sFixture.ExistByName())
			Expect(serverRoute.Spec.Path).To(Equal("/argocd"))

			By("updating path to a different value")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Route.Path = "/argo"
			})

			By("verifying route path is updated")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverRoute), serverRoute); err != nil {
					GinkgoWriter.Println(err)
					return false
				}
				return serverRoute.Spec.Path == "/argo"
			}, "2m", "5s").Should(BeTrue())

			By("removing path configuration")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Route.Path = ""
			})

			By("verifying route path is empty when not configured")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverRoute), serverRoute); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				return serverRoute.Spec.Path == ""

			}, "2m", "5s").Should(BeTrue())

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD).Should(argocdFixture.HaveServerStatus("Running"))

		})
	})
})
