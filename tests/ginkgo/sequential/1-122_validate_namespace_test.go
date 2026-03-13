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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	argoutil "github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	namespaceFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	statefulsetFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-122_validate_namespace", func() {

		var (
			k8sClient client.Client
			ctx       context.Context

			cleanupArgoNamespace   func()
			cleanupSourceNamespace func()
			argoNamespace          *corev1.Namespace
			sourceNamespace        *corev1.Namespace
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			fixture.OutputDebugOnFail(argoNamespace, sourceNamespace)

			if cleanupArgoNamespace != nil {
				cleanupArgoNamespace()
			}
			if cleanupSourceNamespace != nil {
				cleanupSourceNamespace()
			}
		})

		It("Should validate namespace for new resources", func() {

			argoNamespace, cleanupArgoNamespace = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			sourceNamespace, cleanupSourceNamespace = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			By("creating ArgoCD CR with a single source namespace specified")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: argoNamespace.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceNamespaces: []string{sourceNamespace.Name},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying that there exist no clusterrolebindings that point to this namespace-scoped namespace")
			Consistently(func() bool {
				var clusterRoleBindings rbacv1.ClusterRoleBindingList
				if err := k8sClient.List(ctx, &clusterRoleBindings); err != nil {
					GinkgoWriter.Println(err)
					return false
				}
				for _, crb := range clusterRoleBindings.Items {
					for _, subject := range crb.Subjects {
						if subject.Namespace == argoCD.Namespace {
							GinkgoWriter.Println("detected a CRB that pointed to namespace scoped ArgoCD instance. This shouldn't happen:", crb.Name)
							return false
						}
					}
				}
				return true

			}).Should(BeTrue())

			By("verifying Role is not created in sourceNamespace, since the namespace is not cluster-scoped")
			Consistently(&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s_%s", "argocd", sourceNamespace.Name),
					Namespace: sourceNamespace.Name,
				},
			}, "30s", "5s").Should(k8sFixture.NotExistByName())

			By("verifying RoleBinding is not created")
			Consistently(&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoutil.TruncateWithHash(fmt.Sprintf("%s_%s", "argocd", sourceNamespace.Name), argoutil.GetMaxLabelLength()),
					Namespace: sourceNamespace.Name,
				},
			}, "30s", "5s").Should(k8sFixture.NotExistByName())

			By("verifying there exist no rolebindings in source namespace that point to argocd namespace")
			var roleBindingList rbacv1.RoleBindingList
			Expect(k8sClient.List(ctx, &roleBindingList, client.InNamespace(sourceNamespace.Name))).To(Succeed())
			for _, rb := range roleBindingList.Items {
				for _, subject := range rb.Subjects {
					if subject.Namespace == argoCD.Namespace {
						Fail("There should exist no rolebindings that point to our argocd namespace: " + rb.Name)
					}
				}
			}

			By("verifying namespace does not have label")
			Consistently(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: sourceNamespace.Name,
				},
			}, "30s", "3s").Should(k8sFixture.NotHaveLabelWithValue(common.ArgoCDManagedByClusterArgoCDLabel, argoNamespace.Name))

			By("verifying application namespaces is not set on server")
			serverDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-server",
					Namespace: argoNamespace.Name,
				},
			}
			Eventually(serverDeployment, "30s", "3s").Should(k8sFixture.ExistByName())
			Consistently(serverDeployment, "30s", "3s").ShouldNot(deploymentFixture.HaveContainerCommandSubstring("--application-namespaces", 0))

			appControllerStatefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-application-controller",
					Namespace: argoNamespace.Name,
				},
			}
			Eventually(appControllerStatefulSet).Should(k8sFixture.ExistByName())
			Consistently(appControllerStatefulSet, "30s", "3s").ShouldNot(statefulsetFixture.HaveContainerCommandSubstring("--application-namespaces", 0))

		})

		It("Should validate namespace for existing resources", func() {

			argoNamespace, cleanupArgoNamespace = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			sourceNamespace, cleanupSourceNamespace = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			By("creating role/rolebinding in source namespace")
			roleName := fmt.Sprintf("%s_%s", "argocd", sourceNamespace.Name)
			Expect(k8sClient.Create(ctx, &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: sourceNamespace.Name,
				},
				Rules: []rbacv1.PolicyRule{
					{
						Verbs:     []string{"get", "list"},
						APIGroups: []string{""},
						Resources: []string{"configmaps"},
					},
				},
			})).To(Succeed())

			roleBindingName := argoutil.TruncateWithHash(fmt.Sprintf("%s_%s", "argocd", sourceNamespace.Name), argoutil.GetMaxLabelLength())

			Expect(k8sClient.Create(ctx, &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleBindingName,
					Namespace: sourceNamespace.Name,
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "Role",
					Name:     roleName,
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      "argocd-application-controller",
						Namespace: argoNamespace.Name,
					},
				},
			})).To(Succeed())

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd",
					Namespace: argoNamespace.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceNamespaces: []string{sourceNamespace.Name},
				},
			}

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("adding label to source namespace")
			namespaceFixture.Update(sourceNamespace, func(ns *corev1.Namespace) {
				if ns.Labels == nil {
					ns.Labels = map[string]string{}
				}
				ns.Labels[common.ArgoCDManagedByClusterArgoCDLabel] = argoNamespace.Name
			})

			Eventually(&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleName,
					Namespace: sourceNamespace.Name,
				},
			}, "30s", "3s").Should(k8sFixture.NotExistByName())

			Eventually(&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      roleBindingName,
					Namespace: sourceNamespace.Name,
				},
			}, "30s", "3s").Should(k8sFixture.NotExistByName())

			Eventually(&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: sourceNamespace.Name,
				},
			}, "30s", "3s").Should(k8sFixture.NotHaveLabelWithValue(common.ArgoCDManagedByClusterArgoCDLabel, argoNamespace.Name))

			serverDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-server",
					Namespace: argoNamespace.Name,
				},
			}
			Eventually(serverDeployment, "30s", "3s").Should(k8sFixture.ExistByName())
			Consistently(serverDeployment, "30s", "3s").ShouldNot(deploymentFixture.HaveContainerCommandSubstring("--application-namespaces", 0))

			appControllerStatefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-application-controller",
					Namespace: argoNamespace.Name,
				},
			}
			Eventually(appControllerStatefulSet).Should(k8sFixture.ExistByName())
			Consistently(appControllerStatefulSet, "30s", "3s").ShouldNot(statefulsetFixture.HaveContainerCommandSubstring("--application-namespaces", 0))

		})
	})
})
