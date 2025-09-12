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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-044_validate_resource_limit_changes", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("validates that updating resource/limit field of ArgoCD updates the corresponding resource/limit of Deployment/StatefulSet", func() {

			By("creating simple namespace-scoped Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
						},
					},
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("2000Mi"),
							},
						},
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Resources: &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("200Mi"),
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("updating CPU limit to 2 on every component")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				argoCD.Spec.Server.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("2")
				argoCD.Spec.Controller.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("2")
				argoCD.Spec.Repo.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("2")
			})

			// Ensure the given PodTemplate has the expected CPUs
			podTemplateHasCPUs := func(name string, template corev1.PodTemplateSpec) bool {

				container := template.Spec.Containers[0]

				cpu := container.Resources.Limits.Cpu().String()

				GinkgoWriter.Println("PodTemplate", name, "has CPU limit", cpu)
				return cpu == "2"
			}

			deployments := []string{"argocd-server", "argocd-repo-server"}
			for _, deployment := range deployments {
				depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deployment, Namespace: ns.Name}}

				By("verifying the Deployment " + depl.Name + " has expected CPU limit")

				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(depl), depl); err != nil {
						GinkgoWriter.Println(err)
						return false
					}
					return podTemplateHasCPUs(depl.Name, depl.Spec.Template)
				}, "60s", "1s").Should(BeTrue())
			}

			statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "argocd-application-controller", Namespace: ns.Name}}
			By("verifying the StatefulSet " + statefulSet.Name + " has expected CPU limit")
			Eventually(statefulSet).Should(k8sFixture.ExistByName())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				return podTemplateHasCPUs(statefulSet.Name, statefulSet.Spec.Template)
			}).Should(BeTrue())

		})

	})
})
