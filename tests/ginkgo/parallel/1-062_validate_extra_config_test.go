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
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-062_validate_extra_config_test", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that .spec.extraConfig overrides values specified in .spec.sso.dex.config and .spec.disableAdmin", func() {

			By("creating simple namespace-scoped Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					ExtraConfig: map[string]string{"admin.enabled": "true"},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			argocdConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "argocd-cm", Namespace: ns.Name}}
			Eventually(argocdConfigMap).Should(k8sFixture.ExistByName())

			By("verifying ConfigMap picks up admin.enabled setting from ArgoCD CR")
			Eventually(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true"))

			By("disabling admin via CR spec, but enabling via extra config")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.DisableAdmin = true
				ac.Spec.ExtraConfig = map[string]string{"admin.enabled": "true"} // override admin user through extraConfig
			})

			By("verifying that extraConfig setting overrides CR field")
			Eventually(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true"))
			Consistently(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true"))

			By("simulating the user manually modifying the ConfigMap without doing so via the ArgoCD CR")
			configmapFixture.Update(argocdConfigMap, func(cm *corev1.ConfigMap) {
				cm.Data["admin.enabled"] = "false"
			})

			By("the user's simulated change should be reverted back to the desired state expressed within the ArgoCD CR")
			Eventually(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true"))
			Consistently(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true"), "operator should reject any manual updates to the configmap.")

			By("updating CR to use a connector, via .spec.sso.dex.config")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.DisableAdmin = true
				ac.Spec.SSO = &argov1beta1api.ArgoCDSSOSpec{
					Provider: argov1beta1api.SSOProviderTypeDex,
					Dex:      &argov1beta1api.ArgoCDDexSpec{},
				}
				ac.Spec.SSO.Dex.Config = `connectors:
  - type: github
    id: github
    name: github-using-first-class
    config:
      clientID: first-class
      clientSecret: $dex.github.clientSecret
      orgs:
        - name: first-class`

			})

			By("verifying CR .spec.sso.dex.config is set in the ConfigMap")
			Eventually(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true"))
			Consistently(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true"))

			Eventually(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("dex.config", `connectors:
  - type: github
    id: github
    name: github-using-first-class
    config:
      clientID: first-class
      clientSecret: $dex.github.clientSecret
      orgs:
        - name: first-class`))

			By("overriding .spec.sso.dex.config via .spec.extraConfig with a different value")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {

				ac.Spec.DisableAdmin = true
				ac.Spec.SSO.Provider = argov1beta1api.SSOProviderTypeDex
				ac.Spec.SSO.Dex.Config = `connectors:
  - type: github
    id: github
    name: github-using-first-class
    config:
      clientID: first-class
      clientSecret: $dex.github.clientSecret
      orgs:
        - name: first-class`

				ac.Spec.ExtraConfig = map[string]string{
					"admin.enabled": "true",
					"dex.config": `connectors:
  - type: github
    id: github
    name: github-using-extra-config
    config:
      clientID: extra-config
      clientSecret: $dex.github.clientSecret
      orgs:
        - name: extra-config`,
				}
			})

			By("verifying value specified in .spec.extraConfig overrides the value specified in spec field")

			Eventually(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true"))
			Consistently(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true"))
			Eventually(argocdConfigMap).Should(configmapFixture.HaveStringDataKeyValue("dex.config", `connectors:
  - type: github
    id: github
    name: github-using-extra-config
    config:
      clientID: extra-config
      clientSecret: $dex.github.clientSecret
      orgs:
        - name: extra-config`))
		})

	})
})
