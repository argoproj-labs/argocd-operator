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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-049_validate_logs_rbac_enforcement", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that setting .spec.rbac.policy on ArgoCD CR sets the corresponding ConfigMaps, verifies that updating works as well, and verifies that deprecated settings are removed if detected ", func() {

			By("creating a basic Argo CD instance with custom RBAC policy")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			policyStr := `
# Custom role without logs permissions
p, role:no-logs, applications, get, */*, allow
# Custom role with logs permissions
p, role:with-logs, applications, get, */*, allow
p, role:with-logs, logs, get, */*, allow`

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					RBAC: argov1beta1api.ArgoCDRBACSpec{
						Policy: ptr.To(policyStr),
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying the 'argocd-cm' ConfigMap exists and has expected content")
			argocdcmConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: ns.Name,
				},
			}
			Eventually(argocdcmConfigMap, "4m", "5s").Should(k8sFixture.ExistByName())
			Eventually(argocdcmConfigMap, "4m", "5s").Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true")) // required to satisfy object type

			By("verifying that server.rbac.log.enforce.enable is not present in 'argocd-cm' ConfigMap")
			Eventually(argocdcmConfigMap, "4m", "5s").Should(configmapFixture.NotHaveStringDataKey("server.rbac.log.enforce.enable"))

			By("verifying policy value is set in 'argocd-rbac-cm' ConfigMap")
			argocdRBACCMConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-rbac-cm",
					Namespace: ns.Name,
				},
			}
			Eventually(argocdRBACCMConfigMap).Should(k8sFixture.ExistByName())
			Eventually(argocdRBACCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("policy.csv", policyStr))

			By("setting a new .spec.rbac.policy value, and verifying it is set in argocd-rbac-cm config map")

			policyStr = `
# Custom role without logs permissions
p, role:no-logs, applications, get, */*, allow
# Custom role with logs permissions
p, role:with-logs, applications, get, */*, allow
p, role:with-logs, logs, get, */*, allow
# Global log viewer role
p, role:global-log-viewer, logs, get, */*, allow`

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.RBAC.Policy = ptr.To(policyStr)
			})
			Eventually(argocdRBACCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("policy.csv", policyStr))

			By("setting legacy configuration on argocd-cm ConfigMap, with server.rbac.log.enforce.enable set to true")
			configmapFixture.Update(argocdcmConfigMap, func(cm *corev1.ConfigMap) {
				cm.Data = map[string]string{
					"server.rbac.log.enforce.enable": "true",
					"admin.enabled":                  "true",
				}
			})

			By("setting legacy configuration with no default policy, on argocd-rbac-cm ConfigMap")
			configmapFixture.Update(argocdRBACCMConfigMap, func(cm *corev1.ConfigMap) {
				cm.Data = map[string]string{
					"policy.csv": `
# Custom role with only applications access
p, role:app-only, applications, get, */*, allow`,
				}
			})

			By("updating ArgoCD to have no default policy specified")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.RBAC.Policy = ptr.To(`
# Custom role with only applications access
p, role:app-only, applications, get, */*, allow`)
			})

			By("verifying server.rbac.log.enforce.enable is removed from argocd-cm, since it's no longer needed in Argo CD 3.0")
			Eventually(argocdcmConfigMap, "4m", "5s").Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "true"))
			Eventually(argocdcmConfigMap, "4m", "5s").Should(configmapFixture.NotHaveStringDataKey("server.rbac.log.enforce.enable"))

			By("verifying RBAC policy is preserved in argocd-rbac-cm")
			Eventually(argocdRBACCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("policy.csv", `
# Custom role with only applications access
p, role:app-only, applications, get, */*, allow`))

		})
	})
})
