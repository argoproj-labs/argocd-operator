/*
Copyright 2025 ArgoCD Operator Developers

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
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-108_validate_imagepullpolicy", func() {

		var (
			k8sClient   client.Client
			ctx         context.Context
			ns          *corev1.Namespace
			cleanupFunc func()
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			fixture.OutputDebugOnFail(ns)

			if cleanupFunc != nil {
				cleanupFunc()
			}

			// Clean up environment variable
			os.Unsetenv(common.ArgoCDImagePullPolicyEnvName)
		})

		It("ArgoCD CR ImagePullPolicy Validation", func() {
			By("verifying PullAlways is accepted")
			policyAlways := corev1.PullAlways
			argoCD := &argoproj.ArgoCD{
				Spec: argoproj.ArgoCDSpec{
					ImagePullPolicy: &policyAlways,
				},
			}
			Expect(argoCD.Spec.ImagePullPolicy).ToNot(BeNil())
			Expect(*argoCD.Spec.ImagePullPolicy).To(Equal(corev1.PullAlways))

			By("verifying PullIfNotPresent is accepted")
			policyIfNotPresent := corev1.PullIfNotPresent
			argoCD.Spec.ImagePullPolicy = &policyIfNotPresent
			Expect(*argoCD.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))

			By("verifying PullNever is accepted")
			policyNever := corev1.PullNever
			argoCD.Spec.ImagePullPolicy = &policyNever
			Expect(*argoCD.Spec.ImagePullPolicy).To(Equal(corev1.PullNever))

			By("verifying nil imagePullPolicy is allowed (uses default)")
			argoCD.Spec.ImagePullPolicy = nil
			Expect(argoCD.Spec.ImagePullPolicy).To(BeNil())

			By("creating namespace-scoped ArgoCD instance with instance level imagePullPolicy=IfNotPresent")
			ns, cleanupFunc = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			policy := corev1.PullIfNotPresent
			enabled := true
			argoCD = &argoproj.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argoproj.ArgoCDSpec{
					ImagePullPolicy: &policy,
					ApplicationSet: &argoproj.ArgoCDApplicationSet{
						Enabled: &enabled,
					},
					Notifications: argoproj.ArgoCDNotifications{
						Enabled: true,
					},
					Server: argoproj.ArgoCDServerSpec{
						Route: argoproj.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying all core deployments respect instance level imagePullPolicy setting and have imagePullPolicy=IfNotPresent")
			coreDeployments := []string{"argocd-server", "argocd-repo-server", "argocd-redis"}
			for _, deploymentName := range coreDeployments {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: ns.Name},
				}
				Eventually(deployment, "2m", "2s").Should(k8sFixture.ExistByName())
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
						return false
					}
					for _, container := range deployment.Spec.Template.Spec.Containers {
						if container.ImagePullPolicy != corev1.PullIfNotPresent {
							GinkgoWriter.Printf("%s container %s has ImagePullPolicy %s, expected %s\n",
								deploymentName, container.Name, container.ImagePullPolicy, corev1.PullIfNotPresent)
							return false
						}
					}
					return true
				}, "60s", "2s").Should(BeTrue(), "%s should have imagePullPolicy=IfNotPresent", deploymentName)
			}

			By("verifying application-controller statefulset has imagePullPolicy=IfNotPresent")
			controllerStatefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-application-controller", Namespace: ns.Name},
			}
			Eventually(controllerStatefulSet).Should(k8sFixture.ExistByName())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerStatefulSet), controllerStatefulSet); err != nil {
					return false
				}
				for _, container := range controllerStatefulSet.Spec.Template.Spec.Containers {
					if container.ImagePullPolicy != corev1.PullIfNotPresent {
						return false
					}
				}
				return true
			}, "60s", "2s").Should(BeTrue())

			By("verifying applicationset-controller deployment respects imagePullPolicy")
			appsetDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-applicationset-controller", Namespace: ns.Name},
			}
			Eventually(appsetDeployment, "2m", "2s").Should(k8sFixture.ExistByName())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appsetDeployment), appsetDeployment); err != nil {
					return false
				}
				for _, container := range appsetDeployment.Spec.Template.Spec.Containers {
					if container.ImagePullPolicy != corev1.PullIfNotPresent {
						return false
					}
				}
				return true
			}, "60s", "2s").Should(BeTrue())

			By("verifying notifications-controller deployment respects imagePullPolicy")
			notificationsDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-notifications-controller", Namespace: ns.Name},
			}
			Eventually(notificationsDeployment, "2m", "2s").Should(k8sFixture.ExistByName())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notificationsDeployment), notificationsDeployment); err != nil {
					return false
				}
				for _, container := range notificationsDeployment.Spec.Template.Spec.Containers {
					if container.ImagePullPolicy != corev1.PullIfNotPresent {
						return false
					}
				}
				return true
			}, "60s", "2s").Should(BeTrue())

			By("updating instance level imagePullPolicy to Always and verifying changes propagate")
			argocdFixture.Update(argoCD, func(ac *argoproj.ArgoCD) {
				newPolicy := corev1.PullAlways
				ac.Spec.ImagePullPolicy = &newPolicy
			})

			By("verifying server deployment updated to imagePullPolicy=Always")
			serverDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-server", Namespace: ns.Name},
			}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverDeployment), serverDeployment); err != nil {
					return false
				}
				for _, container := range serverDeployment.Spec.Template.Spec.Containers {
					if container.ImagePullPolicy != corev1.PullAlways {
						return false
					}
				}
				return true
			}, "120s", "2s").Should(BeTrue())

			By("verifying repo-server deployment also updated to imagePullPolicy=Always")
			repoDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-repo-server", Namespace: ns.Name},
			}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoDeployment), repoDeployment); err != nil {
					return false
				}
				for _, container := range repoDeployment.Spec.Template.Spec.Containers {
					if container.ImagePullPolicy != corev1.PullAlways {
						return false
					}
				}
				return true
			}, "120s", "2s").Should(BeTrue())
		})

		It("verifies default imagePullPolicy behaviour", func() {
			By("creating namespace-scoped ArgoCD instance without imagePullPolicy specified")
			ns, cleanupFunc = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			argoCD := &argoproj.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argoproj.ArgoCDSpec{
					Server: argoproj.ArgoCDServerSpec{
						Route: argoproj.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying all core deployments use default imagePullPolicy behavior")
			coreDeployments := []string{"argocd-server", "argocd-repo-server", "argocd-redis"}
			for _, deploymentName := range coreDeployments {
				deployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: ns.Name},
				}
				Eventually(deployment, "2m", "2s").Should(k8sFixture.ExistByName())
				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment); err != nil {
						return false
					}
					if len(deployment.Spec.Template.Spec.Containers) == 0 {
						return false
					}
					// Verify that imagePullPolicy is set to default  value
					// When not explicitly set by operator, IfNotPresent is the default value:
					for _, container := range deployment.Spec.Template.Spec.Containers {
						policy := container.ImagePullPolicy
						if policy != corev1.PullIfNotPresent {
							GinkgoWriter.Printf("Deployment %s container %s has unexpected ImagePullPolicy %s\n",
								deploymentName, container.Name, policy)
							return false
						}
					}
					return true
				}, "60s", "2s").Should(BeTrue(), "Deployment %s should use default imagePullPolicy", deploymentName)
			}

			By("verifying application-controller statefulset uses default imagePullPolicy")
			controllerStatefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-application-controller", Namespace: ns.Name},
			}
			Eventually(controllerStatefulSet, "2m", "2s").Should(k8sFixture.ExistByName())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerStatefulSet), controllerStatefulSet); err != nil {
					return false
				}
				for _, container := range controllerStatefulSet.Spec.Template.Spec.Containers {
					policy := container.ImagePullPolicy
					if policy != corev1.PullIfNotPresent {
						GinkgoWriter.Printf("StatefulSet container %s has unexpected ImagePullPolicy %s\n",
							container.Name, policy)
						return false
					}
				}
				return true
			}, "60s", "2s").Should(BeTrue())

		})

		It("verifies subscription env var affects instances without CR policy", func() {

			// Check if running locally - skip this test as it requires modifying operator deployment
			if os.Getenv("LOCAL_RUN") == "true" {
				Skip("Skipping subscription env var test for LOCAL_RUN - operator runs locally without deployment")
			}

			// Find the operator deployment
			operatorDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-operator-controller-manager",
					Namespace: "argocd-operator-system",
				},
			}

			By("checking if operator deployment exists")
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(operatorDeployment), operatorDeployment)
			if err != nil {
				Skip("Operator deployment not found - test requires operator running in cluster: " + err.Error())
			}

			// Store original env value for cleanup
			originalEnvValue, _ := deploymentFixture.GetEnv(operatorDeployment, common.ArgoCDImagePullPolicyEnvName)

			// Ensure cleanup happens
			defer func() {
				By("restoring original operator deployment env var")
				if originalEnvValue != nil {
					deploymentFixture.SetEnv(operatorDeployment, common.ArgoCDImagePullPolicyEnvName, *originalEnvValue)
				} else {
					deploymentFixture.RemoveEnv(operatorDeployment, common.ArgoCDImagePullPolicyEnvName)
				}
				By("waiting for operator pod to restart with original settings")
				time.Sleep(30 * time.Second)
				Eventually(operatorDeployment, "3m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))
			}()

			By("setting IMAGE_PULL_POLICY env var on operator deployment to Always")
			deploymentFixture.SetEnv(operatorDeployment, common.ArgoCDImagePullPolicyEnvName, "Always")

			By("waiting for operator pod to restart with new env var")
			time.Sleep(30 * time.Second) // Give time for pod to start terminating
			Eventually(operatorDeployment, "3m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))

			By("creating first namespace with ArgoCD instance without CR policy")
			ns1, cleanupFunc1 := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc1()

			argoCD1 := &argoproj.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns1.Name},
				Spec: argoproj.ArgoCDSpec{
					Server: argoproj.ArgoCDServerSpec{
						Route: argoproj.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD1)).To(Succeed())

			By("creating second namespace with ArgoCD instance with CR policy set")
			ns2, cleanupFunc2 := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc2()

			policyNever := corev1.PullNever
			argoCD2 := &argoproj.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns2.Name},
				Spec: argoproj.ArgoCDSpec{
					ImagePullPolicy: &policyNever,
					Server: argoproj.ArgoCDServerSpec{
						Route: argoproj.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD2)).To(Succeed())

			By("waiting for both ArgoCD instances to be ready")
			Eventually(argoCD1, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD2, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying first instance uses operator env var (Always)")
			server1 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-server", Namespace: ns1.Name},
			}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(server1), server1); err != nil {
					GinkgoWriter.Printf("Failed to get server1: %v\n", err)
					return false
				}
				for _, container := range server1.Spec.Template.Spec.Containers {
					if container.ImagePullPolicy != corev1.PullAlways {
						GinkgoWriter.Printf("Container %s has policy %s, expected Always\n", container.Name, container.ImagePullPolicy)
						return false
					}
				}
				return true
			}, "60s", "2s").Should(BeTrue(), "First instance should use operator env var (Always)")

			By("verifying second instance uses CR policy (Never) regardless of env var")
			server2 := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-server", Namespace: ns2.Name},
			}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(server2), server2); err != nil {
					GinkgoWriter.Printf("Failed to get server2: %v\n", err)
					return false
				}
				for _, container := range server2.Spec.Template.Spec.Containers {
					if container.ImagePullPolicy != corev1.PullNever {
						GinkgoWriter.Printf("Container %s has policy %s, expected Never\n", container.Name, container.ImagePullPolicy)
						return false
					}
				}
				return true
			}, "60s", "2s").Should(BeTrue(), "Second instance should use CR policy (Never)")

			By("changing operator env var to IfNotPresent")
			deploymentFixture.SetEnv(operatorDeployment, common.ArgoCDImagePullPolicyEnvName, "IfNotPresent")

			By("waiting for operator pod to restart with updated env var")
			time.Sleep(30 * time.Second)
			Eventually(operatorDeployment, "3m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))

			By("verifying first instance eventually uses new env var (IfNotPresent)")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(server1), server1); err != nil {
					GinkgoWriter.Printf("Failed to get server1: %v\n", err)
					return false
				}
				for _, container := range server1.Spec.Template.Spec.Containers {
					if container.ImagePullPolicy != corev1.PullIfNotPresent {
						GinkgoWriter.Printf("Container %s has policy %s, expected IfNotPresent\n", container.Name, container.ImagePullPolicy)
						return false
					}
				}
				return true
			}, "120s", "2s").Should(BeTrue(), "First instance should use updated env var (IfNotPresent)")

			By("verifying second instance still uses CR policy (Never), unaffected by env var change")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(server2), server2); err != nil {
					GinkgoWriter.Printf("Failed to get server2: %v\n", err)
					return false
				}
				for _, container := range server2.Spec.Template.Spec.Containers {
					if container.ImagePullPolicy != corev1.PullNever {
						GinkgoWriter.Printf("Container %s has policy %s, expected Never\n", container.Name, container.ImagePullPolicy)
						return false
					}
				}
				return true
			}, "60s", "2s").Should(BeTrue(), "Second instance should remain with CR policy (Never)")
		})

	})
})
