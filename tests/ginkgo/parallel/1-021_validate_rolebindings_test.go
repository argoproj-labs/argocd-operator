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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-021_validate_rolebindings", func() {

		var (
			ctx       context.Context
			k8sClient client.Client
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies a namespace-scoped Argo CD instance has the expected RoleBindings", func() {

			By("creating new namespace-scoped Argo CD instance and verifying it becomes available")
			randomNS, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: randomNS.Name},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying expected RoleBindings exist and have the expected contents")
			rbsToCheck := []string{
				"argocd-argocd-application-controller",
				"argocd-argocd-redis-ha",
				"argocd-argocd-server",
			}
			for _, rbName := range rbsToCheck {
				rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: argoCD.Namespace}}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rb), rb)).To(Succeed())

				Expect(rb.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
				Expect(rb.RoleRef.Kind).To(Equal("Role"))
				Expect(rb.RoleRef.Name).To(Equal(rbName), "RoleBinding and ServiceAccount have the same name")

				Expect(rb.Subjects).To(HaveLen(1))
				Expect(rb.Subjects[0].Kind).To(Equal("ServiceAccount"))
				Expect(rb.Subjects[0].Name).To(Equal(rbName))
			}

		})

	})
})
