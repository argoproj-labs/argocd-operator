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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1alpha1api "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-103_argocd_alpha_to_beta_conversion", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that creation of a v1alpha1 ArgoCD CR with SSO fields will be translated into a v1beta1 ArgoCD CR via webhook", func() {
			if fixture.EnvLocalRun() {
				Skip("When LOCAL_RUN is set, the API upgrade webhook is not running, which is what this test tets. Thus this test should be skipped.")
				return
			}

			By("creating a v1alpha1-API-version Argo CD instance, with Dex SSO fields")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			oldArgoCD := &argov1alpha1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1alpha1api.ArgoCDSpec{
					Dex: &argov1alpha1api.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
					Server: argov1alpha1api.ArgoCDServerSpec{
						Route: argov1alpha1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, oldArgoCD)).To(Succeed())

			By("retrieving the ArgoCD CR using the v1beta1 API")
			newArgoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
			}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(newArgoCD), newArgoCD)).To(Succeed())

			By("verifying the fields were translated successfully via the webhook, from v1alpha1 to v1beta")
			Expect(newArgoCD.Spec.SSO).ToNot(BeNil())
			Expect(newArgoCD.Spec.SSO.Provider).To(Equal(argov1beta1api.SSOProviderTypeDex))

			Expect(newArgoCD.Spec.SSO.Dex).ToNot(BeNil())
			Expect(newArgoCD.Spec.SSO.Dex.OpenShiftOAuth).To(BeTrue())

			Expect(newArgoCD.Spec.Server.Route.Enabled).To(BeTrue())

			Eventually(newArgoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(newArgoCD, "3m", "5s").Should(argocdFixture.HaveSSOStatus("Running"))

		})

	})
})
