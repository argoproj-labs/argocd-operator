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
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-091_argocd_app_validate-applicationset-status", func() {

		// This test supersedes '1-027_validate_applicationset_status'

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that ArgoCD CR .status.applicationset field correctly reflects the status of applicationset controller workload", func() {

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating simple, namespace-scoped ArgoCD CR")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec:       argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD to have unknown status for appset controller")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.HaveApplicationSetControllerStatus("Unknown"))

			By("enabling appset controller via ArgoCD CR")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ApplicationSet = &argov1beta1api.ArgoCDApplicationSet{}
			})

			By("ensuring appset controller becomes available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.HaveApplicationSetControllerStatus("Running"))

			By("modifying appset controller image")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ApplicationSet.Image = "quay.io/argoproj/argocd@sha256:8576d347f30fa4c56a0129d1c0a0f5ed1e75662f0499f1ed7e917c405fd909dc"
			})

			// The kuttl test upon which this is based doesn't actually check if the image change was made in the Deployment

			By("deleting appset controller Deployment")
			depl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-applicationset-controller",
					Namespace: ns.Name,
				},
			}
			Expect(k8sClient.Delete(ctx, depl)).To(Succeed())

			By("ArgoCD CR .status field for appset controller should go back to 'Pending'")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.HaveApplicationSetControllerStatus("Pending"))

		})

	})
})
