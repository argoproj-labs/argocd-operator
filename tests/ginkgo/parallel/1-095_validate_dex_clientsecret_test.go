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
	"strings"

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

	Context("1-095_validate_dex_clientsecret", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that Dex serviceaccount token secret is not leaked, and is correctly set in Argo CD argocd-secret Secret", func() {

			By("creating simple Argo CD instance with Dex and Openshift OAuth enabled")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
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
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-argocd-dex-server", Namespace: ns.Name}}
			Eventually(serviceAccount).Should(k8sFixture.ExistByName())

			argocdCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-cm", Namespace: ns.Name},
			}
			Eventually(argocdCM).Should(k8sFixture.ExistByName())

			By("verifying argocd-cm ConfigMap is not leaking oidc dex client secret")
			dexConfig := argocdCM.Data["dex.config"]

			Expect(dexConfig).To(ContainSubstring("clientSecret: $oidc.dex.clientSecret"), "'$oidc.dex.clientSecret' should be set. Any other value implies that the client secret is exposed via ConfigMap")

			By("validating that the Dex Client Secret was copied from dex serviceaccount token secret in to argocd-secret, by the operator")

			// To verify the behavior we should first get the token secret name of the dex service account.

			var secretName string
			for _, secretData := range serviceAccount.Secrets {

				if strings.Contains(secretData.Name, "token") {
					secretName = secretData.Name
				}
			}
			Expect(secretName).ToNot(BeEmpty())

			// Extract the clientSecret
			secretReferencedFromServiceAccount := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns.Name}}
			Eventually(secretReferencedFromServiceAccount).Should(k8sFixture.ExistByName())
			tokenFromSASecret := secretReferencedFromServiceAccount.Data["token"]
			Expect(tokenFromSASecret).ToNot(BeEmpty())

			// actualClientSecret is the value of the secret in argocd-secret where argocd-operator should copy the secret from
			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret).Should(k8sFixture.ExistByName())

			actualClientSecret := argocdSecret.Data["oidc.dex.clientSecret"]

			Expect(string(actualClientSecret)).To(Equal(string(tokenFromSASecret)), "Dex Client Secret for OIDC is not valid")

		})

	})
})
