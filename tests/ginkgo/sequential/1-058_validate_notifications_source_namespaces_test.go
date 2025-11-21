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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1api "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	namespaceFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-058_validate_notifications_source_namespaces", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			fixture.OutputDebugOnFail("not-argocd-ns")
		})

		It("ensures that NotificationsConfiguration, Role, and RoleBinding are created in source namespaces when  notifications.sourceNamespaces is configured", func() {

			By("creating Argo CD instance namespace")
			argocdNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-cluster-config")
			defer cleanupFunc()

			By("creating source namespaces")
			sourceNS1, cleanupFunc1 := fixture.CreateNamespaceWithCleanupFunc("notif-source-ns-1")
			defer cleanupFunc1()

			sourceNS2, cleanupFunc2 := fixture.CreateNamespaceWithCleanupFunc("notif-source-ns-2")
			defer cleanupFunc2()

			By("creating Argo CD instance with notifications enabled and sourceNamespaces configured")
			argocd := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: argocdNS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceNamespaces: []string{sourceNS1.Name, sourceNS2.Name},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled:          true,
						SourceNamespaces: []string{sourceNS1.Name, sourceNS2.Name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argocd)).To(Succeed())

			By("waiting for Argo CD to be available")
			Eventually(argocd, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying notification controller is running")
			Eventually(argocd, "4m", "5s").Should(argocdFixture.HaveNotificationControllerStatus("Running"))

			By("verifying NotificationsConfiguration CR is created in source namespace 1")
			notifCfg1 := &argov1alpha1api.NotificationsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-notifications-configuration",
					Namespace: sourceNS1.Name,
				},
			}
			Eventually(notifCfg1).Should(k8sFixture.ExistByName())

			By("verifying NotificationsConfiguration CR is created in source namespace 2")
			notifCfg2 := &argov1alpha1api.NotificationsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-notifications-configuration",
					Namespace: sourceNS2.Name,
				},
			}
			Eventually(notifCfg2).Should(k8sFixture.ExistByName())

			By("verifying Role is created in source namespace 1")
			roleName1 := "example-argocd-" + argocdNS.Name + "-notifications"
			role1 := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName1,
					Namespace: sourceNS1.Name,
				},
			}
			Eventually(role1).Should(k8sFixture.ExistByName())

			By("verifying RoleBinding is created in source namespace 1")
			roleBinding1 := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName1,
					Namespace: sourceNS1.Name,
				},
			}
			Eventually(roleBinding1).Should(k8sFixture.ExistByName())

			By("verifying namespace 1 has the notifications-managed-by-cluster-argocd label")
			Eventually(sourceNS1).Should(namespaceFixture.HaveLabel(common.ArgoCDNotificationsManagedByClusterArgoCDLabel, argocdNS.Name))

			By("verifying namespace 2 has the notifications-managed-by-cluster-argocd label")
			Eventually(sourceNS2).Should(namespaceFixture.HaveLabel(common.ArgoCDNotificationsManagedByClusterArgoCDLabel, argocdNS.Name))

			By("verifying notifications controller deployment has --application-namespaces and --self-service-notification-enabled flags")
			notifDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-notifications-controller",
					Namespace: argocdNS.Name,
				},
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notifDepl), notifDepl)
				if err != nil {
					return false
				}
				if len(notifDepl.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				cmd := notifDepl.Spec.Template.Spec.Containers[0].Command
				cmdStr := strings.Join(cmd, " ")
				hasAppNamespaces := strings.Contains(cmdStr, "--application-namespaces")
				hasSelfService := strings.Contains(cmdStr, "--self-service-notification-enabled")
				hasBothNamespaces := strings.Contains(cmdStr, sourceNS1.Name) && strings.Contains(cmdStr, sourceNS2.Name)
				return hasAppNamespaces && hasSelfService && hasBothNamespaces
			}, "2m", "5s").Should(BeTrue())

			By("verifying ClusterRole is created for notifications controller")
			notifClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-argocd-" + argocdNS.Name + "-argocd-notifications-controller",
				},
			}
			Eventually(notifClusterRole).Should(k8sFixture.ExistByName())

			By("verifying ClusterRoleBinding is created for notifications controller")
			notifClusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-argocd-" + argocdNS.Name + "-argocd-notifications-controller",
				},
			}
			Eventually(notifClusterRoleBinding).Should(k8sFixture.ExistByName())

			By("verifying ClusterRoleBinding references the correct ClusterRole and ServiceAccount")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notifClusterRoleBinding), notifClusterRoleBinding)
				if err != nil {
					return false
				}
				expectedRoleRef := rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "ClusterRole",
					Name:     notifClusterRole.Name,
				}
				expectedSubject := rbacv1.Subject{
					Kind:      "ServiceAccount",
					Name:      "example-argocd-argocd-notifications-controller",
					Namespace: argocdNS.Name,
				}
				return notifClusterRoleBinding.RoleRef == expectedRoleRef &&
					len(notifClusterRoleBinding.Subjects) == 1 &&
					notifClusterRoleBinding.Subjects[0] == expectedSubject
			}, "2m", "5s").Should(BeTrue())

		})

		It("ensures that resources are not created when namespace is not in SourceNamespaces", func() {

			By("creating Argo CD instance namespace")
			argocdNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-cluster-config")
			defer cleanupFunc()

			By("creating source namespaces")
			sourceNS1, cleanupFunc1 := fixture.CreateNamespaceWithCleanupFunc("notif-source-ns-3")
			defer cleanupFunc1()

			unmanagedNS, cleanupFunc2 := fixture.CreateNamespaceWithCleanupFunc("notif-unmanaged-ns")
			defer cleanupFunc2()

			By("creating Argo CD instance with notifications enabled but only sourceNS1 in both SourceNamespaces and Notifications.SourceNamespaces")
			argocd := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: argocdNS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceNamespaces: []string{sourceNS1.Name},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled:          true,
						SourceNamespaces: []string{sourceNS1.Name, unmanagedNS.Name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argocd)).To(Succeed())

			By("waiting for Argo CD to be available")
			Eventually(argocd, "5m", "5s").Should(argocdFixture.BeAvailable())

			fixture.OutputDebugOnFail(argocdNS.Name)

			By("verifying NotificationsConfiguration CR is created in sourceNS1")
			notifCfg1 := &argov1alpha1api.NotificationsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-notifications-configuration",
					Namespace: sourceNS1.Name,
				},
			}
			Eventually(notifCfg1).Should(k8sFixture.ExistByName())

			By("verifying NotificationsConfiguration CR is NOT created in unmanagedNS")
			notifCfgUnmanaged := &argov1alpha1api.NotificationsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-notifications-configuration",
					Namespace: unmanagedNS.Name,
				},
			}
			Consistently(notifCfgUnmanaged).Should(k8sFixture.NotExistByName())

			By("verifying Role is NOT created in unmanagedNS")
			roleName := "example-argocd-" + argocdNS.Name + "-notifications"
			roleUnmanaged := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: unmanagedNS.Name,
				},
			}
			Consistently(roleUnmanaged).Should(k8sFixture.NotExistByName())

			By("verifying unmanagedNS does not have the notifications-managed-by-cluster-argocd label")
			Consistently(unmanagedNS).ShouldNot(namespaceFixture.HaveLabel(common.ArgoCDNotificationsManagedByClusterArgoCDLabel, argocdNS.Name))

			By("verifying notifications controller deployment command only includes sourceNS1")
			notifDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-notifications-controller",
					Namespace: argocdNS.Name,
				},
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notifDepl), notifDepl)
				if err != nil {
					return false
				}
				if len(notifDepl.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				cmd := notifDepl.Spec.Template.Spec.Containers[0].Command
				cmdStr := strings.Join(cmd, " ")
				hasSourceNS1 := strings.Contains(cmdStr, sourceNS1.Name)
				hasUnmanagedNS := strings.Contains(cmdStr, unmanagedNS.Name)
				return hasSourceNS1 && !hasUnmanagedNS
			}, "2m", "5s").Should(BeTrue())

		})

		It("ensures that resources are cleaned up when sourceNamespaces are removed", func() {

			By("creating Argo CD instance namespace")
			argocdNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-cluster-config")
			defer cleanupFunc()

			By("creating source namespaces")
			sourceNS1, cleanupFunc1 := fixture.CreateNamespaceWithCleanupFunc("notif-source-ns-4")
			defer cleanupFunc1()

			sourceNS2, cleanupFunc2 := fixture.CreateNamespaceWithCleanupFunc("notif-source-ns-5")
			defer cleanupFunc2()

			By("creating Argo CD instance with notifications enabled and both namespaces configured")
			argocd := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: argocdNS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceNamespaces: []string{sourceNS1.Name, sourceNS2.Name},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled:          true,
						SourceNamespaces: []string{sourceNS1.Name, sourceNS2.Name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argocd)).To(Succeed())

			By("waiting for Argo CD to be available")
			Eventually(argocd, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying resources are created in both namespaces")
			roleName := "example-argocd-" + argocdNS.Name + "-notifications"
			notifCfg1 := &argov1alpha1api.NotificationsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-notifications-configuration",
					Namespace: sourceNS1.Name,
				},
			}
			Eventually(notifCfg1).Should(k8sFixture.ExistByName())

			notifCfg2 := &argov1alpha1api.NotificationsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-notifications-configuration",
					Namespace: sourceNS2.Name,
				},
			}
			Eventually(notifCfg2).Should(k8sFixture.ExistByName())

			role1 := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: sourceNS1.Name,
				},
			}
			Eventually(role1).Should(k8sFixture.ExistByName())

			role2 := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: sourceNS2.Name,
				},
			}
			Eventually(role2).Should(k8sFixture.ExistByName())

			By("removing sourceNS1 from Notifications.SourceNamespaces")
			argocdFixture.Update(argocd, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Notifications.SourceNamespaces = []string{sourceNS2.Name}
			})

			By("waiting for Argo CD to reconcile")
			Eventually(argocd, "2m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying resources are removed from sourceNS1")
			Eventually(notifCfg1, "3m", "5s").Should(k8sFixture.NotExistByName())
			Eventually(role1, "3m", "5s").Should(k8sFixture.NotExistByName())

			roleBinding1 := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: sourceNS1.Name,
				},
			}
			Eventually(roleBinding1, "3m", "5s").Should(k8sFixture.NotExistByName())

			By("verifying sourceNS1 no longer has the notifications-managed-by-cluster-argocd label")
			Eventually(sourceNS1, "2m", "5s").ShouldNot(namespaceFixture.HaveLabel(common.ArgoCDNotificationsManagedByClusterArgoCDLabel, argocdNS.Name))

			By("verifying resources still exist in sourceNS2")
			Consistently(notifCfg2).Should(k8sFixture.ExistByName())
			Consistently(role2).Should(k8sFixture.ExistByName())

			roleBinding2 := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: sourceNS2.Name,
				},
			}
			Consistently(roleBinding2).Should(k8sFixture.ExistByName())

			By("verifying sourceNS2 still has the notifications-managed-by-cluster-argocd label")
			Consistently(sourceNS2).Should(namespaceFixture.HaveLabel(common.ArgoCDNotificationsManagedByClusterArgoCDLabel, argocdNS.Name))

		})

		It("ensures that resources are not created when notifications are disabled", func() {

			By("creating Argo CD instance namespace")
			argocdNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-cluster-config")
			defer cleanupFunc()

			By("creating source namespace")
			sourceNS1, cleanupFunc1 := fixture.CreateNamespaceWithCleanupFunc("notif-source-ns-6")
			defer cleanupFunc1()

			By("creating Argo CD instance with notifications disabled but sourceNamespaces configured")
			argocd := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: argocdNS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceNamespaces: []string{sourceNS1.Name},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled:          false,
						SourceNamespaces: []string{sourceNS1.Name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argocd)).To(Succeed())

			By("waiting for Argo CD to be available")
			Eventually(argocd, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying NotificationsConfiguration CR is NOT created in source namespace")
			notifCfg := &argov1alpha1api.NotificationsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-notifications-configuration",
					Namespace: sourceNS1.Name,
				},
			}
			Consistently(notifCfg).Should(k8sFixture.NotExistByName())

			By("verifying Role is NOT created in source namespace")
			roleName := "example-argocd-" + argocdNS.Name + "-notifications"
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: sourceNS1.Name,
				},
			}
			Consistently(role).Should(k8sFixture.NotExistByName())

			By("verifying ClusterRole is NOT created for notifications controller")
			notifClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-argocd-" + argocdNS.Name + "-argocd-notifications-controller",
				},
			}
			Consistently(notifClusterRole).Should(k8sFixture.NotExistByName())

			By("verifying ClusterRoleBinding is NOT created for notifications controller")
			notifClusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-argocd-" + argocdNS.Name + "-argocd-notifications-controller",
				},
			}
			Consistently(notifClusterRoleBinding).Should(k8sFixture.NotExistByName())

			By("verifying source namespace does not have the notifications-managed-by-cluster-argocd label")
			Consistently(sourceNS1).ShouldNot(namespaceFixture.HaveLabel(common.ArgoCDNotificationsManagedByClusterArgoCDLabel, argocdNS.Name))

		})

		It("ensures that notifications controller deployment command is updated when sourceNamespaces change", func() {

			By("creating Argo CD instance namespace")
			argocdNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-cluster-config")
			defer cleanupFunc()

			By("creating source namespaces")
			sourceNS1, cleanupFunc1 := fixture.CreateNamespaceWithCleanupFunc("notif-source-ns-7")
			defer cleanupFunc1()

			sourceNS2, cleanupFunc2 := fixture.CreateNamespaceWithCleanupFunc("notif-source-ns-8")
			defer cleanupFunc2()

			By("creating Argo CD instance with notifications enabled and only sourceNS1 configured")
			argocd := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: argocdNS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceNamespaces: []string{sourceNS1.Name, sourceNS2.Name},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled:          true,
						SourceNamespaces: []string{sourceNS1.Name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argocd)).To(Succeed())

			By("waiting for Argo CD to be available")
			Eventually(argocd, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying notifications controller deployment command includes only sourceNS1")
			notifDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-notifications-controller",
					Namespace: argocdNS.Name,
				},
			}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notifDepl), notifDepl)
				if err != nil {
					return false
				}
				if len(notifDepl.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				cmd := notifDepl.Spec.Template.Spec.Containers[0].Command
				cmdStr := strings.Join(cmd, " ")
				hasSourceNS1 := strings.Contains(cmdStr, sourceNS1.Name)
				hasSourceNS2 := strings.Contains(cmdStr, sourceNS2.Name)
				return hasSourceNS1 && !hasSourceNS2
			}, "2m", "5s").Should(BeTrue())

			By("adding sourceNS2 to Notifications.SourceNamespaces")
			argocdFixture.Update(argocd, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Notifications.SourceNamespaces = []string{sourceNS1.Name, sourceNS2.Name}
			})

			By("waiting for Argo CD to reconcile")
			Eventually(argocd, "2m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying notifications controller deployment command now includes both namespaces")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notifDepl), notifDepl)
				if err != nil {
					return false
				}
				if len(notifDepl.Spec.Template.Spec.Containers) == 0 {
					return false
				}
				cmd := notifDepl.Spec.Template.Spec.Containers[0].Command
				cmdStr := strings.Join(cmd, " ")
				hasSourceNS1 := strings.Contains(cmdStr, sourceNS1.Name)
				hasSourceNS2 := strings.Contains(cmdStr, sourceNS2.Name)
				hasSelfService := strings.Contains(cmdStr, "--self-service-notification-enabled")
				return hasSourceNS1 && hasSourceNS2 && hasSelfService
			}, "2m", "5s").Should(BeTrue())

		})

		It("ensures that resources are created when notifications are enabled after being disabled", func() {

			By("creating Argo CD instance namespace")
			argocdNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-cluster-config")
			defer cleanupFunc()

			By("creating source namespace")
			sourceNS1, cleanupFunc1 := fixture.CreateNamespaceWithCleanupFunc("notif-source-ns-9")
			defer cleanupFunc1()

			By("creating Argo CD instance with notifications disabled")
			argocd := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: argocdNS.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceNamespaces: []string{sourceNS1.Name},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled:          false,
						SourceNamespaces: []string{sourceNS1.Name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argocd)).To(Succeed())

			By("waiting for Argo CD to be available")
			Eventually(argocd, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying resources are NOT created")
			notifCfg := &argov1alpha1api.NotificationsConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default-notifications-configuration",
					Namespace: sourceNS1.Name,
				},
			}
			Consistently(notifCfg).Should(k8sFixture.NotExistByName())

			By("enabling notifications")
			argocdFixture.Update(argocd, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Notifications.Enabled = true
			})

			By("waiting for Argo CD to reconcile")
			Eventually(argocd, "2m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argocd, "4m", "5s").Should(argocdFixture.HaveNotificationControllerStatus("Running"))

			By("verifying resources are now created")
			Eventually(notifCfg, "3m", "5s").Should(k8sFixture.ExistByName())

			roleName := "example-argocd-" + argocdNS.Name + "-notifications"
			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: sourceNS1.Name,
				},
			}
			Eventually(role, "3m", "5s").Should(k8sFixture.ExistByName())

			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: sourceNS1.Name,
				},
			}
			Eventually(roleBinding, "3m", "5s").Should(k8sFixture.ExistByName())

			By("verifying source namespace has the notifications-managed-by-cluster-argocd label")
			Eventually(sourceNS1, "2m", "5s").Should(namespaceFixture.HaveLabel(common.ArgoCDNotificationsManagedByClusterArgoCDLabel, argocdNS.Name))

			By("verifying ClusterRole is created for notifications controller")
			notifClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-argocd-" + argocdNS.Name + "-argocd-notifications-controller",
				},
			}
			Eventually(notifClusterRole, "3m", "5s").Should(k8sFixture.ExistByName())

			By("verifying ClusterRoleBinding is created for notifications controller")
			notifClusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "example-argocd-" + argocdNS.Name + "-argocd-notifications-controller",
				},
			}
			Eventually(notifClusterRoleBinding, "3m", "5s").Should(k8sFixture.ExistByName())

		})

	})

})
