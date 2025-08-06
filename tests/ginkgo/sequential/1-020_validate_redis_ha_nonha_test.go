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

package sequential

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	nodeFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/node"
	statefulsetFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-020_validate_redis_ha_nonha", func() {

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
		})

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {

			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = utils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("validates Redis HA and Non-HA", func() {

			// This test enables HA, so it needs to be running on a cluster with at least 3 nodes
			nodeFixture.ExpectHasAtLeastXNodes(3)

			By("creating simple namespace-scoped Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCDInstance := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name,
					Labels: map[string]string{"example": "basic"}},
				Spec: argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCDInstance)).To(Succeed())

			Eventually(argoCDInstance, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCDInstance).Should(argocdFixture.HaveRedisStatus("Running"))

			By("verifying various expected resources exist in namespace")
			Eventually(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis", Namespace: ns.Name}}).Should(k8sFixture.ExistByName())

			depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis", Namespace: ns.Name}}
			Eventually(depl).Should(k8sFixture.ExistByName())
			Eventually(depl).Should(deploymentFixture.HaveReadyReplicas(1))

			By("verifies Redis HA resources should not exist since we are in non-HA mode")

			Consistently(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha", Namespace: ns.Name}}).Should(k8sFixture.NotExistByName())

			Consistently(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha-haproxy", Namespace: ns.Name}}).Should(k8sFixture.NotExistByName())

			Consistently(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha-haproxy", Namespace: ns.Name}}).Should(k8sFixture.NotExistByName())

			Consistently(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha-server", Namespace: ns.Name}}).Should(k8sFixture.NotExistByName())

			By("enabling HA on Argo CD instance")
			argocdFixture.Update(argoCDInstance, func(argocd *argov1beta1api.ArgoCD) {
				argocd.Spec.HA.Enabled = true
			})

			By("verifying expected HA resources are eventually created after we enabled HA")

			Eventually(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha", Namespace: ns.Name}}).Should(k8sFixture.ExistByName())

			Eventually(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha-haproxy", Namespace: ns.Name}}).Should(k8sFixture.ExistByName())

			Eventually(argoCDInstance, "4m", "5s").Should(argocdFixture.HavePhase("Available"))
			Eventually(argoCDInstance).Should(argocdFixture.HaveRedisStatus("Running"))

			statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha-server", Namespace: ns.Name}}
			Eventually(statefulSet).Should(statefulsetFixture.HaveReadyReplicas(3))
			Expect(statefulSet.Spec.Template.Spec.Affinity).To(Equal(
				&corev1.Affinity{
					PodAntiAffinity: &corev1.PodAntiAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
							{
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{
										"app.kubernetes.io/name": argoCDInstance.Name + "-redis-ha",
									},
								},
								TopologyKey: "kubernetes.io/hostname",
							},
						},
					},
				}))

			Eventually(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha-haproxy", Namespace: ns.Name}}, "60s", "5s").Should(deploymentFixture.HaveReadyReplicas(3))

			By("verifying non-HA resources no longer exist, since HA is enabled")

			Expect(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis", Namespace: ns.Name}}).To(k8sFixture.NotExistByName())

			Expect(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis", Namespace: ns.Name}}).To(k8sFixture.NotExistByName())

			By("updating ArgoCD CR to add cpu and memory resource request and limits to HA workloads")

			argocdFixture.Update(argoCDInstance, func(argocd *argov1beta1api.ArgoCD) {
				argocd.Spec.HA.Enabled = true
				argocd.Spec.HA.Resources = &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				}
			})

			By("Argo CD should eventually be ready after updating the resource requirements")
			Eventually(argoCDInstance, "5m", "5s").Should(argocdFixture.BeAvailable()) // it can take a while to schedule the Pods
			Eventually(argoCDInstance, "5m", "5s").Should(argocdFixture.HaveRedisStatus("Running"))

			By("verifying Deployment and StatefulSet have expected resources that we set in previous step")

			depl = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha-haproxy", Namespace: ns.Name}}
			Eventually(depl, "2m", "5s").Should(deploymentFixture.HaveReadyReplicas(3))

			haProxyContainer := deploymentFixture.GetTemplateSpecContainerByName("haproxy", *depl)

			Expect(haProxyContainer).ToNot(BeNil())
			Expect(haProxyContainer.Resources.Limits.Cpu().AsDec().String()).To(Equal("0.500"))
			Expect(haProxyContainer.Resources.Limits.Memory().AsDec().String()).To(Equal("268435456")) // 256Mib in bytes
			Expect(haProxyContainer.Resources.Requests.Cpu().AsDec().String()).To(Equal("0.200"))
			Expect(haProxyContainer.Resources.Requests.Memory().AsDec().String()).To(Equal("134217728")) // 128MiB  in bytes

			configInitContainer := deploymentFixture.GetTemplateSpecInitContainerByName("config-init", *depl)

			Expect(configInitContainer.Resources.Limits.Cpu().AsDec().String()).To(Equal("0.500"))
			Expect(configInitContainer.Resources.Limits.Memory().AsDec().String()).To(Equal("268435456"))
			Expect(configInitContainer.Resources.Requests.Cpu().AsDec().String()).To(Equal("0.200"))
			Expect(configInitContainer.Resources.Requests.Memory().AsDec().String()).To(Equal("134217728"))

			ss := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha-server", Namespace: ns.Name}}
			Eventually(ss, "2m", "5s").Should(statefulsetFixture.HaveReadyReplicas(3))

			redisContainer := statefulsetFixture.GetTemplateSpecContainerByName("redis", *ss)
			Expect(redisContainer).ToNot(BeNil())
			Expect(redisContainer.Resources.Limits.Cpu().AsDec().String()).To(Equal("0.500"))
			Expect(redisContainer.Resources.Limits.Memory().AsDec().String()).To(Equal("268435456"))
			Expect(redisContainer.Resources.Requests.Cpu().AsDec().String()).To(Equal("0.200"))
			Expect(redisContainer.Resources.Requests.Memory().AsDec().String()).To(Equal("134217728"))

			sentinelContainer := statefulsetFixture.GetTemplateSpecContainerByName("sentinel", *ss)
			Expect(sentinelContainer).ToNot(BeNil())
			Expect(sentinelContainer.Resources.Limits.Cpu().AsDec().String()).To(Equal("0.500"))
			Expect(sentinelContainer.Resources.Limits.Memory().AsDec().String()).To(Equal("268435456"))
			Expect(sentinelContainer.Resources.Requests.Cpu().AsDec().String()).To(Equal("0.200"))
			Expect(sentinelContainer.Resources.Requests.Memory().AsDec().String()).To(Equal("134217728"))

			configInitContainer = statefulsetFixture.GetTemplateSpecInitContainerByName("config-init", *ss)
			Expect(configInitContainer.Resources.Limits.Cpu().AsDec().String()).To(Equal("0.500"))
			Expect(configInitContainer.Resources.Limits.Memory().AsDec().String()).To(Equal("268435456"))
			Expect(configInitContainer.Resources.Requests.Cpu().AsDec().String()).To(Equal("0.200"))
			Expect(configInitContainer.Resources.Requests.Memory().AsDec().String()).To(Equal("134217728"))

			By("disabling HA on ArgoCD CR")

			argocdFixture.Update(argoCDInstance, func(argocd *argov1beta1api.ArgoCD) {
				argocd.Spec.HA.Enabled = false
			})

			By("verifying Argo CD becomes ready again after HA is disabled")

			Eventually(argoCDInstance, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCDInstance, "60s", "5s").Should(argocdFixture.HaveRedisStatus("Running"))

			By("verifying expected non-HA resources exist again and HA resources no longer exist")
			depl = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis", Namespace: ns.Name}}
			Eventually(depl).Should(k8sFixture.ExistByName())
			Eventually(depl).Should(deploymentFixture.HaveReadyReplicas(1))

			Consistently(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha-haproxy", Namespace: ns.Name}}).Should(k8sFixture.NotExistByName())

			Consistently(&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: argoCDInstance.Name + "-redis-ha-server", Namespace: ns.Name}}).Should(k8sFixture.NotExistByName())

		})
	})
})
