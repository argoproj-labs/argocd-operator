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
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-099_validate_server_autoscale", func() {

		// This test supersedes '1-032_validate_server_hpa'

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that setting ArgoCD CR Server replicas and autoscaling affect the corresponding Deployment and HPA values", func() {

			By("creating simple Argo CD instance with 2 server replicas")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Replicas: ptr.To(int32(2)),
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying Argo CD Server component Deployment has expected values from ArgoCD CR")

			serverDepl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-server", Namespace: ns.Name}}
			Eventually(serverDepl).Should(k8sFixture.ExistByName())
			Expect(*serverDepl.Spec.Replicas).To(Equal(int32(2)))

			By("verifying Deployment is Available")
			Eventually(serverDepl).Should(deploymentFixture.HaveConditionTypeStatus(appsv1.DeploymentAvailable, corev1.ConditionTrue))
			Eventually(serverDepl).Should(deploymentFixture.HaveConditionTypeStatus(appsv1.DeploymentProgressing, corev1.ConditionTrue))

			By("enabling Argo CD Server autoscaling")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Autoscale = argov1beta1api.ArgoCDServerAutoscaleSpec{
					Enabled: true,
					HPA: &autoscalingv1.HorizontalPodAutoscalerSpec{
						MinReplicas:                    ptr.To(int32(4)),
						MaxReplicas:                    int32(7),
						TargetCPUUtilizationPercentage: ptr.To(int32(50)),
						ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
							Kind:       "deployment",
							APIVersion: "apps/v1",
							Name:       "example-argocd-server",
						},
					},
				}
			})

			By("verifying autoscaling values are set on Server Deployment replicas")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDepl), serverDepl); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				replicas := *serverDepl.Spec.Replicas
				GinkgoWriter.Println("serverDepl replicas", replicas)

				return replicas >= 4 && replicas <= 7
			}, "1m", "5s").Should(BeTrue(), "server replica count should match expectation")

			By("updating the autoscaling values on Argo CD CR")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Autoscale = argov1beta1api.ArgoCDServerAutoscaleSpec{
					Enabled: true,
					HPA: &autoscalingv1.HorizontalPodAutoscalerSpec{
						MinReplicas:                    ptr.To(int32(8)),
						MaxReplicas:                    int32(12),
						TargetCPUUtilizationPercentage: ptr.To(int32(50)),
						ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
							Kind:       "deployment",
							APIVersion: "apps/v1",
							Name:       "example-argocd-server",
						},
					},
				}
			})

			By("verifying that the values set on ArgoCD CR are eventually set on the example-argocd-server HPA")
			hpa := &autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-server", Namespace: ns.Name},
			}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(hpa), hpa); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				if hpa.Spec.MaxReplicas != int32(12) {
					GinkgoWriter.Println(hpa.Spec.MaxReplicas)
					return false
				}

				if *hpa.Spec.MinReplicas != int32(8) {
					GinkgoWriter.Println(hpa.Spec.MinReplicas)
					return false
				}

				str := hpa.Spec.ScaleTargetRef
				if str.APIVersion != "apps/v1" || str.Kind != "deployment" || str.Name != "example-argocd-server" {
					GinkgoWriter.Println(str)
					return false
				}

				if len(hpa.Spec.Metrics) != 1 {
					GinkgoWriter.Println(len(hpa.Spec.Metrics))
					return false
				}

				metric := hpa.Spec.Metrics[0]

				if metric.Resource == nil {
					GinkgoWriter.Println(metric.Resource)
					return false
				}

				if metric.Resource.Name != "cpu" {
					GinkgoWriter.Println(metric.Resource.Name)
					return false
				}

				if *metric.Resource.Target.AverageUtilization != int32(50) {
					GinkgoWriter.Println(*metric.Resource.Target.AverageUtilization)
					return false
				}
				if metric.Resource.Target.Type != autoscalingv2.UtilizationMetricType {
					GinkgoWriter.Println(metric.Resource.Target.Type)
					return false
				}

				return true

			}).Should(BeTrue())
		})

	})
})
