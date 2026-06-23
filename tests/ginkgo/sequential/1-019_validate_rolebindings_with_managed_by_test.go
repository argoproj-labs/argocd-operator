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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/rolebinding"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-019_validate_rolebindings_with_managed_by", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensuring that when an Argo CD instance is created in a namespace that is managed by another Argo CD instance, that the expected roles/rolebindings are created in both namespaces", func() {

			By("creating a namespace for containing central Argo CD")
			centralArgoCD_NS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("central-argocd")
			defer cleanupFunc()

			By("creating a namespace that is managed by the above ArgoCD instance")
			argocd1_NS, cleanupFunc := fixture.CreateManagedNamespaceWithCleanupFunc("argocd-1", centralArgoCD_NS.Name)
			defer cleanupFunc()

			By("creating a namespace in central-argocd")
			argoCD_central := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: centralArgoCD_NS.Name, Labels: map[string]string{"example": "basic"}},
				Spec: argov1beta1api.ArgoCDSpec{
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeDex,
						Dex: &argov1beta1api.ArgoCDDexSpec{
							Config: "test-config",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD_central)).To(Succeed())

			By("verifying roles/rolebindings exist in central Argo CD ns")
			appControllerRole_central := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-argocd-application-controller", Namespace: centralArgoCD_NS.Name},
			}
			Eventually(appControllerRole_central).Should(k8sFixture.ExistByName())

			serverRole_central := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-argocd-server", Namespace: centralArgoCD_NS.Name},
			}
			Eventually(serverRole_central).Should(k8sFixture.ExistByName())

			appControllerRoleBinding_central := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-argocd-application-controller",
					Namespace: centralArgoCD_NS.Name,
				},
			}
			Eventually(appControllerRoleBinding_central).Should(k8sFixture.ExistByName())
			Eventually(appControllerRoleBinding_central).Should(rolebinding.HaveRoleRef(rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "example-argocd-argocd-application-controller",
			}))

			serverRoleBinding_central := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-argocd-server",
					Namespace: "central-argocd",
				},
			}
			Eventually(serverRoleBinding_central).Should(k8sFixture.ExistByName())
			Eventually(serverRoleBinding_central).Should(rolebinding.HaveRoleRef(rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "example-argocd-argocd-server",
			}))

			By("creating new Argo CD instance in namespace that is managed by another Argo CD instance")
			argoCD1 := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "child-argocd", Namespace: argocd1_NS.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeDex,
						Dex: &argov1beta1api.ArgoCDDexSpec{
							Config: "test-config",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD1)).To(Succeed())

			By("verifying the central Argo CD roles are still as expected")
			appControllerRole_central = &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-argocd-application-controller", Namespace: centralArgoCD_NS.Name},
			}
			Eventually(appControllerRole_central).Should(k8sFixture.ExistByName())

			serverRole_central = &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-argocd-server", Namespace: centralArgoCD_NS.Name},
			}
			Eventually(serverRole_central).Should(k8sFixture.ExistByName())

			appControllerRoleBinding_central = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-argocd-application-controller",
					Namespace: centralArgoCD_NS.Name,
				},
			}
			Eventually(appControllerRoleBinding_central).Should(k8sFixture.ExistByName())
			Eventually(appControllerRoleBinding_central).Should(rolebinding.HaveRoleRef(rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "example-argocd-argocd-application-controller",
			}))

			serverRoleBinding_central = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-argocd-server",
					Namespace: "central-argocd",
				},
			}
			Eventually(serverRoleBinding_central).Should(k8sFixture.ExistByName())
			Eventually(serverRoleBinding_central).Should(rolebinding.HaveRoleRef(rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "example-argocd-argocd-server",
			}))

			By("verifying child roles and role bindings are as expected")

			rolesToCheck := []string{"child-argocd-argocd-application-controller", "child-argocd-argocd-server", "child-argocd-argocd-dex-server", "child-argocd-argocd-redis-ha"}

			for _, roleToCheck := range rolesToCheck {
				role := &rbacv1.Role{
					ObjectMeta: metav1.ObjectMeta{Name: roleToCheck, Namespace: argocd1_NS.Name},
				}
				Eventually(role).Should(k8sFixture.ExistByName())

			}

			roleBindingsToCheck := []string{"child-argocd-argocd-application-controller", "child-argocd-argocd-server", "child-argocd-argocd-dex-server", "child-argocd-argocd-redis-ha"}

			for _, roleBindingToCheck := range roleBindingsToCheck {
				roleBinding := &rbacv1.RoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:      roleBindingToCheck,
						Namespace: argocd1_NS.Name,
					},
				}
				Eventually(roleBinding).Should(k8sFixture.ExistByName())
				Eventually(roleBinding).Should(rolebinding.HaveRoleRef(rbacv1.RoleRef{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "Role",
					Name:     roleBindingToCheck,
				}))

			}

		})

	})
})
