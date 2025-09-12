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
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-109_validate_server_sidecar", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()

		})

		It("verifies that setting a sidecar in server via ArgoCD CR will cause that sidecar to appear in server Deployment", func() {

			By("creating an ArgoCD CR with a sidecar in the application controller")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						SidecarContainers: []corev1.Container{{
							Name:  "sidecar",
							Image: "quay.io/fedora/fedora:latest",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("32Mi"),
								},
							},
						}},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting the for Argo CD Application Controller Pod to be available")
			serverDepl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-server", Namespace: ns.Name}}
			Eventually(serverDepl, "3m", "5s").Should(k8sFixture.ExistByName())

			By("verifying that sidecar container within Pod has the same value that we set in Argo CD CR")

			sidecarContainer := deploymentFixture.GetTemplateSpecContainerByName("sidecar", *serverDepl)
			Expect(sidecarContainer).ToNot(BeNil())

			Expect(sidecarContainer.Name).To(Equal("sidecar"))
			Expect(sidecarContainer.Image).To(Equal("quay.io/fedora/fedora:latest"))
			Expect(sidecarContainer.Resources.Limits.Cpu().String()).To(Equal("50m"))
			Expect(sidecarContainer.Resources.Limits.Memory().String()).To(Equal("64Mi"))

			Expect(sidecarContainer.Resources.Requests.Cpu().String()).To(Equal("10m"))
			Expect(sidecarContainer.Resources.Requests.Memory().String()).To(Equal("32Mi"))

			Expect(serverDepl.Spec.Template.Spec.Containers[0].Name).To(Equal("argocd-server"))

		})

	})
})
