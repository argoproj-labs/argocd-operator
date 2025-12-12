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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-055_validate_notification_controller", func() {
		// This test supersedes 1-021_validate_notification_controller

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensuring that notification controller can be enabled and disabled, and that notification controller k8s resources are created/deleted as expected", func() {

			By("creating simple namespace-scoped Argo CD instance with notification controller disabled")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argocd := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled: false,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argocd)).To(Succeed())

			By("verifying notification controller status is not set")

			Eventually(argocd, "5m", "5s").Should(argocdFixture.BeAvailable())

			Eventually(argocd).Should(argocdFixture.HaveNotificationControllerStatus(""))

			By("enabling notification controller")
			argocdFixture.Update(argocd, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Notifications = argov1beta1api.ArgoCDNotifications{
					Enabled: true,
				}
			})

			By("verifying Argo CD and notification controller start as expected")
			Eventually(argocd, "4m", "5s").Should(argocdFixture.HaveNotificationControllerStatus("Running"))
			Eventually(argocd, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying that all expect notification controller k8s resources exist")
			notifMetricsService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-notifications-controller-metrics",
					Namespace: ns.Name,
				},
			}
			Eventually(notifMetricsService).Should(k8sFixture.ExistByName())

			notifDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-notifications-controller",
					Namespace: ns.Name,
				},
			}
			Eventually(notifDepl).Should(k8sFixture.ExistByName())

			Eventually(notifDepl).Should(deployment.HaveConditionTypeStatus(appsv1.DeploymentAvailable, corev1.ConditionTrue))
			Eventually(notifDepl).Should(deployment.HaveConditionTypeStatus(appsv1.DeploymentProgressing, corev1.ConditionTrue))

			notifSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-notifications-secret",
					Namespace: ns.Name,
				},
			}
			Eventually(notifSecret).Should(k8sFixture.ExistByName())

			notifCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-notifications-cm",
					Namespace: ns.Name,
				},
			}
			Eventually(notifCM).Should(k8sFixture.ExistByName())

			notifSA := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-argocd-notifications-controller",
					Namespace: ns.Name,
				},
			}
			Eventually(notifSA).Should(k8sFixture.ExistByName())

			notifRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-argocd-notifications-controller",
					Namespace: ns.Name,
				},
			}
			Eventually(notifRole).Should(k8sFixture.ExistByName())

			notifRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-argocd-notifications-controller",
					Namespace: ns.Name,
				},
			}
			Eventually(notifRoleBinding).Should(k8sFixture.ExistByName())

			By("disabling notification controller in ArgoCD CR")
			argocdFixture.Update(argocd, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Notifications = argov1beta1api.ArgoCDNotifications{
					Enabled: false,
				}
			})

			By("verifying notification controller k8s resources are deleted")
			Eventually(notifDepl, "3m", "5s").Should(k8sFixture.NotExistByName())
			Eventually(notifSecret).Should(k8sFixture.NotExistByName())
			Eventually(notifCM).Should(k8sFixture.NotExistByName())
			Eventually(notifSA).Should(k8sFixture.NotExistByName())
			Eventually(notifRole).Should(k8sFixture.NotExistByName())
			Eventually(notifRoleBinding).Should(k8sFixture.NotExistByName())

			By("verifying notification controller .status field of ArgoCD CR is back to empty")
			Eventually(argocd).Should(argocdFixture.HaveNotificationControllerStatus(""))

		})

		It("ensure that sourceNamespace resources are not created for namespace-scoped instance", func() {

			By("creating test namespaces")
			fooNs, fooNsCleanupFunc := fixture.CreateNamespaceWithCleanupFunc("foo-src-ns")
			defer fooNsCleanupFunc()

			By("creating namespace-scoped Argo CD instance with sourceNamespaces")
			ns, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("1-055-src-ns-test")
			defer cleanupFunc()

			argocd := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled:          true,
						SourceNamespaces: []string{fooNs.Name},
					},
					SourceNamespaces: []string{fooNs.Name},
				},
			}
			Expect(k8sClient.Create(ctx, argocd)).To(Succeed())

			By("verifying Argo CD and notification controller start as expected")
			Eventually(argocd, "4m", "5s").Should(argocdFixture.HaveNotificationControllerStatus("Running"))

			By("verifying that sourceNamespace cmd args don't exist")
			depl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-notifications-controller",
					Namespace: ns.Name,
				},
			}
			Eventually(depl).Should(k8sFixture.ExistByName())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(depl), depl)
				if err != nil {
					return false
				}
				if len(depl.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				cmd := depl.Spec.Template.Spec.Containers[0].Command
				cmdStr := strings.Join(cmd, " ")
				return !strings.Contains(cmdStr, "--application-namespaces")
			}, "2m", "5s").Should(BeTrue())

			By("verifying sourceNamespace rbac resources are not created")
			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-1-055-src-ns-test-argocd-notifications-controller",
					Namespace: ns.Name,
				},
			}
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-1-055-src-ns-test-argocd-notifications-controller",
					Namespace: ns.Name,
				},
			}
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-1-055-src-ns-test-notifications",
					Namespace: fooNs.Name,
				},
			}
			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-1-055-src-ns-test-notifications",
					Namespace: fooNs.Name,
				},
			}

			Eventually(role).Should(k8sFixture.NotExistByName())
			Eventually(roleBinding).Should(k8sFixture.NotExistByName())
			Eventually(clusterRole).Should(k8sFixture.NotExistByName())
			Eventually(clusterRoleBinding).Should(k8sFixture.NotExistByName())

		})

	})
})
