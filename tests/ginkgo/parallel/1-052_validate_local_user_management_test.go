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
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/secret"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-052_validate_local_user_management", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies setting local user in ArgoCD CR creates the appropriate Argo CD resources, and that they are recreated if deleted", func() {

			By("creating namespace-scoped Argo CD instance in a new namespace, with a single local user")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					LocalUsers: []argov1beta1api.LocalUserSpec{
						{
							Name:          "alice",
							TokenLifetime: "100h",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying the Argo CD becomes available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying Secret is created for local user")
			aliceLocalUser := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "alice-local-user",
					Namespace: ns.Name,
				},
			}
			Eventually(aliceLocalUser).Should(k8sFixture.ExistByName())

			By("verifying Argo CD argocd-cm ConfigMap references user, and user is enabled")
			argocdCMConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: argoCD.Namespace,
				},
			}
			Eventually(argocdCMConfigMap).Should(k8sFixture.ExistByName())
			Eventually(argocdCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("accounts.alice", "apiKey"))
			Eventually(argocdCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("accounts.alice.enabled", "true"))

			By("verifying argocd-secret Secret contains token for user")
			argocdSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: argoCD.Namespace,
				},
			}
			Eventually(argocdSecret).Should(k8sFixture.ExistByName())
			Eventually(argocdSecret).Should(secret.HaveNonEmptyKeyValue("accounts.alice.tokens"), "Entry 'alice.account.tokens' should be found in argocd-secret")

			By("delete local user Secret")
			Expect(k8sClient.Delete(ctx, aliceLocalUser)).To(Succeed())

			By("verifying local user Secret is recreated")
			Eventually(aliceLocalUser).Should(k8sFixture.ExistByName())
			Consistently(aliceLocalUser).Should(k8sFixture.ExistByName())

		})

	})
})
