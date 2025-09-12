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
	"k8s.io/utils/ptr"

	argov1alpha1api "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-002_alpha_to_beta_keycloak_conversion", func() {

		var (
			ctx       context.Context
			k8sClient client.Client
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies SSO keycloak values in v1alpha1 ArgoCD API are translated into v1beta1 API", func() {
			if fixture.EnvLocalRun() {
				Skip("Conversion via webhook requires the operator to be running on the cluster, which is not the case for a local run")
				return
			}

			ns, nsCleanup := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer nsCleanup()

			By("creating Argo CD with Keycloak SSO and extraConfig values via v1alpha1 API")

			argoCDalpha1 := &argov1alpha1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: ns.Name,
				},
				Spec: argov1alpha1api.ArgoCDSpec{
					SSO: &argov1alpha1api.ArgoCDSSOSpec{
						Provider:  argov1alpha1api.SSOProviderTypeKeycloak,
						VerifyTLS: ptr.To(false),
					},
					ExtraConfig: map[string]string{
						"oidc.tls.insecure.skip.verify": "true",
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

			By("verifying expected Argo CD is running and has the expected values via the v1beta1 API")

			Eventually(argoCDbeta1, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCDbeta1, "2m", "5s").Should(
				And(argocdFixture.HavePhase("Available")))

			Expect(argoCDbeta1.Spec.SSO).ToNot(BeNil())
			Expect(argoCDbeta1.Spec.SSO.Provider).To(Equal(argov1beta1api.SSOProviderTypeKeycloak))
			Expect(*argoCDbeta1.Spec.SSO.Keycloak.VerifyTLS).To(BeFalse())
			Expect(argoCDbeta1.Spec.ExtraConfig["oidc.tls.insecure.skip.verify"]).To(Equal("true"))

			By("deleting ArgoCD CR via v1alpha1 API")
			Expect(k8sClient.Delete(ctx, argoCDalpha1)).To(Succeed())

			By("verifying ArgoCD CR no longer exists via v1beta1 API")
			Eventually(argoCDbeta1).Should(k8sFixture.NotExistByName())

		})

	})
})
