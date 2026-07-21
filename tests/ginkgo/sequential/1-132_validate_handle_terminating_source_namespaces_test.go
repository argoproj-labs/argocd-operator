/*
Copyright 2026.

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

	argocdv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	appFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	appprojectFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/appproject"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	namespaceFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/namespace"
	statefulsetFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-132_validate_handle_terminating_source_namespaces", func() {

		var (
			k8sClient                client.Client
			ctx                      context.Context
			argoNS                   *corev1.Namespace
			terminatingNS            *corev1.Namespace
			activeNS                 *corev1.Namespace
			configMapTerminatingNS   *corev1.ConfigMap
			argoNSCleanupFunc        func()
			terminatingNSCleanupFunc func()
			activeNSCleanupFunc      func()
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			fixture.OutputDebugOnFail(argoNS, terminatingNS, activeNS)

			if configMapTerminatingNS != nil {
				configmapFixture.Update(configMapTerminatingNS, func(cm *corev1.ConfigMap) {
					cm.Finalizers = nil
				})
			}
			if activeNSCleanupFunc != nil {
				activeNSCleanupFunc()
			}
			if terminatingNSCleanupFunc != nil {
				terminatingNSCleanupFunc()
			}
			if argoNSCleanupFunc != nil {
				argoNSCleanupFunc()
			}
		})

		It("ensures that if one source namespace is stuck in terminating, it does not prevent other source namespaces from being managed or deployed to", func() {
			By("creating cluster-scoped Argo CD instance with sourceNamespaces wildcard")
			argoNS, argoNSCleanupFunc = fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-cluster-config")
			terminatingNS, terminatingNSCleanupFunc = fixture.CreateNamespaceWithCleanupFunc("src-1-132-terminating")

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: argoNS.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					SourceNamespaces: []string{"src-1-132-*"},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying terminating source namespace is initially managed")
			Eventually(terminatingNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNS.Name))

			terminatingRoleName := fmt.Sprintf("%s_%s", argoCD.Name, terminatingNS.Name)
			Eventually(&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: terminatingRoleName, Namespace: terminatingNS.Name},
			}).Should(k8sFixture.ExistByName())
			Eventually(&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: terminatingRoleName, Namespace: terminatingNS.Name},
			}).Should(k8sFixture.ExistByName())

			By("creating a ConfigMap with an unowned finalizer in the terminating source namespace")
			configMapTerminatingNS = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "blocking-finalizer-cm",
					Namespace:  terminatingNS.Name,
					Finalizers: []string{"some.random/finalizer"},
				},
			}
			Expect(k8sClient.Create(ctx, configMapTerminatingNS)).To(Succeed())

			By("deleting the terminating source namespace, which puts it into a simulated stuck Terminating state")
			go func() {
				defer GinkgoRecover()
				Expect(k8sClient.Delete(ctx, terminatingNS)).To(Succeed())
			}()

			By("verifying source namespace moves into terminating state")
			Eventually(terminatingNS).Should(namespaceFixture.HavePhase(corev1.NamespaceTerminating))

			By("creating an active source namespace that matches the same sourceNamespaces pattern")
			activeNS, activeNSCleanupFunc = fixture.CreateNamespaceWithCleanupFunc("src-1-132-active")

			By("verifying Role/RoleBinding are created in the active source namespace")
			activeRoleName := fmt.Sprintf("%s_%s", argoCD.Name, activeNS.Name)
			Eventually(&rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: activeRoleName, Namespace: activeNS.Name},
			}, "2m", "5s").Should(k8sFixture.ExistByName())
			Eventually(&rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: activeRoleName, Namespace: activeNS.Name},
			}, "2m", "5s").Should(k8sFixture.ExistByName())
			Eventually(activeNS).Should(namespaceFixture.HaveLabel("argocd.argoproj.io/managed-by-cluster-argocd", argoNS.Name))

			By("verifying ArgoCD CR remains Available")
			Eventually(argoCD, "2m", "5s").Should(argocdFixture.BeAvailable())
			Consistently(argoCD, "20s", "5s").Should(argocdFixture.BeAvailable())

			By("verifying --application-namespaces is set on server and application-controller")
			serverDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: argoCD.Name + "-server", Namespace: argoNS.Name},
			}
			Eventually(serverDeployment).Should(k8sFixture.ExistByName())
			Eventually(serverDeployment).Should(deploymentFixture.HaveContainerCommandSubstring("--application-namespaces", 0))

			appControllerStatefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: argoCD.Name + "-application-controller", Namespace: argoNS.Name},
			}
			Eventually(appControllerStatefulSet).Should(k8sFixture.ExistByName())
			Eventually(appControllerStatefulSet).Should(statefulsetFixture.HaveContainerCommandSubstring("--application-namespaces", 0))

			By("allowing Applications from the active source namespace in the default AppProject")
			appProject := &argocdv1alpha1.AppProject{
				ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: argoNS.Name},
			}
			Eventually(appProject).Should(k8sFixture.ExistByName())
			appprojectFixture.Update(appProject, func(project *argocdv1alpha1.AppProject) {
				project.Spec.SourceNamespaces = append(project.Spec.SourceNamespaces, activeNS.Name)
			})

			By("creating a test Argo CD Application in the active source namespace")
			app := &argocdv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "my-app", Namespace: activeNS.Name},
				Spec: argocdv1alpha1.ApplicationSpec{
					Source: &argocdv1alpha1.ApplicationSource{
						Path:           "test/examples/kustomize-guestbook",
						RepoURL:        "https://github.com/redhat-developer/gitops-operator",
						TargetRevision: "HEAD",
					},
					Destination: argocdv1alpha1.ApplicationDestination{
						Namespace: activeNS.Name,
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

			By("verifying Argo CD is successfully able to deploy to the active source namespace")
			Eventually(app, "4m", "5s").Should(appFixture.HaveSyncStatusCode(argocdv1alpha1.SyncStatusCodeSynced))
		})
	})
})
