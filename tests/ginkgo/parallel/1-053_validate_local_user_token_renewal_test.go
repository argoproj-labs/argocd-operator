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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-053_validate_local_user_token_renewal", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("creates ArgoCD CR with a local user with short token lifetime, and then verifies the token is renewed every 10 seconds", func() {

			By("creating namespace-scoped Argo CD instance in a new namespace, with a single local user defined, but the user token has a lifetime of only 10 seconds")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					LocalUsers: []argov1beta1api.LocalUserSpec{
						{
							Name:          "alice",
							TokenLifetime: "10s", // Note the lifetime of 10 seconds; it should expire quickly
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

			argocdSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: argoCD.Namespace,
				},
			}
			Eventually(argocdSecret).Should(k8sFixture.ExistByName())
			firstVal := (string)(argocdSecret.Data["accounts.alice.tokens"])
			Expect(firstVal).ToNot(BeEmpty())

			By("sleeping 15 seconds, to ensure token expires, and is replaced")
			time.Sleep(15 * time.Second)

			Expect(argocdSecret).Should(k8sFixture.ExistByName())
			secondVal := (string)(argocdSecret.Data["accounts.alice.tokens"])
			Expect(secondVal).ToNot(BeEmpty())

			Expect(firstVal).ToNot(Equal(secondVal))

			By("sleeping 11 seconds, to ensure token expires, and is replaced")
			time.Sleep(11 * time.Second)
			Expect(argocdSecret).Should(k8sFixture.ExistByName())
			thirdVal := (string)(argocdSecret.Data["accounts.alice.tokens"])
			Expect(thirdVal).ToNot(BeEmpty())
			Expect(secondVal).ToNot(Equal(thirdVal))

		})

	})
})
