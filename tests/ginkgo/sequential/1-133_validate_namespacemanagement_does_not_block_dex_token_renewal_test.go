/*
Copyright 2026.

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

package sequential

import (
	"context"
	"os"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	secretFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/secret"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-133_validate_namespacemanagement_does_not_block_dex_token_renewal", func() {

		var (
			k8sClient            client.Client
			ctx                  context.Context
			cleanupArgoNamespace func()
			cleanupManagedNS     func()
			argoNamespace        *corev1.Namespace
			managedNamespace     *corev1.Namespace
			restoreOperatorEnv   func()
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			fixture.EnsureRunningOnOpenShift()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
			restoreOperatorEnv = func() {}
		})

		AfterEach(func() {
			fixture.OutputDebugOnFail(argoNamespace, managedNamespace)

			if restoreOperatorEnv != nil {
				restoreOperatorEnv()
			}
			if cleanupManagedNS != nil {
				cleanupManagedNS()
			}
			if cleanupArgoNamespace != nil {
				cleanupArgoNamespace()
			}
		})

		It("keeps ArgoCD Available and renews Dex tokens when a disallowed NamespaceManagement CR exists", func() {

			By("ensuring NamespaceManagement feature is enabled on the operator")
			if os.Getenv("LOCAL_RUN") == "true" {
				By("LOCAL_RUN: expecting ALLOW_NAMESPACE_MANAGEMENT_IN_NAMESPACE_SCOPED_INSTANCES=true on the local operator process")
			} else {
				operatorDeployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "argocd-operator-controller-manager",
						Namespace: "argocd-operator-system",
					},
				}
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(operatorDeployment), operatorDeployment)
				if err != nil {
					Skip("Operator deployment not found - test requires operator running in cluster: " + err.Error())
				}

				originalEnvValue, _ := deploymentFixture.GetEnv(operatorDeployment, "manager", common.EnableManagedNamespace)
				restoreOperatorEnv = func() {
					By("restoring original operator NamespaceManagement env var")
					if originalEnvValue != nil {
						deploymentFixture.SetEnv(operatorDeployment, "manager", common.EnableManagedNamespace, *originalEnvValue)
					} else {
						deploymentFixture.RemoveEnv(operatorDeployment, "manager", common.EnableManagedNamespace)
					}
					time.Sleep(30 * time.Second)
					Eventually(operatorDeployment, "3m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))
				}

				deploymentFixture.SetEnv(operatorDeployment, "manager", common.EnableManagedNamespace, "true")
				time.Sleep(30 * time.Second)
				Eventually(operatorDeployment, "3m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))
			}

			argoNamespace, cleanupArgoNamespace = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			managedNamespace, cleanupManagedNS = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			By("creating ArgoCD with Dex OpenShift OAuth, short-lived Dex tokens, and deny-all NamespaceManagement")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: argoNamespace.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeDex,
						Dex: &argov1beta1api.ArgoCDDexSpec{
							OpenShiftOAuth:       true,
							EnableSATokenRenewal: ptr.To(true),
						},
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
					NamespaceManagement: []argov1beta1api.ManagedNamespaces{
						{
							Name:           "*",
							AllowManagedBy: false,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD to become Available before introducing a disallowed NamespaceManagement CR")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			tokenSecretName := "example-argocd-argocd-dex-server-token"
			tokenSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: tokenSecretName, Namespace: argoNamespace.Name}}
			Eventually(tokenSecret, "2m", "5s").Should(k8sFixture.ExistByName())
			Eventually(tokenSecret).Should(secretFixture.HaveNonEmptyKeyValue("token"))
			Eventually(tokenSecret).Should(secretFixture.HaveNonEmptyKeyValue("expiry"))

			By("creating a NamespaceManagement CR that targets this ArgoCD but is not allowed")
			nm := &argov1beta1api.NamespaceManagement{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-protect",
					Namespace: managedNamespace.Name,
				},
				Spec: argov1beta1api.NamespaceManagementSpec{
					ManagedBy: argoNamespace.Name,
				},
			}
			Expect(k8sClient.Create(ctx, nm)).To(Succeed())

			By("verifying the NamespaceManagement CR reports that the namespace is not permitted")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(nm), nm); err != nil {
					return false
				}
				for _, c := range nm.Status.Conditions {
					if c.Type == "Reconciled" && c.Status == metav1.ConditionFalse &&
						strings.Contains(c.Message, "not permitted for management") {
						GinkgoWriter.Println("NamespaceManagement status:", c.Message)
						return true
					}
				}
				return false
			}, "2m", "5s").Should(BeTrue(), "NamespaceManagement CR should report not permitted (ensure ALLOW_NAMESPACE_MANAGEMENT_IN_NAMESPACE_SCOPED_INSTANCES=true)")

			By("verifying ArgoCD stays Available and does not fail reconcile because of the disallowed NamespaceManagement CR")
			Consistently(argoCD, "45s", "5s").Should(argocdFixture.HavePhase("Available"))
			Consistently(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD); err != nil {
					return false
				}
				for _, c := range argoCD.Status.Conditions {
					if c.Type == "Reconciled" && c.Status == metav1.ConditionFalse &&
						(strings.Contains(c.Message, "namespace management errors") ||
							strings.Contains(c.Message, "not permitted for management")) {
						GinkgoWriter.Println("unexpected ArgoCD reconcile error:", c.Message)
						return false
					}
				}
				return true
			}, "45s", "5s").Should(BeTrue(), "disallowed NamespaceManagement must not fail the ArgoCD reconcile")

			By("forcing Dex token secret into an expired state to require renewal without waiting for the real TTL")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(tokenSecret), tokenSecret)).To(Succeed())
			oldToken := string(tokenSecret.Data["token"])
			expiredAt := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
			tokenSecret.Data["expiry"] = []byte(expiredAt)
			Expect(k8sClient.Update(ctx, tokenSecret)).To(Succeed())

			By("triggering ArgoCD reconcile while the disallowed NamespaceManagement CR still exists")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				if ac.Annotations == nil {
					ac.Annotations = map[string]string{}
				}
				ac.Annotations["test.argocd.argoproj.io/trigger-nm-dex-renewal"] = time.Now().Format(time.RFC3339Nano)
			})

			By("verifying Dex token is renewed even though the NamespaceManagement CR remains disallowed")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(tokenSecret), tokenSecret); err != nil {
					return false
				}
				expiryRaw := string(tokenSecret.Data["expiry"])
				expiry, err := time.Parse(time.RFC3339, expiryRaw)
				if err != nil {
					GinkgoWriter.Println("unparseable expiry:", expiryRaw, err)
					return false
				}
				newToken := string(tokenSecret.Data["token"])
				GinkgoWriter.Println("dex token expiry:", expiry.UTC(), "tokenChanged:", newToken != oldToken)
				return time.Until(expiry) > 0 && newToken != oldToken
			}, "3m", "5s").Should(BeTrue(), "Dex token must be renewed while a disallowed NamespaceManagement CR exists")

			Eventually(argoCD, "2m", "5s").Should(argocdFixture.BeAvailable())
		})
	})
})
