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

package parallel

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-125_validate_applicationset_cleanup_when_spec_field_omitted", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("removes ApplicationSet workload and namespaced RBAC when spec.applicationSet is removed from the ArgoCD CR", func() {
			testNS, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			const crName = "argocd-appset-spec-omit"
			appsetWorkloadName := crName + "-applicationset-controller"

			By("creating Argo CD with spec.applicationSet set to an empty object")
			argocdCR := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crName,
					Namespace: testNS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(true),
					},
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Enabled: ptr.To(true),
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Enabled: ptr.To(true),
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Enabled: ptr.To(true),
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{},
				},
			}
			Expect(k8sClient.Create(ctx, argocdCR)).To(Succeed())
			Eventually(argocdCR, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying ApplicationSet controller resources exist")
			depl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: appsetWorkloadName, Namespace: testNS.Name},
			}
			Eventually(depl).Should(k8sFixture.ExistByName())

			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: appsetWorkloadName, Namespace: testNS.Name},
			}
			Eventually(sa).Should(k8sFixture.ExistByName())

			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: appsetWorkloadName, Namespace: testNS.Name},
			}
			Eventually(role).Should(k8sFixture.ExistByName())

			rb := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: appsetWorkloadName, Namespace: testNS.Name},
			}
			Eventually(rb).Should(k8sFixture.ExistByName())

			svc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: appsetWorkloadName, Namespace: testNS.Name},
			}
			Eventually(svc).Should(k8sFixture.ExistByName())

			By("removing spec.applicationSet from the ArgoCD CR")
			argocdFixture.Update(argocdCR, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ApplicationSet = nil
			})
			Eventually(argocdCR, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying ApplicationSet resources are deleted")
			Eventually(depl).Should(k8sFixture.NotExistByName())
			Consistently(depl).Should(k8sFixture.NotExistByName())

			Eventually(sa).Should(k8sFixture.NotExistByName())
			Consistently(sa).Should(k8sFixture.NotExistByName())

			Eventually(role).Should(k8sFixture.NotExistByName())
			Consistently(role).Should(k8sFixture.NotExistByName())

			Eventually(rb).Should(k8sFixture.NotExistByName())
			Consistently(rb).Should(k8sFixture.NotExistByName())

			Eventually(svc).Should(k8sFixture.NotExistByName())
			Consistently(svc).Should(k8sFixture.NotExistByName())
		})
	})
})
