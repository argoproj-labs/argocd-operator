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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-048_validate_controller_sharding", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that enabling sharding with 3 replicas causes Application Controller to split to 3 replicas, and disabling reverts to 1", func() {

			By("creating simple namespace-scoped Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec:       argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("enabling sharding with 3 replicas")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.Sharding = argov1beta1api.ArgoCDApplicationControllerShardSpec{
					Enabled:  true,
					Replicas: 3,
				}
			})

			By("checking all 3 replica pods exist")
			for podCount := 0; podCount <= 2; podCount++ {
				pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("argocd-application-controller-%d", podCount), Namespace: ns.Name}}
				Eventually(pod).Should(k8sFixture.ExistByName())
			}

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying REPLICAS env var is set in app controller StatefulSet")

			statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "argocd-application-controller", Namespace: ns.Name}}
			Eventually(statefulSet).Should(k8sFixture.ExistByName())
			var match bool
			for _, env := range statefulSet.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "ARGOCD_CONTROLLER_REPLICAS" && env.Value == "3" {
					match = true
					break
				}
			}
			Expect(match).To(BeTrue(), "StatefulSet should have expected ARGOCD_CONTROLLER_REPLICAS")

			By("disabling sharding")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.Sharding = argov1beta1api.ArgoCDApplicationControllerShardSpec{
					Enabled:  false,
					Replicas: 3,
				}
			})

			By("checking 2nd and 3rd replica pods no longer exist")
			for podCount := 1; podCount <= 2; podCount++ {
				pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("argocd-application-controller-%d", podCount), Namespace: ns.Name}}
				Eventually(pod).ShouldNot(k8sFixture.ExistByName())
			}

			By("verifying ARGOCD_CONTROLLER_REPLICAS is no longer present in StatefulSet")
			Eventually(statefulSet).Should(k8sFixture.ExistByName())
			var replicasVarFound bool
			for _, env := range statefulSet.Spec.Template.Spec.Containers[0].Env {
				GinkgoWriter.Println(env)
				if env.Name == "ARGOCD_CONTROLLER_REPLICAS" {
					replicasVarFound = true
					break
				}
			}
			Expect(replicasVarFound).To(BeFalse(), "If sharding is disabled then the ARGOCD_CONTROLLER_REPLICAS var is not set")

		})

	})
})
