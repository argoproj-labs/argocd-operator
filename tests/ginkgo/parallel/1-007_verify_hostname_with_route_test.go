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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-007_verify_hostname_with_route", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that setting a long value on appset webhook route will cause it to be accepted within Route", func() {

			Skip("Original kuttl test never ran, see GITOPS-7315. This skip added July 2025")

			fixture.EnsureRunningOnOpenShift()

			By("creating simple Argo CD instance with application set webhook enabled via Route")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-long-name-for-route-testiiiiiiiiiiiiiiiiiiiiiiiing", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						WebhookServer: argov1beta1api.WebhookServerSpec{
							Route: argov1beta1api.ArgoCDRouteSpec{
								Enabled: true,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying Route is created")
			route := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-long-name-for-route-testiiiiiiiiiiiiiiiiiiiiiiiing-appset",
					Namespace: ns.Name,
				},
			}
			Eventually(route).Should(k8sFixture.ExistByName())

			By("verifying route ingress host has a value")
			Eventually(func() bool {

				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(route), route); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				if len(route.Status.Ingress) == 0 {
					return false
				}

				return len(route.Status.Ingress[0].Host) > 0

			}).Should(BeTrue())

		})

	})
})
