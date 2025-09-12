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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-035_validate_applicationset_reconcile_enabled_set_true", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("individually verifies that Argo CD workloads can be enabled and disabled via ArgoCD CR, beginning with all workloads enabled", func() {
			testNS, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			eventuallyDeploymentsExist := func(deploymentsShouldExist []string) {
				for _, deployment := range deploymentsShouldExist {
					By("verifying Deployment '" + deployment + "' exists")
					depl := &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      deployment,
							Namespace: testNS.Name,
						},
					}
					Eventually(depl).Should(k8sFixture.ExistByName())
				}
			}

			consistentlyDeploymentsDoNotExist := func(deploymentsShouldNotExist []string) {
				for _, deployment := range deploymentsShouldNotExist {
					By("verifying Deployment '" + deployment + "' consistently doesn't exist")
					depl := &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      deployment,
							Namespace: testNS.Name,
						},
					}
					Eventually(depl).Should(k8sFixture.NotExistByName())
					Consistently(depl).Should(k8sFixture.NotExistByName())
				}
			}

			eventuallyRolesExist := func(roleShouldExist []string) {
				for _, roleName := range roleShouldExist {
					By("verifying Role '" + roleName + "' exists")
					role := &rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      roleName,
							Namespace: testNS.Name,
						},
					}
					Eventually(role).Should(k8sFixture.ExistByName())
				}
			}
			consistentlyRolesDoNotExist := func(roleShouldNotExist []string) {
				for _, roleName := range roleShouldNotExist {
					By("verifying Role '" + roleName + "' consistently doesn't exist")
					role := &rbacv1.Role{
						ObjectMeta: metav1.ObjectMeta{
							Name:      roleName,
							Namespace: testNS.Name,
						},
					}
					Eventually(role).Should(k8sFixture.NotExistByName())
					Consistently(role).Should(k8sFixture.NotExistByName())
				}
			}

			eventuallyRoleBindingsExist := func(rolebindingsShouldExist []string) {
				for _, rolebindingName := range rolebindingsShouldExist {
					By("verifying RoleBinding '" + rolebindingName + "' exists")
					rb := &rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      rolebindingName,
							Namespace: testNS.Name,
						},
					}
					Eventually(rb).Should(k8sFixture.ExistByName())
				}
			}

			consistentlyRoleBindingsDoNotExist := func(rolebindingsShouldNotExist []string) {
				for _, rolebindingName := range rolebindingsShouldNotExist {
					By("verifying RoleBinding '" + rolebindingName + "' consistently doesn't exist")
					rb := &rbacv1.RoleBinding{
						ObjectMeta: metav1.ObjectMeta{
							Name:      rolebindingName,
							Namespace: testNS.Name,
						},
					}
					Eventually(rb).Should(k8sFixture.NotExistByName())
					Consistently(rb).Should(k8sFixture.NotExistByName())
				}
			}

			eventuallyStatefulSetsExist := func(statefulsetShouldExist []string) {
				for _, ssName := range statefulsetShouldExist {
					By("verifying StatefulSet '" + ssName + "' exists")
					ss := &appsv1.StatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      ssName,
							Namespace: testNS.Name,
						},
					}
					Eventually(ss).Should(k8sFixture.ExistByName())
				}
			}

			consistentlyStatefulSetsDoNotExist := func(statefulsetShouldNotExist []string) {
				for _, ssName := range statefulsetShouldNotExist {
					By("verifying StatefulSet '" + ssName + "' consistently doesn't exist")
					ss := &appsv1.StatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      ssName,
							Namespace: testNS.Name,
						},
					}
					Eventually(ss).Should(k8sFixture.NotExistByName())
					Consistently(ss).Should(k8sFixture.NotExistByName())

				}
			}

			By("1) create Argo CD instance with all workloads enabled")
			argocd_test1 := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-test",
					Namespace: testNS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(true),
					},
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Enabled: ptr.To(true),
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Enabled: ptr.To(true),
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Enabled: ptr.To(true),
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(true),
					},
				},
			}
			Expect(k8sClient.Create(ctx, argocd_test1)).To(Succeed())
			Eventually(argocd_test1, "5m", "5s").Should(argocdFixture.BeAvailable())

			{

				deploymentsShouldExist := []string{"argocd-test-redis", "argocd-test-repo-server", "argocd-test-server", "argocd-test-applicationset-controller"}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test-argocd-application-controller", "argocd-test-argocd-server", "argocd-test-argocd-redis-ha", "argocd-test-argocd-redis", "argocd-test-applicationset-controller"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test-argocd-application-controller", "argocd-test-argocd-server", "argocd-test-argocd-redis-ha", "argocd-test-argocd-redis", "argocd-test-applicationset-controller"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{"argocd-test-application-controller"})

				deploymentsShouldNotExist := []string{}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("2) update Argo CD instance to include all workloads except controller")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec = argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(false),
					},
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Enabled: ptr.To(true),
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Enabled: ptr.To(true),
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Enabled: ptr.To(true),
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(true),
					},
				}

			})
			Eventually(argocd_test1, "5m", "5s").Should(argocdFixture.BeAvailable())

			{

				deploymentsShouldExist := []string{"argocd-test-redis", "argocd-test-repo-server", "argocd-test-server", "argocd-test-applicationset-controller"}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test-argocd-server", "argocd-test-argocd-redis-ha", "argocd-test-argocd-redis", "argocd-test-applicationset-controller"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test-argocd-server", "argocd-test-argocd-redis-ha", "argocd-test-argocd-redis", "argocd-test-applicationset-controller"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{})

				consistentlyStatefulSetsDoNotExist([]string{"argocd-test-application-controller"})

				deploymentsShouldNotExist := []string{}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test-application-controller"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{"argocd-test-application-controller"}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("3) update Argo CD instance to include all workloads except controller and redis")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec = argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(false),
					},
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Enabled: ptr.To(false),
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Enabled: ptr.To(true),
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Enabled: ptr.To(true),
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(true),
					},
				}

			})

			Eventually(argocd_test1, "5m", "5s").Should(argocdFixture.BeAvailable())

			{

				deploymentsShouldExist := []string{"argocd-test-repo-server", "argocd-test-server", "argocd-test-applicationset-controller"}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test-argocd-server", "argocd-test-applicationset-controller"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test-argocd-server", "argocd-test-applicationset-controller"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{})

				consistentlyStatefulSetsDoNotExist([]string{"argocd-test-application-controller"})

				deploymentsShouldNotExist := []string{"argocd-test-redis"}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test-argocd-redis", "argocd-test-application-controller"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{"argocd-test-argocd-redis", "argocd-test-application-controller"}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("4) update Argo CD instance to include all workloads except controller, redis, and repo server")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec = argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(false),
					},
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Enabled: ptr.To(false),
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Enabled: ptr.To(false),
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Enabled: ptr.To(true),
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(true),
					},
				}

			})

			Eventually(argocd_test1, "5m", "5s").Should(argocdFixture.BeAvailable())

			{

				deploymentsShouldExist := []string{"argocd-test-server", "argocd-test-applicationset-controller"}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test-argocd-server", "argocd-test-applicationset-controller"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test-argocd-server", "argocd-test-applicationset-controller"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{})

				consistentlyStatefulSetsDoNotExist([]string{"argocd-test-application-controller"})

				deploymentsShouldNotExist := []string{"argocd-test-redis", "argocd-test-repo-server"}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test-application-controller", "argocd-test-redis", "argocd-test-repo-server"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{"argocd-test-application-controller", "argocd-test-redis", "argocd-test-repo-server"}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("5) update Argo CD instance to only include applicationset")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec = argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(false),
					},
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Enabled: ptr.To(false),
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Enabled: ptr.To(false),
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Enabled: ptr.To(false),
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(true),
					},
				}
			})
			Eventually(argocd_test1, "5m", "5s").Should(argocdFixture.BeAvailable())

			{

				deploymentsShouldExist := []string{"argocd-test-applicationset-controller"}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test-applicationset-controller"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test-applicationset-controller"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{})

				consistentlyStatefulSetsDoNotExist([]string{"argocd-test-application-controller"})

				deploymentsShouldNotExist := []string{"argocd-test-server", "argocd-test-redis", "argocd-test-repo-server"}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test-argocd-application-controller", "argocd-test-argocd-server", "argocd-test-argocd-redis"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{"argocd-test-argocd-application-controller", "argocd-test-argocd-server", "argocd-test-argocd-redis"}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("6) Update Argo CD instance so no workloads are enabled")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec = argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(false),
					},
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Enabled: ptr.To(false),
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Enabled: ptr.To(false),
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Enabled: ptr.To(false),
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(false),
					},
				}

			})

			{

				deploymentsShouldExist := []string{}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{})

				consistentlyStatefulSetsDoNotExist([]string{})

				deploymentsShouldNotExist := []string{"argocd-test-redis", "argocd-test-repo-server", "argocd-test-server"}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test-server", "argocd-test-application-controller"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("7) Update Argo CD instance so no workloads are enabled, but HA is enabled")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec = argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(false),
					},
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Enabled: ptr.To(false),
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Enabled: ptr.To(false),
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Enabled: ptr.To(false),
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(false),
					},
					HA: argov1beta1api.ArgoCDHASpec{
						Enabled: true,
					},
				}

			})

			{

				deploymentsShouldExist := []string{}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{})

				consistentlyStatefulSetsDoNotExist([]string{})

				deploymentsShouldNotExist := []string{"argocd-test-redis", "argocd-test-repo-server", "argocd-test-server"}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test-server", "argocd-test-application-controller"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}
		})

	})

})
