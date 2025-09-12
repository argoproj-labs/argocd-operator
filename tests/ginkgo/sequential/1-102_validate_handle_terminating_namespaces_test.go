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

	argocdv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	appFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	namespaceFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-102_validate_handle_terminating_namespaces", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that if one managed-by namespace is stuck in terminating, it does not prevent other managed-by namespaces from being managed or deployed to", func() {

			By("creating simple namespace-scoped Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec:       argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("creating a namespace 'jane' containing a ConfigMap with a unowned finalizer")
			janeNs, cleanupFunc := fixture.CreateManagedNamespaceWithCleanupFunc("jane", ns.Name)
			defer cleanupFunc()

			configMapJaneNs := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "my-config-map-2", Namespace: janeNs.Name, Finalizers: []string{"some.random/finalizer"}},
			}
			Expect(k8sClient.Create(ctx, &configMapJaneNs)).To(Succeed())

			// At the end of the test, ensure the ConfigMap finalizer is removed so that the namespace is cleaned up
			defer func() {
				configmapFixture.Update(&configMapJaneNs, func(cm *corev1.ConfigMap) {
					cm.Finalizers = nil
				})
			}()

			By("deleting the jane NS in a background go routine, which puts the jane NS into a simulated stuck in terminating state")
			go func() {
				defer GinkgoRecover()
				Expect(k8sClient.Delete(ctx, janeNs)).To(Succeed())
			}()

			By("verifying jane ns moves into terminating state")
			Eventually(janeNs).Should(namespaceFixture.HavePhase(corev1.NamespaceTerminating))

			By("creating John NS")
			johnNs, cleanupFunc := fixture.CreateManagedNamespaceWithCleanupFunc("john", ns.Name)
			defer cleanupFunc()

			By("Wait for managed-by rolebindings to be created in John NS")
			Eventually(func() bool {
				var roleBindingList rbacv1.RoleBindingList
				if err := k8sClient.List(ctx, &roleBindingList, &client.ListOptions{Namespace: johnNs.Name}); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				match := false
				for _, roleBinding := range roleBindingList.Items {
					if strings.Contains(roleBinding.Name, "argocd-argocd-server") {
						match = true
						break
					}
				}
				if !match {
					GinkgoWriter.Println("argocd-server RoleBinding not yet found")
					return false
				}

				match = false
				for _, roleBinding := range roleBindingList.Items {
					if strings.Contains(roleBinding.Name, "argocd-application-controller") {
						match = true
						break
					}
				}
				if !match {
					GinkgoWriter.Println("argocd-application-controller RoleBinding not yet found")
					return false
				}

				return true
			}).Should(BeTrue())

			By("creating a test Argo CD Application targeting john NS")

			app := &argocdv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "my-app", Namespace: ns.Name},
				Spec: argocdv1alpha1.ApplicationSpec{
					Source: &argocdv1alpha1.ApplicationSource{
						Path:           "test/examples/kustomize-guestbook",
						RepoURL:        "https://github.com/redhat-developer/gitops-operator",
						TargetRevision: "HEAD",
					},
					Destination: argocdv1alpha1.ApplicationDestination{
						Namespace: johnNs.Name,
						Server:    "https://kubernetes.default.svc",
					},
					Project: "default",
					SyncPolicy: &argocdv1alpha1.SyncPolicy{
						Automated: &argocdv1alpha1.SyncPolicyAutomated{
							Prune:    true,
							SelfHeal: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("verifying Argo CD is successfully able to deploy to the John Namespace")

			Eventually(app, "4m", "5s").Should(appFixture.HaveSyncStatusCode(argocdv1alpha1.SyncStatusCodeSynced))

		})

	})
})
