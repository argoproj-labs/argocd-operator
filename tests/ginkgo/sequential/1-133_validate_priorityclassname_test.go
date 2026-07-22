/*
Copyright 2025 ArgoCD Operator Developers

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
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	Context("1-133_validate_priorityclassname", func() {

		const (
			testPriorityClassName        = "e2e-test-high-priority"
			testPriorityClassNameUpdated = "e2e-test-medium-priority"
		)

		var (
			k8sClient   client.Client
			ctx         context.Context
			ns          *corev1.Namespace
			cleanupFunc func()
		)

		// allDeploymentPriorityClasses returns the sorted, deduplicated set of
		// priorityClassName values across all Deployments in ns. When every
		// Deployment carries the same class the result is []string{thatClass}.
		var allDeploymentPriorityClasses = func() []string {
			var list appsv1.DeploymentList
			if err := k8sClient.List(ctx, &list, client.InNamespace(ns.Name)); err != nil {
				return nil
			}
			seen := map[string]struct{}{}
			for _, d := range list.Items {
				seen[d.Spec.Template.Spec.PriorityClassName] = struct{}{}
			}
			result := make([]string, 0, len(seen))
			for name := range seen {
				result = append(result, name)
			}
			sort.Strings(result)
			return result
		}

		// allStatefulSetPriorityClasses returns the sorted, deduplicated set of
		// priorityClassName values across all StatefulSets in ns.
		var allStatefulSetPriorityClasses = func() []string {
			var list appsv1.StatefulSetList
			if err := k8sClient.List(ctx, &list, client.InNamespace(ns.Name)); err != nil {
				return nil
			}
			seen := map[string]struct{}{}
			for _, ss := range list.Items {
				seen[ss.Spec.Template.Spec.PriorityClassName] = struct{}{}
			}
			result := make([]string, 0, len(seen))
			for name := range seen {
				result = append(result, name)
			}
			sort.Strings(result)
			return result
		}

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			if ns != nil {
				fixture.OutputDebugOnFail(ns)
			}
			if cleanupFunc != nil {
				cleanupFunc()
			}

			// Clean up PriorityClass resources created for this test
			for _, pcName := range []string{testPriorityClassName, testPriorityClassNameUpdated} {
				pc := &schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: pcName}}
				if err := k8sClient.Delete(ctx, pc); err != nil && !apierrors.IsNotFound(err) {
					GinkgoWriter.Printf("Failed to delete PriorityClass %s: %v\n", pcName, err)
				}
			}
		})

		It("verifies spec.priorityClassName is applied to all operator-managed workloads", func() {

			By("creating PriorityClass resources for the test")
			pc := &schedulingv1.PriorityClass{
				ObjectMeta: metav1.ObjectMeta{Name: testPriorityClassName},
				Value:      int32(1000000),
			}
			Expect(k8sClient.Create(ctx, pc)).To(Succeed())

			pcUpdated := &schedulingv1.PriorityClass{
				ObjectMeta: metav1.ObjectMeta{Name: testPriorityClassNameUpdated},
				Value:      int32(500000),
			}
			Expect(k8sClient.Create(ctx, pcUpdated)).To(Succeed())

			By("creating a namespace-scoped ArgoCD instance with priorityClassName and all optional components enabled")
			ns, cleanupFunc = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			enabled := true
			argoCD := &argoproj.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argoproj.ArgoCDSpec{
					PriorityClassName: testPriorityClassName,
					ApplicationSet: &argoproj.ArgoCDApplicationSet{
						Enabled: &enabled,
					},
					Notifications: argoproj.ArgoCDNotifications{
						Enabled: true,
					},
					Server: argoproj.ArgoCDServerSpec{
						Route: argoproj.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying all Deployments and StatefulSets have the expected priorityClassName")
			Eventually(allDeploymentPriorityClasses, "2m", "5s").Should(Equal([]string{testPriorityClassName}))
			Eventually(allStatefulSetPriorityClasses, "2m", "5s").Should(Equal([]string{testPriorityClassName}))

			By("updating priorityClassName to a new value")
			argocdFixture.Update(argoCD, func(ac *argoproj.ArgoCD) {
				ac.Spec.PriorityClassName = testPriorityClassNameUpdated
			})

			By("verifying all Deployments and StatefulSets are updated with the new priorityClassName")
			Eventually(allDeploymentPriorityClasses, "3m", "5s").Should(Equal([]string{testPriorityClassNameUpdated}))
			Eventually(allStatefulSetPriorityClasses, "3m", "5s").Should(Equal([]string{testPriorityClassNameUpdated}))

			By("clearing priorityClassName and verifying all workloads no longer have it set")
			argocdFixture.Update(argoCD, func(ac *argoproj.ArgoCD) {
				ac.Spec.PriorityClassName = ""
			})

			Eventually(allDeploymentPriorityClasses, "3m", "5s").Should(Equal([]string{""}))
			Eventually(allStatefulSetPriorityClasses, "3m", "5s").Should(Equal([]string{""}))
		})

		It("verifies that workloads without priorityClassName set use empty string (default behaviour)", func() {
			By("creating a namespace-scoped ArgoCD instance without priorityClassName")
			ns, cleanupFunc = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			enabled := true
			argoCD := &argoproj.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argoproj.ArgoCDSpec{
					ApplicationSet: &argoproj.ArgoCDApplicationSet{
						Enabled: &enabled,
					},
					Notifications: argoproj.ArgoCDNotifications{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying all Deployments and StatefulSets have no priorityClassName set")
			Eventually(allDeploymentPriorityClasses, "2m", "5s").Should(Equal([]string{""}))
			Eventually(allStatefulSetPriorityClasses, "2m", "5s").Should(Equal([]string{""}))
		})
	})
})
