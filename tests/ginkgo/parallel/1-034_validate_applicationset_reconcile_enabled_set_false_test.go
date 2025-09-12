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

	Context("1-034_validate_applicationset_reconcile_enabled_set_false", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("individually verifies that Argo CD workloads can be enabled and disabled via ArgoCD CR", func() {
			test1NS, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			eventuallyDeploymentsExist := func(deploymentsShouldExist []string) {
				for _, deployment := range deploymentsShouldExist {
					By("verifying Deployment '" + deployment + "' exists")
					depl := &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      deployment,
							Namespace: test1NS.Name,
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
							Namespace: test1NS.Name,
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
							Namespace: test1NS.Name,
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
							Namespace: test1NS.Name,
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
							Namespace: test1NS.Name,
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
							Namespace: test1NS.Name,
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
							Namespace: test1NS.Name,
						},
					}
					Eventually(ss).Should(k8sFixture.ExistByName())
				}
			}

			By("1) creating Argo CD instance with no workloads enabled")
			argocd_test1 := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-test1",
					Namespace: test1NS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
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
				},
			}
			Expect(k8sClient.Create(ctx, argocd_test1)).To(Succeed())
			Eventually(argocd_test1, "5m", "5s").Should(argocdFixture.BeAvailable())

			{

				deploymentsShouldExist := []string{}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test1-argocd-redis-ha"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test1-argocd-redis-ha"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{})

				deploymentsShouldNotExist := []string{"argocd-test1-redis", "argocd-test1-repo-server", "argocd-test1-server", "argocd-test1-applicationset-controller"}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test1-argocd-application-controller", "argocd-test1-argocd-server", "argocd-test1-redis", "argocd-test1-redis-ha"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{"argocd-test1-argocd-application-controller", "argocd-test1-argocd-server", "argocd-test1-redis", "argocd-test1-redis-ha"}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("2) updating Argo CD instance so only application controller is enabled")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {

				ac.Spec = argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(true),
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

			Eventually(argocd_test1, "2m", "5s").Should(argocdFixture.BeAvailable())

			{

				deploymentsShouldExist := []string{}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test1-argocd-application-controller"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test1-argocd-application-controller"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{"argocd-test1-application-controller"})

				deploymentsShouldNotExist := []string{"argocd-test1-redis", "argocd-test1-repo-server", "argocd-test1-server", "argocd-test1-applicationset-controller"}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test1-server", "argocd-test1-argocd-redis", "argocd-test1-repo-server", "argocd-test1-applicationset-controller"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{"argocd-test1-server", "argocd-test1-argocd-redis", "argocd-test1-repo-server", "argocd-test1-applicationset-controller"}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("3) updating Argo CD instance so only app controller and redis are enabled")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {

				ac.Spec = argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(true),
					},
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Enabled: ptr.To(true),
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

			Eventually(argocd_test1, "2m", "5s").Should(argocdFixture.BeAvailable())

			{

				deploymentsShouldExist := []string{"argocd-test1-redis"}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test1-argocd-application-controller", "argocd-test1-argocd-redis", "argocd-test1-argocd-redis-ha"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test1-argocd-application-controller", "argocd-test1-argocd-redis", "argocd-test1-argocd-redis-ha"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{"argocd-test1-application-controller"})

				deploymentsShouldNotExist := []string{"argocd-test1-repo-server", "argocd-test1-server", "argocd-test1-applicationset-controller"}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test1-argocd-server", "argocd-test1-applicationset-controller"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{"argocd-test1-argocd-server", "argocd-test1-applicationset-controller"}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("4) updating Argo CD so only controller, redis, and repo server are enabled")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {

				ac.Spec = argov1beta1api.ArgoCDSpec{
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
						Enabled: ptr.To(false),
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(false),
					},
				}

			})
			Eventually(argocd_test1, "2m", "5s").Should(argocdFixture.BeAvailable())

			{
				deploymentsShouldExist := []string{"argocd-test1-redis", "argocd-test1-repo-server"}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test1-argocd-application-controller", "argocd-test1-argocd-redis-ha", "argocd-test1-argocd-redis"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test1-argocd-application-controller", "argocd-test1-argocd-redis-ha", "argocd-test1-argocd-redis"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{"argocd-test1-application-controller"})

				deploymentsShouldNotExist := []string{"argocd-test1-server", "argocd-test1-applicationset-controller"}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test1-server", "argocd-test1-applicationset-controller"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{"argocd-test1-argocd-server", "argocd-test1-applicationset-controller"}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("5) updating Argo CD so only app controller, redis, repo server, and server are enabled")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {

				ac.Spec = argov1beta1api.ArgoCDSpec{
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
						Enabled: ptr.To(false),
					},
				}

			})
			Eventually(argocd_test1, "2m", "5s").Should(argocdFixture.BeAvailable())

			{
				deploymentsShouldExist := []string{"argocd-test1-redis", "argocd-test1-repo-server", "argocd-test1-server"}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test1-argocd-application-controller", "argocd-test1-argocd-server", "argocd-test1-argocd-redis", "argocd-test1-argocd-redis-ha"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test1-argocd-application-controller", "argocd-test1-argocd-server", "argocd-test1-argocd-redis", "argocd-test1-argocd-redis-ha"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{"argocd-test1-application-controller"})

				deploymentsShouldNotExist := []string{}
				consistentlyDeploymentsDoNotExist(deploymentsShouldNotExist)

				rolesShouldNotExist := []string{"argocd-test1-applicationset-controller"}
				consistentlyRolesDoNotExist(rolesShouldNotExist)

				roleBindingsShouldNotExist := []string{"argocd-test1-applicationset-controller"}
				consistentlyRoleBindingsDoNotExist(roleBindingsShouldNotExist)
			}

			By("6) updating Argo CD so all workloads are enabled")
			argocdFixture.Update(argocd_test1, func(ac *argov1beta1api.ArgoCD) {

				ac.Spec = argov1beta1api.ArgoCDSpec{
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
				}

			})
			Eventually(argocd_test1, "2m", "5s").Should(argocdFixture.BeAvailable())

			{
				deploymentsShouldExist := []string{"argocd-test1-redis", "argocd-test1-repo-server", "argocd-test1-server"}
				eventuallyDeploymentsExist(deploymentsShouldExist)

				roleShouldExist := []string{"argocd-test1-argocd-application-controller", "argocd-test1-argocd-server", "argocd-test1-argocd-redis", "argocd-test1-argocd-redis-ha", "argocd-test1-applicationset-controller"}
				eventuallyRolesExist(roleShouldExist)

				roleBindingShouldExist := []string{"argocd-test1-argocd-application-controller", "argocd-test1-argocd-server", "argocd-test1-argocd-redis", "argocd-test1-argocd-redis-ha", "argocd-test1-applicationset-controller"}
				eventuallyRoleBindingsExist(roleBindingShouldExist)

				eventuallyStatefulSetsExist([]string{"argocd-test1-application-controller"})
			}

		})

	})
})
