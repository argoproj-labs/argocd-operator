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
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-122_validate_namespace", func() {

		var (
			k8sClient client.Client
			ctx       context.Context

			cleanupArgoNamespace   func()
			cleanupSourceNamespace func()
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			if cleanupArgoNamespace != nil {
				cleanupArgoNamespace()
			}
			if cleanupSourceNamespace != nil {
				cleanupSourceNamespace()
			}
		})

		It("Should validate namespace for new resources", func() {
			var argoNamespace *corev1.Namespace
			var sourceNamespace *corev1.Namespace

			argoNamespace, cleanupArgoNamespace = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			sourceNamespace, cleanupSourceNamespace = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			fixture.OutputDebugOnFail(argoNamespace, sourceNamespace)

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
			Eventually(argoCD, "240s", "5s").Should(argocdFixture.BeAvailable())

			Consistently(&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s_%s", "argocd", sourceNamespace.Name),
					Namespace: sourceNamespace.Name,
				},
			}, "30s", "3s").Should(k8sFixture.NotExistByName())

			Consistently(&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoutil.TruncateWithHash(fmt.Sprintf("%s_%s", "argocd", sourceNamespace.Name), argoutil.GetMaxLabelLength()),
					Namespace: sourceNamespace.Name,
				},
			}, "30s", "3s").Should(k8sFixture.NotExistByName())

			Consistently(&corev1.Namespace{
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
		})

		It("Should validate namespace for existing resources", func() {
			var argoNamespace *corev1.Namespace
			var sourceNamespace *corev1.Namespace

			argoNamespace, cleanupArgoNamespace = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			sourceNamespace, cleanupSourceNamespace = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			fixture.OutputDebugOnFail(argoNamespace, sourceNamespace)

			namespaceFixture.Update(sourceNamespace, func(ns *corev1.Namespace) {
				if ns.Labels == nil {
					ns.Labels = map[string]string{}
				}
				ns.Labels[common.ArgoCDManagedByClusterArgoCDLabel] = argoNamespace.Name
			})

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
			Eventually(argoCD, "240s", "5s").Should(argocdFixture.BeAvailable())

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
		})
	})
})
