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
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	namespaceFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/rolebinding"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-024_validate_apps_in_any_namespace", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {

			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = utils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies source namespaces feature adds role/rolebindings and label to source namespaces", func() {

			By("Creating namespaces")

			centralArgoCDNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("central-argocd")
			defer cleanupFunc()

			test_1_24_customNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("test-1-24-custom")
			defer cleanupFunc()

			test_2_24_customNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("test-2-24-custom")
			defer cleanupFunc()

			longnsNS, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("longns-abcdefghijklmnopqrstuvwxyz-123456789012345")
			defer cleanupFunc()

			By("creating argocd instance in central-argocd, with the other namespaces as source namespaces")

			centralArgoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: centralArgoCDNS.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceNamespaces: []string{
						"test-1-24-custom",
						"test-2-24-custom",
						"longns-abcdefghijklmnopqrstuvwxyz-123456789012345",
					},
				},
			}
			Expect(k8sClient.Create(ctx, centralArgoCD)).To(Succeed())

			By("verifying other namespaces have managed-by-cluster-argocd role, and expected role/rolebindings")
			Eventually(test_1_24_customNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "central-argocd"))

			example_argocd_test_1_24_custom_Role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_test-1-24-custom",
					Namespace: test_1_24_customNS.Name,
				},
			}
			Eventually(example_argocd_test_1_24_custom_Role).Should(k8sFixture.ExistByName())

			example_argocd_test_1_24_custom_RoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_test-1-24-custom",
					Namespace: test_1_24_customNS.Name,
				},
			}
			Eventually(example_argocd_test_1_24_custom_RoleBinding).Should(k8sFixture.ExistByName())
			Eventually(example_argocd_test_1_24_custom_RoleBinding).Should(rolebinding.HaveRoleRef(rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "example-argocd_test-1-24-custom",
			}))
			Eventually(example_argocd_test_1_24_custom_RoleBinding).Should(rolebinding.HaveSubject(rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "example-argocd-argocd-server",
				Namespace: centralArgoCDNS.Name,
			}))
			Eventually(example_argocd_test_1_24_custom_RoleBinding).Should(rolebinding.HaveSubject(rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "example-argocd-argocd-application-controller",
				Namespace: centralArgoCDNS.Name,
			}))

			Eventually(test_2_24_customNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "central-argocd"))

			example_argocd_test_2_24_custom_Role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_test-2-24-custom",
					Namespace: test_2_24_customNS.Name,
				},
			}
			Eventually(example_argocd_test_2_24_custom_Role).Should(k8sFixture.ExistByName())

			example_argocd_test_2_24_custom_RoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_test-2-24-custom",
					Namespace: test_2_24_customNS.Name,
				},
			}
			Eventually(example_argocd_test_2_24_custom_RoleBinding).Should(k8sFixture.ExistByName())
			Eventually(example_argocd_test_2_24_custom_RoleBinding).Should(rolebinding.HaveRoleRef(rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "example-argocd_test-2-24-custom",
			}))
			Eventually(example_argocd_test_2_24_custom_RoleBinding).Should(rolebinding.HaveSubject(rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "example-argocd-argocd-server",
				Namespace: centralArgoCDNS.Name,
			}))
			Eventually(example_argocd_test_2_24_custom_RoleBinding).Should(rolebinding.HaveSubject(rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "example-argocd-argocd-application-controller",
				Namespace: centralArgoCDNS.Name,
			}))

			By("verifying RoleBindings are created for long namespace")

			longNSRoleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd_longns-abcdefghijklmnopqrstuvwxyz-123456-9a19a95",
					Namespace: longnsNS.Name,
				},
			}
			Eventually(longNSRoleBinding).Should(k8sFixture.ExistByName())

			By("verifying RoleBinding name is exactly 63 characters")
			Expect(longNSRoleBinding.Name).To(HaveLen(63))

			By("verifying that the name contains the expected hash suffix")
			Expect(longNSRoleBinding.Name).To(ContainSubstring("-9a19a95"))

			By("Updating ArgoCD source namespaces to only test-2-24-custom")
			argocdFixture.Update(centralArgoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.SourceNamespaces = []string{
					"test-2-24-custom",
				}
			})

			By("verifying 1-24-custom no longer has managed-by-cluster namespace, so it is no longer a source namespace")
			Eventually(test_1_24_customNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "central-argocd"))
			Eventually(example_argocd_test_1_24_custom_Role).ShouldNot(k8sFixture.ExistByName())
			Eventually(example_argocd_test_1_24_custom_RoleBinding).ShouldNot(k8sFixture.ExistByName())

			By("deleting Argo CD instance, and verifying label and resources are removed")

			Expect(k8sClient.Delete(ctx, centralArgoCD)).To(Succeed())

			Eventually(test_2_24_customNS).ShouldNot(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", "central-argocd"))

			Eventually(example_argocd_test_2_24_custom_Role).Should(k8sFixture.NotExistByName())
			Eventually(example_argocd_test_2_24_custom_RoleBinding).Should(k8sFixture.NotExistByName())

		})

	})
})
