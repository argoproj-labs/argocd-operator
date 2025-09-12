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

	Context("1-119_argocd_respectRBAC", func() {

		// This test supersedes 1-045_validate_controller_respect_rbac

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()

		})

		It("ensures that setting .spec.controller.respectRBAC will cause that value to be set in Argo CD's argocd-cm ConfigMap, and that invalid values are ignored", func() {

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating basic Argo CD instance with respect RBAC set to normal")

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						RespectRBAC: "normal",
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying strict respectRBAC is set in argocd-cm ConfigMap")
			argocdCMConfigMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: ns.Name,
				},
			}
			Eventually(argocdCMConfigMap).Should(k8sFixture.ExistByName())

			Eventually(argocdCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("resource.respectRBAC", "normal"))

			By("updating Argo CD instance to respect RBAC set to strict")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.RespectRBAC = "strict"
			})

			By("verifying strict respectRBAC is set in argocd-cm ConfigMap")
			Eventually(argocdCMConfigMap).Should(k8sFixture.ExistByName())
			Eventually(argocdCMConfigMap).Should(configmapFixture.HaveStringDataKeyValue("resource.respectRBAC", "strict"))

			By("updating Argo CD instance to respect RBAC set to invalid valid")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.RespectRBAC = "somethibg"
			})

			Eventually(argocdCMConfigMap).ShouldNot(configmapFixture.HaveStringDataKeyValue("resource.respectRBAC", "strict"))
			Consistently(argocdCMConfigMap).ShouldNot(configmapFixture.HaveStringDataKeyValue("resource.respectRBAC", "strict"))

			By("updating Argo CD instance to respect RBAC set to empty value")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.RespectRBAC = ""
			})

			Eventually(argocdCMConfigMap).ShouldNot(configmapFixture.HaveStringDataKeyValue("resource.respectRBAC", "strict"))
			Consistently(argocdCMConfigMap).ShouldNot(configmapFixture.HaveStringDataKeyValue("resource.respectRBAC", "strict"))

		})

	})
})
