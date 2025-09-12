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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-050_validate_dex_sso_rbac_migration", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that changes to .spec.sso.rbac.policy are set on the argocd-rbac-cm ConfigMap", func() {

			By("creating a namespace-scoped ArgoCD instance with rbac policy, and with mock dex connector configuration")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeDex,
						Dex: &argov1beta1api.ArgoCDDexSpec{
							Config: `
connectors:
- type: mock
  id: mock
  name: Mock
  config:
    users:
    - email: test@example.com
      name: Test User
      groups: ["test-group"]
- type: mock
  id: mock2
  name: Mock2
  config:
    users:
    - email: admin@example.com
      name: Admin User
      groups: ["admin-group"]`,
						},
					},
					RBAC: argov1beta1api.ArgoCDRBACSpec{
						DefaultPolicy:     ptr.To("role:readonly"),
						PolicyMatcherMode: ptr.To("glob"),
						Policy: ptr.To(`
# Legacy policies using encoded sub claims (simulating Argo CD 2.x)
g, ChdleGFtcGxlQGFyZ29wcm9qLmlvEgJkZXhfY29ubl9pZA, role:test-role
p, ChdleGFtcGxlQGFyZ29wcm9qLmlvEgJkZXhfY29ubl9pZA, applications, get, */*, allow
p, ChdleGFtcGxlQGFyZ29wcm9qLmlvEgJkZXhfY29ubl9pZA, logs, get, */*, allow

# Admin user with encoded sub claim
g, QWRtaW5AZXhhbXBsZS5jb20gZGV4X2Nvbm5faWQ, role:admin
p, QWRtaW5AZXhhbXBsZS5jb20gZGV4X2Nvbm5faWQ, *, *, */*, allow

# Group-based policies (these should work in both versions)
g, test-group, role:test-role
g, admin-group, role:admin`),
					},
				},
			}

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying Argo CD becomes available and SSO is running")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD).Should(argocdFixture.HaveSSOStatus("Running"))

			By("verifying dex Deployment is running")
			dexDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-dex-server",
					Namespace: ns.Name,
				},
			}
			Eventually(dexDepl).Should(k8sFixture.ExistByName())
			Eventually(dexDepl, "2m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))

			By("verifying value in argocd-rbac-cm matches value from ArgoCD CR")
			argocd_rbac_cmConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-rbac-cm",
					Namespace: ns.Name,
				},
			}
			Eventually(argocd_rbac_cmConfigMap).Should(k8sFixture.ExistByName())
			Eventually(argocd_rbac_cmConfigMap).Should(configmap.HaveStringDataKeyValue("policy.csv", `
# Legacy policies using encoded sub claims (simulating Argo CD 2.x)
g, ChdleGFtcGxlQGFyZ29wcm9qLmlvEgJkZXhfY29ubl9pZA, role:test-role
p, ChdleGFtcGxlQGFyZ29wcm9qLmlvEgJkZXhfY29ubl9pZA, applications, get, */*, allow
p, ChdleGFtcGxlQGFyZ29wcm9qLmlvEgJkZXhfY29ubl9pZA, logs, get, */*, allow

# Admin user with encoded sub claim
g, QWRtaW5AZXhhbXBsZS5jb20gZGV4X2Nvbm5faWQ, role:admin
p, QWRtaW5AZXhhbXBsZS5jb20gZGV4X2Nvbm5faWQ, *, *, */*, allow

# Group-based policies (these should work in both versions)
g, test-group, role:test-role
g, admin-group, role:admin`))

			By("modifying the ArgoCD CR .spec.rbac.policy field to a new value")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.RBAC.Policy = ptr.To(`
# Migrated policies using federated_claims.user_id (Argo CD 3.0+)
g, test@example.com, role:test-role
p, test@example.com, applications, get, */*, allow
p, test@example.com, logs, get, */*, allow

# Admin user with federated_claims.user_id
g, admin@example.com, role:admin
p, admin@example.com, *, *, */*, allow

# Group-based policies (these should work in both versions)
g, test-group, role:test-role
g, admin-group, role:admin`)
			})

			By("verifying Argo CD is available and sso is running")
			Eventually(argoCD).Should(argocdFixture.BeAvailable())
			Eventually(argoCD).Should(argocdFixture.HaveSSOStatus("Running"))

			By("verifying argocd-rbac-cm has expected value from ArgoCD CR")
			Eventually(argocd_rbac_cmConfigMap).Should(configmap.HaveStringDataKeyValue("policy.csv", `
# Migrated policies using federated_claims.user_id (Argo CD 3.0+)
g, test@example.com, role:test-role
p, test@example.com, applications, get, */*, allow
p, test@example.com, logs, get, */*, allow

# Admin user with federated_claims.user_id
g, admin@example.com, role:admin
p, admin@example.com, *, *, */*, allow

# Group-based policies (these should work in both versions)
g, test-group, role:test-role
g, admin-group, role:admin`))

		})

	})
})
