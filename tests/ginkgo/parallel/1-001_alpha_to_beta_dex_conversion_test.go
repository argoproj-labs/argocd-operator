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

	argov1alpha1api "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-001_alpha_to_beta_dex_conversion", func() {

		var (
			ctx       context.Context
			k8sClient client.Client
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("Ensure dex spec field can be converted between ArgoCD v1alpha1 and v1beta1", func() {

			if fixture.EnvLocalRun() {
				Skip("Conversion via webhook requires the operator to be running on the cluster, which is not the case for a local run")
				return
			}

			ns, nsCleanup := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer nsCleanup()

			By("creating Argo CD with dex and server values via v1alpha1 API")

			argoCDalpha1 := &argov1alpha1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: ns.Name,
				},
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
			Expect(k8sClient.Create(ctx, argoCDalpha1)).To(Succeed())

			argoCDbeta1 := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: ns.Name,
				},
			}
			Expect(argoCDbeta1).Should(k8sFixture.ExistByName())

			By("verifying expected resources exist using v1beta1 API")

			Eventually(argoCDbeta1, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCDbeta1, "2m", "5s").Should(
				And(argocdFixture.HavePhase("Available"), argocdFixture.HaveSSOStatus("Running")))

			Expect(argoCDbeta1.Spec.SSO.Provider).To(Equal(argov1beta1api.SSOProviderTypeDex))
			Expect(argoCDbeta1.Spec.SSO.Dex.OpenShiftOAuth).To(BeTrue())
			Expect(argoCDbeta1.Spec.Server.Route.Enabled).To(BeTrue())

			By("deleting ArgoCD CR via v1alpha1 API")
			Expect(k8sClient.Delete(ctx, argoCDalpha1)).To(Succeed())

			By("verifying ArgoCD CR no long exists via v1beta1 API")
			Eventually(argoCDbeta1).Should(k8sFixture.NotExistByName())

		})

	})
})
