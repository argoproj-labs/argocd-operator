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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-029_validate_applicationset_extra_command_args", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()

		})

		It("ensures that extra arguments can be added to application set controller", func() {

			By("creating a simple ArgoCD CR and waiting for it to become available")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying application set controller Deployment becomes available")
			appSetControllerDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-applicationset-controller",
					Namespace: ns.Name,
				},
			}

			Eventually(appSetControllerDepl).Should(k8sFixture.ExistByName())
			Eventually(appSetControllerDepl).Should(deploymentFixture.HaveReadyReplicas(1))

			By("adding a new parameter via .spec.applicationset.extraCommandArgs")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ApplicationSet.ExtraCommandArgs = []string{"--enable-progressive-rollouts"}
			})

			By("verifying new parameter is added, and the existing parameters are still present")
			Eventually(appSetControllerDepl).Should(deploymentFixture.HaveContainerCommandSubstring("--enable-progressive-rollouts", 0))

			Expect(len(appSetControllerDepl.Spec.Template.Spec.Containers[0].Command)).To(BeNumerically(">=", 7))

			By("removing the extra command arg")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ApplicationSet.ExtraCommandArgs = nil
			})

			By("verifying the parameter has been removed")
			Eventually(appSetControllerDepl).ShouldNot(deploymentFixture.HaveContainerCommandSubstring("--enable-progressive-rollouts", 0))
			Consistently(appSetControllerDepl).ShouldNot(deploymentFixture.HaveContainerCommandSubstring("--enable-progressive-rollouts", 0))
			Expect(len(appSetControllerDepl.Spec.Template.Spec.Containers[0].Command)).To(BeNumerically(">=", 7))

		})

	})
})
