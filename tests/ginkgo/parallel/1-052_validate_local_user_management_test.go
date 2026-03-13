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
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	secretFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/secret"
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

			By("verifying the Argo CD instance becomes available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			aliceLocalUser := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "alice-local-user",
					Namespace: ns.Name,
				},
			}
			argocdCMConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: argoCD.Namespace,
				},
			}
			argocdSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-secret",
					Namespace: argoCD.Namespace,
				},
			}

			// verifyConfigurationIsAsExpected verifies all related resources (ConfigMap, Secret, etc) exist and have expected value
			verifyConfigurationIsAsExpected := func(description string) {

				By(description + " - verifying Secret exists for local user")
				Eventually(aliceLocalUser).Should(k8sFixture.ExistByName())
				Consistently(aliceLocalUser, "5s", "1s").Should(k8sFixture.ExistByName())

				By(description + "- verifying Argo CD argocd-cm ConfigMap references user, and user is enabled")
				Eventually(argocdCMConfigMap).Should(k8sFixture.ExistByName())
				Eventually(argocdCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("accounts.alice", "apiKey"))
				Consistently(argocdCMConfigMap, "5s", "1s").Should(configmapFixture.HaveStringDataKeyValue("accounts.alice", "apiKey"))
				Eventually(argocdCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("accounts.alice.enabled", "true"))

				By(description + "- verifying argocd-secret Secret contains token for user")
				Eventually(argocdSecret).Should(k8sFixture.ExistByName())

				Consistently(argocdSecret, "5s", "1s").Should(k8sFixture.ExistByName())
				Eventually(argocdSecret).Should(secretFixture.HaveNonEmptyKeyValue("accounts.alice.tokens"), "Entry 'alice.account.tokens' should be found in argocd-secret")
				Consistently(argocdSecret, "5s", "1s").Should(secretFixture.HaveNonEmptyKeyValue("accounts.alice.tokens"), "Entry 'alice.account.tokens' should be found in argocd-secret")

			}

			verifyConfigurationIsAsExpected("initial creation")

			// ----

			By("delete local user Secret")
			Expect(k8sClient.Delete(ctx, aliceLocalUser)).To(Succeed())

			By("verifying local user Secret is recreated")
			verifyConfigurationIsAsExpected("after local Secret deletion")

			// -----

			By("deleting argocd-cm ConfigMap, to verify it is recreated with expected values")

			Expect(k8sClient.Delete(ctx, argocdCMConfigMap)).To(Succeed())

			verifyConfigurationIsAsExpected("after argocd-cm deletion")

			// -----
			By("removing alice user, and creating new user bob")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.LocalUsers = []argov1beta1api.LocalUserSpec{
					{
						Name:          "bob",
						TokenLifetime: "100h",
					},
				}
			})

			By("verifying alice-local-user Secret is deleted")
			Eventually(aliceLocalUser).Should(k8sFixture.NotExistByName())

			By("verifying bob-local-user Secret is created")

			bobLocalUser := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bob-local-user",
					Namespace: ns.Name,
				},
			}
			Eventually(bobLocalUser).Should(k8sFixture.ExistByName())

			By("verifying alice is removed from argocd-cm ConfigMap")
			Eventually(argocdCMConfigMap).Should(k8sFixture.ExistByName())
			Eventually(argocdCMConfigMap).Should(configmapFixture.NotHaveStringDataKey("accounts.alice"))
			Eventually(argocdCMConfigMap).Should(configmapFixture.NotHaveStringDataKey("accounts.alice.enabled"))
			Consistently(argocdCMConfigMap).Should(configmapFixture.NotHaveStringDataKey("accounts.alice"))
			Consistently(argocdCMConfigMap).Should(configmapFixture.NotHaveStringDataKey("accounts.alice.enabled"))

			By("verifying Argo CD argocd-cm ConfigMap references bob, and bob is enabled")
			Eventually(argocdCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("accounts.bob", "apiKey"))
			Consistently(argocdCMConfigMap, "5s", "1s").Should(configmapFixture.HaveStringDataKeyValue("accounts.bob", "apiKey"))
			Eventually(argocdCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("accounts.bob.enabled", "true"))

			By("verifying argocd-secret Secret contains token for bob")
			Eventually(argocdSecret).Should(k8sFixture.ExistByName())
			Consistently(argocdSecret, "5s", "1s").Should(k8sFixture.ExistByName())
			Eventually(argocdSecret).Should(secretFixture.HaveNonEmptyKeyValue("accounts.bob.tokens"))
			Consistently(argocdSecret, "5s", "1s").Should(secretFixture.HaveNonEmptyKeyValue("accounts.bob.tokens"))

			By("verifying argocd-secret Secret does not contain token for alice")
			Eventually(argocdSecret).Should(secretFixture.NotHaveDataKey("accounts.alice.tokens"))
			Consistently(argocdSecret).Should(secretFixture.NotHaveDataKey("accounts.alice.tokens"))

			// -----

			By("removing localUsers field, which should cause the resources to be deleted")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.LocalUsers = nil
			})

			By("verifying local user Secret is removed")
			Eventually(aliceLocalUser).Should(k8sFixture.NotExistByName())
			Consistently(aliceLocalUser).Should(k8sFixture.NotExistByName())

			By("verifying Argo CD argocd-cm ConfigMap continues to exist")
			Eventually(argocdCMConfigMap).Should(k8sFixture.ExistByName())
			Consistently(argocdCMConfigMap).Should(k8sFixture.ExistByName())

		})

		It("verifies that if left over resources exist after the local user is deleted from ArgoCD CR, the resources are deleted", func() {

			By("creating namespace-scoped Argo CD instance in a new namespace, with no local users")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec:       argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying the Argo CD instance becomes available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("adding account fields to ConfigMap, and verifying they are removed by reconciler")

			argocdCMConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: argoCD.Namespace,
				},
			}

			configmapFixture.Update(argocdCMConfigMap, func(cm *corev1.ConfigMap) {
				if cm.Data == nil {
					cm.Data = map[string]string{}
				}
				cm.Data["accounts.alice"] = "apiKey"
				cm.Data["accounts.alice.enabled"] = "true"
			})

			Eventually(argocdCMConfigMap).Should(configmapFixture.NotHaveStringDataKey("accounts.alice.enabled"))
			Eventually(argocdCMConfigMap).Should(configmapFixture.NotHaveStringDataKey("accounts.alice"))

			Consistently(argocdCMConfigMap).Should(configmapFixture.NotHaveStringDataKey("accounts.alice.enabled"))
			Consistently(argocdCMConfigMap).Should(configmapFixture.NotHaveStringDataKey("accounts.alice"))

			// -----

			By("creating local user Secret (without an entry in ArgoCD CR) and verifying it is deleted")

			// Create a manually crafted local user Secret to verify it gets deleted by the reconciler
			manuallyCreatedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "alice-local-user",
					Namespace: ns.Name,
					Labels: map[string]string{
						"app.kubernetes.io/component":     "local-users",
						"app.kubernetes.io/managed-by":    "example-argocd",
						"app.kubernetes.io/name":          "alice-local-user",
						"app.kubernetes.io/part-of":       "argocd",
						"operator.argoproj.io/tracked-by": "argocd",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion:         "argoproj.io/v1beta1",
							Kind:               "ArgoCD",
							BlockOwnerDeletion: ptr.To(true),
							Controller:         ptr.To(true),
							Name:               argoCD.Name,
							UID:                argoCD.UID,
						},
					},
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"apiToken":      []byte("fake-api-token-" + string(uuid.NewUUID())),
					"autoRenew":     []byte("true"),
					"expAt":         []byte("2761409557"),
					"tokenLifetime": []byte("100h"),
					"user":          []byte("alice"),
				},
			}

			By("creating the manually crafted local user Secret")
			Expect(k8sClient.Create(ctx, manuallyCreatedSecret)).To(Succeed())

			By("verifying the manually created Secret is deleted by the reconciler")
			Eventually(manuallyCreatedSecret).Should(k8sFixture.NotExistByName())
			Consistently(manuallyCreatedSecret).Should(k8sFixture.NotExistByName())
		})

	})
})
