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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	statefulsetFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-043_validate_cleanup_when_using_remote", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies the when remote redis and remote repo server are enabled, that the redis/repo server workloads are not running, and also that when remote is disabled the workloads return", func() {

			By("creating simple namespace-scoped Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec:       argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			ensureDefaultWorkloadsExist := func() {

				By("verifying that the default Argo CD workloads exist and are running")

				Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

				By("verifying Argo CD statefulset is ready replicas: 1")
				ss := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-application-controller", Namespace: argoCD.Namespace}}
				Eventually(ss).Should(k8sFixture.ExistByName())
				Eventually(ss).Should(statefulsetFixture.HaveReplicas(1))
				Eventually(ss).Should(statefulsetFixture.HaveReadyReplicas(1))

				By("verifying that Argo CD Deployments use the nodeSelector value from ArgoCD CR")
				deploymentNameList := []string{"example-argocd-redis", "example-argocd-repo-server", "example-argocd-server"}

				for _, deploymentName := range deploymentNameList {

					depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: ns.Name}}
					Expect(depl).To(k8sFixture.ExistByName())

					Eventually(depl).Should(deploymentFixture.HaveReadyReplicas(1))

				}

				serviceNameList := []string{"example-argocd-repo-server", "example-argocd-redis"}

				for _, serviceName := range serviceNameList {

					service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: ns.Name,
					}}
					Eventually(service).Should(k8sFixture.ExistByName())
				}
			}

			ensureDefaultWorkloadsExist()

			By("enabling remote redis and remote repo server")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {

				ac.Spec = argov1beta1api.ArgoCDSpec{
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Remote: ptr.To("https://redis.remote.host:6379"),
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Remote: ptr.To("https://repo-server.remote.host:8081"),
					},
				}
			})

			By("verifying redis and repo server are no longer running")

			redisDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-redis",
					Namespace: ns.Name,
				},
			}
			Eventually(redisDepl).ShouldNot(k8sFixture.ExistByName())
			Consistently(redisDepl).ShouldNot(k8sFixture.ExistByName())

			repoServerDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-repo-server",
					Namespace: ns.Name,
				},
			}
			Eventually(repoServerDepl).ShouldNot(k8sFixture.ExistByName())
			Consistently(repoServerDepl).ShouldNot(k8sFixture.ExistByName())

			deletedServices := []string{"example-argocd-repo-server", "example-argocd-redis"}

			for _, deletedService := range deletedServices {
				service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
					Name:      deletedService,
					Namespace: ns.Name,
				}}
				Eventually(service).ShouldNot(k8sFixture.ExistByName())
				Consistently(service).ShouldNot(k8sFixture.ExistByName())
			}

			By("disabling remote repo server and redis")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec = argov1beta1api.ArgoCDSpec{}
			})

			ensureDefaultWorkloadsExist()
		})

	})
})
