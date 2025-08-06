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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-016_validate_keycloak", func() {

		var (
			ctx       context.Context
			k8sClient client.Client
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that keycloak resources are created when keycloak provider is enabled", func() {

			By("creating new namespace-scoped Argo CD instance with Keycloak enabled, and Argo CD Server ingress enabled")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-keycloak", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeKeycloak,
						Keycloak: &argov1beta1api.ArgoCDKeycloakSpec{
							VerifyTLS: ptr.To(false), // required when running operator locally
						},
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Ingress: argov1beta1api.ArgoCDIngressSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying ArgoCD CR becomes ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying expected Keycloak resources are created")

			// This behaviour is non-OpenShift only
			if !fixture.RunningOnOpenShift() {
				keycloakDepl := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "keycloak",
						Namespace: ns.Name,
					},
				}
				Eventually(keycloakDepl).Should(deploymentFixture.HaveReplicas(1))
			}

			keycloakService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "keycloak",
					Namespace: ns.Name,
				},
			}
			Eventually(keycloakService).Should(k8sFixture.ExistByName())

			// This behaviour is non-OpenShift only
			if !fixture.RunningOnOpenShift() {
				ingress := &networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "keycloak",
						Namespace: ns.Name,
					},
				}
				Eventually(ingress).Should(k8sFixture.ExistByName())
			}
		})

	})
})
