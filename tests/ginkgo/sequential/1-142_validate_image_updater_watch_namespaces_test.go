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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	argoutil "github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deplFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-142_validate_image_updater_watch_namespaces", func() {

		const argocdName = "example-argocd"

		var (
			k8sClient        client.Client
			ctx              context.Context
			argoNamespace    *corev1.Namespace
			argoCD           *argov1beta1api.ArgoCD
			cleanupFunctions []func()
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
			cleanupFunctions = []func(){}
		})

		AfterEach(func() {
			fixture.OutputDebugOnFail(argoNamespace)

			if argoCD != nil {
				err := k8sClient.Delete(ctx, argoCD)
				if err != nil && !apierrors.IsNotFound(err) {
					Expect(err).ToNot(HaveOccurred())
				}
			}

			for _, f := range cleanupFunctions {
				f()
			}
		})

		// imageUpdaterRoleName returns the expected Role name created by the operator
		// in a watch namespace for the Image Updater.
		imageUpdaterRoleName := func(ns string) string {
			return fmt.Sprintf("%s_%s", argocdName, ns)
		}

		// imageUpdaterRoleBindingName returns the expected RoleBinding name (possibly truncated).
		imageUpdaterRoleBindingName := func(ns string) string {
			return argoutil.TruncateWithHash(fmt.Sprintf("%s_%s", argocdName, ns), argoutil.GetMaxLabelLength())
		}

		// expectRBACExists asserts that the operator has created a Role and RoleBinding in ns.
		expectRBACExists := func(ns string) {
			GinkgoHelper()
			By("verifying Role exists in " + ns)
			Eventually(&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: imageUpdaterRoleName(ns), Namespace: ns},
			}, "2m", "5s").Should(k8sFixture.ExistByName())

			By("verifying RoleBinding exists in " + ns)
			Eventually(&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: imageUpdaterRoleBindingName(ns), Namespace: ns},
			}, "2m", "5s").Should(k8sFixture.ExistByName())
		}

		// expectRBACAbsent asserts that no Image Updater Role or RoleBinding is present in ns.
		expectRBACAbsent := func(ns string) {
			GinkgoHelper()
			By("verifying Role is absent in " + ns)
			Consistently(&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: imageUpdaterRoleName(ns), Namespace: ns},
			}, "15s", "3s").Should(k8sFixture.NotExistByName())

			By("verifying RoleBinding is absent in " + ns)
			Consistently(&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: imageUpdaterRoleBindingName(ns), Namespace: ns},
			}, "15s", "3s").Should(k8sFixture.NotExistByName())
		}

		// expectRBACPruned asserts that a previously existing Role and RoleBinding are eventually deleted.
		expectRBACPruned := func(ns string) {
			GinkgoHelper()
			By("verifying Role is pruned from " + ns)
			Eventually(&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: imageUpdaterRoleName(ns), Namespace: ns},
			}, "2m", "5s").Should(k8sFixture.NotExistByName())

			By("verifying RoleBinding is pruned from " + ns)
			Eventually(&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: imageUpdaterRoleBindingName(ns), Namespace: ns},
			}, "2m", "5s").Should(k8sFixture.NotExistByName())
		}

		It("verifies that creating a new namespace matching IMAGE_UPDATER_WATCH_NAMESPACES triggers automatic RBAC creation without manual operator interaction", func() {

			By("creating a cluster-scoped namespace for the Argo CD instance")
			argoNamespace, _ = fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-iuw-new-ns")
			cleanupFunctions = append(cleanupFunctions, func() { fixture.DeleteNamespace(argoNamespace) })

			By("creating an initial matching namespace app-dyn-1 before ArgoCD is configured")
			appNs1, cleanup := fixture.CreateNamespaceWithCleanupFunc("app-dyn-1")
			cleanupFunctions = append(cleanupFunctions, cleanup)

			By("creating ArgoCD CR with Image Updater enabled and IMAGE_UPDATER_WATCH_NAMESPACES=app-dyn-* (glob)")
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argocdName,
					Namespace: argoNamespace.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					ImageUpdater: argov1beta1api.ArgoCDImageUpdaterSpec{
						Enabled: true,
						Env: []corev1.EnvVar{
							{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "app-dyn-*"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD instance to become available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying initial RBAC is created in app-dyn-1 (pre-existing namespace matched by the pattern)")
			expectRBACExists(appNs1.Name)

			By("verifying IMAGE_UPDATER_WATCH_NAMESPACES in the deployment is set to app-dyn-1")
			imageUpdaterDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-argocd-image-updater-controller", argocdName),
					Namespace: argoNamespace.Name,
				},
			}
			Eventually(imageUpdaterDeployment, "2m", "5s").Should(
				deplFixture.HaveContainerWithEnvVar("IMAGE_UPDATER_WATCH_NAMESPACES", "app-dyn-1", 0),
			)

			By("creating a new namespace app-dyn-2 AFTER the operator is already running")
			appNs2, cleanup := fixture.CreateNamespaceWithCleanupFunc("app-dyn-2")
			cleanupFunctions = append(cleanupFunctions, cleanup)

			By("verifying the operator auto-reconciles and creates RBAC in app-dyn-2 (triggered by imageUpdaterWatchNSMapper)")
			expectRBACExists(appNs2.Name)

			By("verifying IMAGE_UPDATER_WATCH_NAMESPACES in the deployment is expanded to include app-dyn-2")
			Eventually(imageUpdaterDeployment, "2m", "5s").Should(
				deplFixture.HaveContainerWithEnvVar("IMAGE_UPDATER_WATCH_NAMESPACES", "app-dyn-1,app-dyn-2", 0),
			)

			By("creating another non-matching namespace to confirm it does not affect RBAC or deployment env var")
			_, cleanup = fixture.CreateNamespaceWithCleanupFunc("other-dyn-ns")
			cleanupFunctions = append(cleanupFunctions, cleanup)

			By("verifying RBAC is absent in other-dyn-ns (no match)")
			expectRBACAbsent("other-dyn-ns")

			By("verifying deployment env var is still app-dyn-1,app-dyn-2 after unmatched namespace creation")
			Eventually(imageUpdaterDeployment, "1m", "5s").Should(
				deplFixture.HaveContainerWithEnvVar("IMAGE_UPDATER_WATCH_NAMESPACES", "app-dyn-1,app-dyn-2", 0),
			)
		})

		It("verifies glob and regex patterns in IMAGE_UPDATER_WATCH_NAMESPACES create RBAC only in matching namespaces and prune stale entries", func() {

			By("creating a cluster-scoped namespace for the Argo CD instance")
			argoNamespace, _ = fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-iuw-ns")
			cleanupFunctions = append(cleanupFunctions, func() { fixture.DeleteNamespace(argoNamespace) })

			By("creating target namespaces: app-ns-1 and app-ns-2 (match app-ns-*), other-ns (no match)")
			appNs1, cleanup := fixture.CreateNamespaceWithCleanupFunc("app-ns-1")
			cleanupFunctions = append(cleanupFunctions, cleanup)

			appNs2, cleanup := fixture.CreateNamespaceWithCleanupFunc("app-ns-2")
			cleanupFunctions = append(cleanupFunctions, cleanup)

			_, cleanup = fixture.CreateNamespaceWithCleanupFunc("other-ns")
			cleanupFunctions = append(cleanupFunctions, cleanup)

			By("creating ArgoCD CR with Image Updater enabled and IMAGE_UPDATER_WATCH_NAMESPACES=app-ns-* (glob)")
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argocdName,
					Namespace: argoNamespace.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					ImageUpdater: argov1beta1api.ArgoCDImageUpdaterSpec{
						Enabled: true,
						Env: []corev1.EnvVar{
							{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "app-ns-*"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD instance to become available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying RBAC is created in app-ns-1 and app-ns-2 (glob matches)")
			expectRBACExists(appNs1.Name)
			expectRBACExists(appNs2.Name)

			By("verifying RBAC is NOT created in other-ns (pattern does not match)")
			expectRBACAbsent("other-ns")

			By("verifying IMAGE_UPDATER_WATCH_NAMESPACES env var in the deployment is expanded to concrete namespaces")
			imageUpdaterDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					// Deployment name: {argocd-name}-argocd-image-updater-controller
					Name:      fmt.Sprintf("%s-argocd-image-updater-controller", argocdName),
					Namespace: argoNamespace.Name,
				},
			}
			// resolveImageUpdaterWatchNamespaces sorts within each pattern's matches, so
			// app-ns-1 comes before app-ns-2.
			Eventually(imageUpdaterDeployment, "2m", "5s").Should(
				deplFixture.HaveContainerWithEnvVar("IMAGE_UPDATER_WATCH_NAMESPACES", "app-ns-1,app-ns-2", 0),
			)

			By("updating IMAGE_UPDATER_WATCH_NAMESPACES to a regex that only matches app-ns-1")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ImageUpdater.Env = []corev1.EnvVar{
					{Name: "IMAGE_UPDATER_WATCH_NAMESPACES", Value: "/^app-ns-1$/"},
				}
			})

			By("verifying RBAC for app-ns-1 is retained (regex still matches)")
			expectRBACExists(appNs1.Name)

			By("verifying RBAC for app-ns-2 is pruned (no longer matched by any pattern)")
			expectRBACPruned(appNs2.Name)

			By("verifying IMAGE_UPDATER_WATCH_NAMESPACES in the deployment is updated to only app-ns-1")
			Eventually(imageUpdaterDeployment, "2m", "5s").Should(
				deplFixture.HaveContainerWithEnvVar("IMAGE_UPDATER_WATCH_NAMESPACES", "app-ns-1", 0),
			)
		})
	})
})
