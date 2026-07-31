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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// newArgoCDForDexOpenShiftOAuthE2E returns the ArgoCD CR.
func newArgoCDForDexOpenShiftOAuthE2E(namespace string) *argov1beta1api.ArgoCD {
	return &argov1beta1api.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: namespace},
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
}

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

		It("verifies that the Dex client secret is sourced from a short-lived TokenRequest token and is correctly set in argocd-secret", func() {

			By("creating simple Argo CD instance with Dex and Openshift OAuth enabled")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := newArgoCDForDexOpenShiftOAuthE2E(ns.Name)
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			dexSAName := "example-argocd-argocd-dex-server"
			serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: dexSAName, Namespace: ns.Name}}
			Eventually(serviceAccount).Should(k8sFixture.ExistByName())

			By("verifying no non-expiring kubernetes.io/service-account-token Secret exists for the Dex SA (absence must hold for a short window, not only at one instant)")
			Consistently(func() bool {
				secretList := &corev1.SecretList{}
				if err := k8sClient.List(ctx, secretList, client.InNamespace(ns.Name)); err != nil {
					return false
				}
				for _, s := range secretList.Items {
					if s.Type == corev1.SecretTypeServiceAccountToken &&
						strings.HasPrefix(s.Name, dexSAName+"-token-") &&
						s.Annotations[corev1.ServiceAccountNameKey] == dexSAName {
						return false
					}
				}
				return true
			}, "20s", "4s").Should(BeTrue(), "no legacy kubernetes.io/service-account-token Secret for the Dex SA should appear while reconciles continue")

			By("verifying argocd-cm ConfigMap is not leaking oidc dex client secret")
			argocdCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "argocd-cm", Namespace: ns.Name}}
			Eventually(argocdCM).Should(k8sFixture.ExistByName())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(argocdCM), argocdCM); err != nil {
					return false
				}
				return strings.Contains(argocdCM.Data["dex.config"], "clientSecret: $oidc.dex.clientSecret")
			}, "2m", "5s").Should(BeTrue(), "'$oidc.dex.clientSecret' should be set. Any other value implies that the client secret is exposed via ConfigMap")

			By("verifying the Dex SA has no non-expiring kubernetes.io/service-account-token Secrets in its .secrets list")
			// The operator must clean up legacy SA token Secrets and must not auto-generate new ones.
			dexSANoLegacyTokenRefs := func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount); err != nil {
					return false
				}
				for _, ref := range serviceAccount.Secrets {
					if strings.Contains(ref.Name, "dex-server-token") {
						GinkgoWriter.Println("Dex SA still has legacy token Secret reference:", ref.Name)
						return false
					}
				}
				return true
			}
			Eventually(dexSANoLegacyTokenRefs, "2m", "5s").Should(BeTrue(), "Dex SA .secrets must not reference any legacy non-expiring token Secrets")
			By("verifying that absence of legacy token Secret references in the Dex SA .secrets list persists")
			Consistently(dexSANoLegacyTokenRefs, "20s", "4s").Should(BeTrue(), "Dex SA .secrets must keep no legacy non-expiring token Secret references")

			By("verifying the dedicated short-lived Dex token Secret was created by the operator")
			tokenSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-argocd-dex-server-token", Namespace: ns.Name}}
			Eventually(tokenSecret, "2m", "5s").Should(k8sFixture.ExistByName())
			Eventually(tokenSecret).Should(secretFixture.HaveNonEmptyKeyValue("token"))
			Eventually(tokenSecret).Should(secretFixture.HaveNonEmptyKeyValue("expiry"))

			By("verifying the token expiry is a valid RFC3339 timestamp in the future")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(tokenSecret), tokenSecret); err != nil {
					return false
				}
				expiry, err := time.Parse(time.RFC3339, string(tokenSecret.Data["expiry"]))
				if err != nil {
					GinkgoWriter.Println("expiry is not valid RFC3339:", string(tokenSecret.Data["expiry"]), err)
					return false
				}
				GinkgoWriter.Println("token expiry:", expiry.UTC())
				return time.Until(expiry) > 0
			}, "2m", "5s").Should(BeTrue(), "Dex token 'expiry' must be a valid RFC3339 timestamp in the future")

			By("validating that the Dex client secret in argocd-secret matches the token in the dedicated token Secret")
			argocdSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "argocd-secret", Namespace: ns.Name}}
			Eventually(argocdSecret).Should(k8sFixture.ExistByName())
			Eventually(argocdSecret).Should(secretFixture.HaveNonEmptyKeyValue("oidc.dex.clientSecret"))

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(tokenSecret), tokenSecret); err != nil {
					return false
				}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(argocdSecret), argocdSecret); err != nil {
					return false
				}
				return string(tokenSecret.Data["token"]) == string(argocdSecret.Data["oidc.dex.clientSecret"])
			}, "2m", "5s").Should(BeTrue(), "Dex client secret in argocd-secret must match the token in the dedicated Dex token Secret")
		})

		It("verifies the operator deletes legacy non-expiring Dex kubernetes.io/service-account-token Secrets and drops them from the Dex SA", func() {

			By("creating simple Argo CD instance with Dex and Openshift OAuth enabled")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := newArgoCDForDexOpenShiftOAuthE2E(ns.Name)
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			dexSAName := "example-argocd-argocd-dex-server"
			dexSA := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: dexSAName, Namespace: ns.Name}}
			Eventually(dexSA).Should(k8sFixture.ExistByName())

			legacyName := dexSAName + "-token-e2elegacy"
			By("creating a legacy non-expiring kubernetes.io/service-account-token Secret for the Dex SA (operator-tracked label required for cleanup list)")
			legacySecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      legacyName,
					Namespace: ns.Name,
					Labels: map[string]string{
						common.ArgoCDTrackedByOperatorLabel: common.ArgoCDAppName,
					},
					Annotations: map[string]string{
						corev1.ServiceAccountNameKey: dexSAName,
					},
				},
				Type: corev1.SecretTypeServiceAccountToken,
			}
			Expect(k8sClient.Create(ctx, legacySecret)).To(Succeed())

			By("adding the legacy Secret to the Dex SA .secrets list to mimic stale controller state")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dexSA), dexSA); err != nil {
					GinkgoWriter.Printf("get Dex SA: %v\n", err)
					return false
				}
				for _, ref := range dexSA.Secrets {
					if ref.Name == legacyName {
						return true
					}
				}
				dexSA.Secrets = append(dexSA.Secrets, corev1.ObjectReference{Name: legacyName})
				if err := k8sClient.Update(ctx, dexSA); err != nil {
					GinkgoWriter.Printf("update Dex SA with legacy secret ref: %v\n", err)
					return false
				}
				return true
			}, "2m", "3s").Should(BeTrue(), "Dex SA should list the synthetic legacy token Secret reference")

			By("triggering reconciliation so the operator runs legacy Dex token Secret cleanup (creating the Secret does not enqueue the ArgoCD reconcile)")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				if ac.Annotations == nil {
					ac.Annotations = make(map[string]string)
				}
				ac.Annotations["test.argocd.argoproj.io/trigger-legacy-dex-token-reconcile"] = time.Now().Format(time.RFC3339Nano)
			})

			By("waiting for the operator to delete the legacy Secret")
			legacySecretGone := func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(legacySecret), legacySecret)
				return apierrors.IsNotFound(err)
			}
			Eventually(legacySecretGone, "2m", "5s").Should(BeTrue(), "legacy kubernetes.io/service-account-token Secret must be deleted")
			By("verifying the legacy Secret stays deleted")
			Consistently(legacySecretGone, "20s", "4s").Should(BeTrue(), "legacy kubernetes.io/service-account-token Secret must not reappear")

			By("waiting for the Dex SA to no longer reference legacy dex-server-token Secrets")
			dexSANoLegacyTokenRefs := func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dexSA), dexSA); err != nil {
					return false
				}
				for _, ref := range dexSA.Secrets {
					if strings.Contains(ref.Name, "dex-server-token") {
						GinkgoWriter.Println("Dex SA still has legacy token Secret reference:", ref.Name)
						return false
					}
				}
				return true
			}
			Eventually(dexSANoLegacyTokenRefs, "2m", "5s").Should(BeTrue(), "Dex SA .secrets must not reference legacy non-expiring token Secrets")
			By("verifying that absence of legacy token Secret references in the Dex SA .secrets list persists")
			Consistently(dexSANoLegacyTokenRefs, "20s", "4s").Should(BeTrue(), "Dex SA .secrets must keep no legacy non-expiring token Secret references")

			By("verifying the Argo CD instance stays healthy after legacy cleanup")
			Eventually(argoCD, "2m", "5s").Should(argocdFixture.BeAvailable())
		})

	})
})
