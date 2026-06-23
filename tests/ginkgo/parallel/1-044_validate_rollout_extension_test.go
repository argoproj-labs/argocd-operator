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
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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

	Context("1-044_validate_rollout_extension", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that enableRolloutsUI can be enabled/disabled on ArgoCD CR, and the server Deployment is updated accordingly", func() {

			By("creating simple Argo CD instance enableRolloutsUI: true")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						EnableRolloutsUI: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying argocd-server exists")
			argoCDServer := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-server",
					Namespace: ns.Name,
				},
			}
			Eventually(argoCDServer, "2m", "5s").Should(k8sFixture.ExistByName())

			By("verifying rollout-extension init-container exists, and has the correct values and volume  mounts")
			initContainer := deploymentFixture.GetTemplateSpecInitContainerByName("rollout-extension", *argoCDServer)
			Expect(initContainer).ToNot(BeNil())

			Expect(initContainer.Image).To(Equal("quay.io/argoprojlabs/argocd-extension-installer:v0.0.8"))

			Expect(initContainer.Env).To(Equal([]corev1.EnvVar{
				{Name: "EXTENSION_URL", Value: "https://github.com/argoproj-labs/rollout-extension/releases/download/v0.3.6/extension.tar"},
			}))

			Expect(initContainer.VolumeMounts).To(Equal([]corev1.VolumeMount{
				{
					Name:      "rollout-extensions",
					MountPath: "/tmp/extensions/",
				},
				{
					Name:      "tmp",
					MountPath: "/tmp",
				},
			}))

			Expect(*initContainer.SecurityContext).To(Equal(corev1.SecurityContext{
				AllowPrivilegeEscalation: ptr.To(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						"ALL",
					},
				},
				ReadOnlyRootFilesystem: ptr.To(true),
				RunAsNonRoot:           ptr.To(true),
				// RunAsUser:              ptr.To(int64(999)),
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeRuntimeDefault,
				},
			}))

			By("verifying argo cd server has expected rollout-extensions volume")
			Expect(argoCDServer).Should(deploymentFixture.HaveSpecTemplateSpecVolume(corev1.Volume{
				Name: "rollout-extensions",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}))

			By("verifying argocd-server container has volume mount to rollout-extensions")
			container := deploymentFixture.GetTemplateSpecContainerByName("argocd-server", *argoCDServer)
			Expect(container).ToNot(BeNil())

			match := false
			for _, volumeMount := range container.VolumeMounts {

				if reflect.DeepEqual(volumeMount, corev1.VolumeMount{
					Name:      "rollout-extensions",
					MountPath: "/tmp/extensions/",
				}) {
					match = true
				}
			}
			Expect(match).To(BeTrue())

			By("disabling Rollouts UI")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.EnableRolloutsUI = false
			})

			Eventually(argoCDServer, "2m", "5s").Should(k8sFixture.ExistByName())

			By("verifying init container is no longer specified")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCDServer), argoCDServer); err != nil {
					GinkgoWriter.Println(err)
					return false
				}
				return len(argoCDServer.Spec.Template.Spec.InitContainers) == 0
			}).Should(BeTrue())

			By("verifying rollout-extensions volume no longer exists")
			match = false
			for _, volume := range argoCDServer.Spec.Template.Spec.Volumes {

				if volume.Name == "rollout-extensions" {
					match = true
				}
			}
			Expect(match).To(BeFalse())

			By("verifying volume mount into argocd-server container for rollout extension no longer exists")
			container = deploymentFixture.GetTemplateSpecContainerByName("argocd-server", *argoCDServer)
			Expect(container).ToNot(BeNil())

			match = false
			for _, volumeMount := range container.VolumeMounts {

				if volumeMount.Name == "rollout-extensions" {
					match = true
				}
			}
			Expect(match).To(BeFalse())

		})
	})
})
