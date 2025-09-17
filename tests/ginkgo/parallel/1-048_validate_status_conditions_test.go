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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-048_validate_status_conditions", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that .status.condition correct reports configuration errors, such as sso dex and keycloak", func() {

			By("create an Argo CD instance with empty SSO, which is invalid")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					SSO: &argov1beta1api.ArgoCDSSOSpec{},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying that the invalid Argo CD CR shows the expected error message")

			Eventually(argoCD, "2m", "5s").Should(
				And(argocdFixture.HavePhase("Failed"),
					argocdFixture.HaveSSOStatus("Failed"),
				))

			Eventually(argoCD, "2m", "5s").Should(argocdFixture.HaveCondition(metav1.Condition{
				Message: "illegal SSO configuration: Unsupported SSO provider type. Supported provider is dex",
				Reason:  "ErrorOccurred",
				Status:  "False",
				Type:    "Reconciled",
			}))

			By("setting a valid dex configuration in SSO field, which should fix the condition error")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.SSO = &argov1beta1api.ArgoCDSSOSpec{
					Dex: &argov1beta1api.ArgoCDDexSpec{
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
						OpenShiftOAuth: true,
					},
					Provider: argov1beta1api.SSOProviderTypeDex,
				}
			})

			By("verifying the phase and SSO status are now correct, and the error condition has been removed")
			Eventually(argoCD, "2m", "5s").Should(
				And(argocdFixture.HavePhase("Available"),
					argocdFixture.HaveSSOStatus("Running"),
				))

			Eventually(argoCD, "2m", "5s").Should(argocdFixture.HaveCondition(metav1.Condition{
				Message: "",
				Reason:  "Success",
				Status:  "True",
				Type:    "Reconciled",
			}))

			By("modifying provider to Keycloak, which is invalid")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.SSO.Provider = argov1beta1api.SSOProviderTypeKeycloak
			})

			By("verifying .status.condition goes to invalid with expected error message")
			Eventually(argoCD, "2m", "5s").Should(
				And(argocdFixture.HavePhase("Failed"),
					argocdFixture.HaveSSOStatus("Failed"),
				))

			Eventually(argoCD, "2m", "5s").Should(argocdFixture.HaveCondition(metav1.Condition{
				Message: "keycloak is set as SSO provider, but keycloak support has been deprecated and is no longer available",
				Reason:  "ErrorOccurred",
				Status:  "False",
				Type:    "Reconciled",
			}))

			By("modifying provider to Keycloak, which is invalid")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.SSO.Provider = argov1beta1api.SSOProviderTypeDex
			})

			By("verifying the phase and SSO status are now correct, and the error condition has been removed")
			Eventually(argoCD, "2m", "5s").Should(
				And(argocdFixture.HavePhase("Available"),
					argocdFixture.HaveSSOStatus("Running"),
				))

			Eventually(argoCD, "2m", "5s").Should(argocdFixture.HaveCondition(metav1.Condition{
				Message: "",
				Reason:  "Success",
				Status:  "True",
				Type:    "Reconciled",
			}))

		})

	})
})
