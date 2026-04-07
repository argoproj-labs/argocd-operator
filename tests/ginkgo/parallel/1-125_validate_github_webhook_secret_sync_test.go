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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	secretFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/secret"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-125_validate_github_webhook_secret_sync", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies spec.webhookSecrets.github.secretRef is synced into argocd-secret as webhook.github.secret", func() {

			By("creating Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret).Should(k8sFixture.ExistByName())

			By("creating user Secret with GitHub webhook token")
			expectedToken := "e2e-github-webhook-secret-token"
			userSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "github-webhook-credentials", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"token": expectedToken},
			}
			Expect(k8sClient.Create(ctx, userSecret)).To(Succeed())

			By("setting spec.webhookSecrets.github.secretRef on the ArgoCD CR")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.WebhookSecrets = &argov1beta1api.ArgoCDWebhookSecretsSpec{
				GitHub: &argov1beta1api.ArgoCDWebhookSecretsGitHub{
					SecretRef: &argov1beta1api.WebhookSecretKeySelector{
						Name: "github-webhook-credentials",
						Key:  "token",
					},
				},
			}
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())

			By("waiting for argocd-secret to contain webhook.github.secret matching the referenced Secret")
			Eventually(argocdSecret, "2m", "3s").Should(
				secretFixture.HaveDataKeyValue(common.ArgoCDKeyGitHubWebhookSecret, []byte(expectedToken)),
			)
		})
	})
})
