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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1api "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-004_beta_to_alpha_conversion", func() {

		var (
			ctx       context.Context
			k8sClient client.Client
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies v1beta1 ArgoCD CR containing Dex SSO values can be converted to v1alpha1", func() {

			if fixture.EnvLocalRun() {
				Skip("Conversion via webhook requires the operator to be running on the cluster, which is not the case for a local run")
				return
			}

			ns, nsCleanup := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer nsCleanup()

			By("creating Argo CD with dex and server values via v1beta1 API")

			argoCDBeta1 := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeDex,
						Dex: &argov1beta1api.ArgoCDDexSpec{
							OpenShiftOAuth: true,
						},
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCDBeta1)).To(Succeed())

			argoCDAlpha1 := &argov1alpha1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: ns.Name,
				},
			}
			Expect(argoCDAlpha1).Should(k8sFixture.ExistByName())

			// During beta to alpha conversion, converting sso fields back to deprecated fields is ignored as there is no data loss since the new fields in v1beta1 are also present in v1alpha1
			By("verifying expected resources exist using v1alpha1 API")

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCDAlpha1), argoCDAlpha1); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				return argoCDAlpha1.Status.Phase == "Available"

			}, "4m", "5s").Should(BeTrue())

			Expect(argoCDAlpha1.Spec.SSO.Provider).To(Equal(argov1alpha1api.SSOProviderTypeDex))
			Expect(argoCDAlpha1.Spec.SSO.Dex.OpenShiftOAuth).To(BeTrue())
			Expect(argoCDAlpha1.Spec.Server.Route.Enabled).To(BeTrue())

			dexDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-dex-server",
					Namespace: ns.Name,
				},
			}
			Eventually(dexDeployment).Should(k8sFixture.ExistByName())
			Eventually(dexDeployment, "3m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))

			By("deleting ArgoCD CR via v1alpha1 API")
			Expect(k8sClient.Delete(ctx, argoCDAlpha1)).To(Succeed())

			By("verifying ArgoCD CR no long exists via v1alpha1 API")
			Eventually(argoCDAlpha1).Should(k8sFixture.NotExistByName())

			By("verifying ArgoCD CR no long exists via v1beta1 API")
			Expect(argoCDBeta1).To(k8sFixture.NotExistByName())

			By("verifying dex deployment is deleted as well")
			Eventually(dexDeployment).Should(k8sFixture.NotExistByName())

		})

	})
})
