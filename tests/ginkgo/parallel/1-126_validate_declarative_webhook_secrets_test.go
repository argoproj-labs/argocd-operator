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
	"k8s.io/apimachinery/pkg/types"
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

	Context("1-126_validate_declarative_webhook_secrets", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			var err error
			k8sClient, _, err = fixtureUtils.GetE2ETestKubeClientWithError()
			Expect(err).NotTo(HaveOccurred())
			ctx = context.Background()
		})

		It("verifies spec.webhookSecrets.github.webhookSecretRef is synced into argocd-secret as webhook.github.secret", func() {
			By("creating Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-github", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{Enabled: true},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for argocd-secret to exist")
			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret, "2m", "3s").Should(k8sFixture.ExistByName())

			By("creating user Secret with GitHub webhook token")
			expectedToken := "e2e-github-webhook-secret-token"
			userSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "github-webhook-credentials", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"token": expectedToken},
			}
			Expect(k8sClient.Create(ctx, userSecret)).To(Succeed())

			By("setting spec.webhookSecrets.github.webhookSecretRef on the ArgoCD CR")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.WebhookSecrets = &argov1beta1api.ArgoCDWebhookSecretsSpec{
				GitHub: &argov1beta1api.ArgoCDWebhookSecretsGitHub{
					WebhookSecretRef: &argov1beta1api.WebhookSecretKeySelector{
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

		It("verifies spec.webhookSecrets.gitlab.webhookSecretRef is synced into argocd-secret as webhook.gitlab.secret", func() {
			By("creating Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{Enabled: true},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for argocd-secret to exist")
			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret, "2m", "3s").Should(k8sFixture.ExistByName())

			By("creating user Secret with GitLab webhook credentials")
			expected := "e2e-gitlab-webhook-secret-value"
			userSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "gitlab-webhook-credentials", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"secret": expected},
			}
			Expect(k8sClient.Create(ctx, userSecret)).To(Succeed())

			By("setting spec.webhookSecrets.gitlab.webhookSecretRef on the ArgoCD CR")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.WebhookSecrets = &argov1beta1api.ArgoCDWebhookSecretsSpec{
				GitLab: &argov1beta1api.ArgoCDWebhookSecretsGitLab{
					WebhookSecretRef: &argov1beta1api.WebhookSecretKeySelector{
						Name: "gitlab-webhook-credentials",
						Key:  "secret",
					},
				},
			}
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())

			By("waiting for argocd-secret to contain webhook.gitlab.secret matching the referenced Secret")
			Eventually(argocdSecret, "2m", "3s").Should(
				secretFixture.HaveDataKeyValue(common.ArgoCDKeyGitLabWebhookSecret, []byte(expected)),
			)
		})

		It("verifies spec.webhookSecrets.azureDevOps username and password secretRefs are synced into argocd-secret", func() {
			By("creating Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-ado", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{Enabled: true},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for argocd-secret to exist")
			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret, "2m", "3s").Should(k8sFixture.ExistByName())

			By("creating user Secret with Azure DevOps webhook credentials")
			expectedUser := "e2e-ado-username"
			expectedPass := "e2e-ado-password-or-pat"
			userSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "ado-webhook-credentials", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"username": expectedUser,
					"password": expectedPass,
				},
			}
			Expect(k8sClient.Create(ctx, userSecret)).To(Succeed())

			By("setting spec.webhookSecrets.azureDevOps on the ArgoCD CR")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.WebhookSecrets = &argov1beta1api.ArgoCDWebhookSecretsSpec{
				AzureDevOps: &argov1beta1api.ArgoCDWebhookSecretsAzureDevOps{
					UsernameSecretRef: &argov1beta1api.WebhookSecretKeySelector{
						Name: "ado-webhook-credentials",
						Key:  "username",
					},
					PasswordSecretRef: &argov1beta1api.WebhookSecretKeySelector{
						Name: "ado-webhook-credentials",
						Key:  "password",
					},
				},
			}
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())

			By("waiting for both Azure DevOps keys in argocd-secret")
			Eventually(argocdSecret, "2m", "3s").Should(
				And(
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyAzureDevOpsWebhookUsername, []byte(expectedUser)),
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyAzureDevOpsWebhookPassword, []byte(expectedPass)),
				),
			)
		})

		It("verifies GitHub and GitLab webhook secret references can be configured together on one ArgoCD instance", func() {
			By("creating Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-multi", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{Enabled: true},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for argocd-secret to exist")
			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret, "2m", "3s").Should(k8sFixture.ExistByName())

			By("creating user Secrets for GitHub and GitLab webhook credentials")
			ghToken := "e2e-combined-github-token"
			glSecret := "e2e-combined-gitlab-secret"
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "gh-creds", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"token": ghToken},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "gl-creds", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"secret": glSecret},
			})).To(Succeed())

			By("setting spec.webhookSecrets.github and spec.webhookSecrets.gitlab on the ArgoCD CR")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.WebhookSecrets = &argov1beta1api.ArgoCDWebhookSecretsSpec{
				GitHub: &argov1beta1api.ArgoCDWebhookSecretsGitHub{
					WebhookSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "gh-creds", Key: "token"},
				},
				GitLab: &argov1beta1api.ArgoCDWebhookSecretsGitLab{
					WebhookSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "gl-creds", Key: "secret"},
				},
			}
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())

			By("waiting for argocd-secret to contain GitHub and GitLab webhook keys")
			Eventually(argocdSecret, "2m", "3s").Should(
				And(
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyGitHubWebhookSecret, []byte(ghToken)),
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyGitLabWebhookSecret, []byte(glSecret)),
				),
			)
		})

		It("verifies Bitbucket Cloud, Bitbucket Server, and Gogs webhook secret references are synced into argocd-secret", func() {
			By("creating Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-bb-gogs", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{Enabled: true},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for argocd-secret to exist")
			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret, "2m", "3s").Should(k8sFixture.ExistByName())

			By("creating user Secrets for Bitbucket Cloud, Bitbucket Server, and Gogs")
			bbUUID := "e2e-bb-cloud-uuid"
			bbSrv := "e2e-bb-server-secret"
			gogsVal := "e2e-gogs-secret"
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "bb-cloud-creds", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"uuid": bbUUID},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "bb-server-creds", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"secret": bbSrv},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "gogs-creds", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"secret": gogsVal},
			})).To(Succeed())

			By("setting spec.webhookSecrets for Bitbucket Cloud, Bitbucket Server, and Gogs on the ArgoCD CR")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.WebhookSecrets = &argov1beta1api.ArgoCDWebhookSecretsSpec{
				Bitbucket: &argov1beta1api.ArgoCDWebhookSecretsBitbucket{
					WebhookUUIDSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "bb-cloud-creds", Key: "uuid"},
				},
				BitbucketServer: &argov1beta1api.ArgoCDWebhookSecretsBitbucketServer{
					WebhookSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "bb-server-creds", Key: "secret"},
				},
				Gogs: &argov1beta1api.ArgoCDWebhookSecretsGogs{
					WebhookSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "gogs-creds", Key: "secret"},
				},
			}
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())

			By("waiting for argocd-secret to contain Bitbucket Cloud, Bitbucket Server, and Gogs webhook keys")
			Eventually(argocdSecret, "2m", "3s").Should(
				And(
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyBitbucketCloudWebhookSecret, []byte(bbUUID)),
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyBitbucketServerWebhookSecret, []byte(bbSrv)),
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyGogsWebhookSecret, []byte(gogsVal)),
				),
			)
		})

		It("removes webhook.github.secret from argocd-secret when spec.webhookSecrets is cleared", func() {
			By("creating Argo CD instance and syncing GitHub webhook secret")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-clear-github", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{Enabled: true},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for argocd-secret to exist")
			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret, "2m", "3s").Should(k8sFixture.ExistByName())

			token := "e2e-clear-github-token"
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "gh-clear-creds", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"token": token},
			})).To(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.WebhookSecrets = &argov1beta1api.ArgoCDWebhookSecretsSpec{
				GitHub: &argov1beta1api.ArgoCDWebhookSecretsGitHub{
					WebhookSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "gh-clear-creds", Key: "token"},
				},
			}
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())

			By("waiting for argocd-secret to contain webhook.github.secret")
			Eventually(argocdSecret, "2m", "3s").Should(
				secretFixture.HaveDataKeyValue(common.ArgoCDKeyGitHubWebhookSecret, []byte(token)),
			)

			By("clearing spec.webhookSecrets on the ArgoCD CR")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.WebhookSecrets = nil
			})
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for webhook.github.secret to be removed from argocd-secret")
			Eventually(argocdSecret, "2m", "3s").Should(secretFixture.NotHaveDataKey(common.ArgoCDKeyGitHubWebhookSecret))
		})

		It("removes webhook.gitlab.secret when GitLab is dropped from spec while GitHub remains", func() {
			By("creating Argo CD instance and syncing GitHub and GitLab webhook secrets")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-partial-gl", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{Enabled: true},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for argocd-secret to exist")
			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret, "2m", "3s").Should(k8sFixture.ExistByName())

			ghTok := "e2e-partial-gh-token"
			glSec := "e2e-partial-gl-secret"
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "partial-gh", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"token": ghTok},
			})).To(Succeed())
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "partial-gl", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"secret": glSec},
			})).To(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.WebhookSecrets = &argov1beta1api.ArgoCDWebhookSecretsSpec{
				GitHub: &argov1beta1api.ArgoCDWebhookSecretsGitHub{
					WebhookSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "partial-gh", Key: "token"},
				},
				GitLab: &argov1beta1api.ArgoCDWebhookSecretsGitLab{
					WebhookSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "partial-gl", Key: "secret"},
				},
			}
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())

			By("waiting for argocd-secret to contain GitHub and GitLab webhook keys")
			Eventually(argocdSecret, "2m", "3s").Should(
				And(
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyGitHubWebhookSecret, []byte(ghTok)),
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyGitLabWebhookSecret, []byte(glSec)),
				),
			)

			By("removing only GitLab from spec.webhookSecrets via merge patch")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			gitLabRemovalPatch := []byte(`{"spec":{"webhookSecrets":{"gitlab":null}}}`)
			Expect(k8sClient.Patch(ctx, argoCD, client.RawPatch(types.MergePatchType, gitLabRemovalPatch))).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for GitLab key to be removed and GitHub key unchanged")
			Eventually(argocdSecret, "2m", "3s").Should(
				And(
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyGitHubWebhookSecret, []byte(ghTok)),
					secretFixture.NotHaveDataKey(common.ArgoCDKeyGitLabWebhookSecret),
				),
			)
		})

		It("removes Azure DevOps webhook keys from argocd-secret when spec.webhookSecrets is cleared", func() {
			By("creating Argo CD instance and syncing Azure DevOps webhook credentials")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-clear-ado", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{Enabled: true},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for argocd-secret to exist")
			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret, "2m", "3s").Should(k8sFixture.ExistByName())

			u := "e2e-clear-ado-user"
			p := "e2e-clear-ado-pass"
			Expect(k8sClient.Create(ctx, &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "ado-clear-creds", Namespace: ns.Name},
				Type:       corev1.SecretTypeOpaque,
				StringData: map[string]string{"username": u, "password": p},
			})).To(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.WebhookSecrets = &argov1beta1api.ArgoCDWebhookSecretsSpec{
				AzureDevOps: &argov1beta1api.ArgoCDWebhookSecretsAzureDevOps{
					UsernameSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "ado-clear-creds", Key: "username"},
					PasswordSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "ado-clear-creds", Key: "password"},
				},
			}
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())

			By("waiting for argocd-secret to contain Azure DevOps webhook username and password")
			Eventually(argocdSecret, "2m", "3s").Should(
				And(
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyAzureDevOpsWebhookUsername, []byte(u)),
					secretFixture.HaveDataKeyValue(common.ArgoCDKeyAzureDevOpsWebhookPassword, []byte(p)),
				),
			)

			By("clearing spec.webhookSecrets on the ArgoCD CR")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.WebhookSecrets = nil
			})
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for Azure DevOps webhook keys to be removed from argocd-secret")
			Eventually(argocdSecret, "2m", "3s").Should(
				And(
					secretFixture.NotHaveDataKey(common.ArgoCDKeyAzureDevOpsWebhookUsername),
					secretFixture.NotHaveDataKey(common.ArgoCDKeyAzureDevOpsWebhookPassword),
				),
			)
		})

		It("rejects spec.webhookSecrets.azureDevOps when only usernameSecretRef is set (CRD validation: both refs required together)", func() {
			By("creating namespace and an ArgoCD CR with azureDevOps missing passwordSecretRef")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			invalid := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-invalid-ado-pair", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{Enabled: true},
					},
					WebhookSecrets: &argov1beta1api.ArgoCDWebhookSecretsSpec{
						AzureDevOps: &argov1beta1api.ArgoCDWebhookSecretsAzureDevOps{
							UsernameSecretRef: &argov1beta1api.WebhookSecretKeySelector{Name: "only-user-ref", Key: "username"},
							// PasswordSecretRef intentionally omitted — violates CRD XValidation (pair rule).
						},
					},
				},
			}

			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred(), "apiserver should reject azureDevOps with only usernameSecretRef")
			msg := err.Error()
			Expect(msg).To(And(
				ContainSubstring("usernameSecretRef"),
				ContainSubstring("passwordSecretRef"),
				ContainSubstring("together"),
			))
		})
	})
})
