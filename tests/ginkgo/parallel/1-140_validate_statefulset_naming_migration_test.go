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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-140_validate_statefulset_naming_migration", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = utils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("should preserve original naming for short CR names (backward compatibility)", func() {

			By("creating namespace for test")
			ns, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-short-name")
			defer cleanupFunc()

			// Use a short CR name that does not need abbreviation
			crName := "example-argocd"

			By("creating ArgoCD CR with short name")
			argoCDInstance := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCDInstance)).To(Succeed())

			By("waiting for operator to reconcile")
			Eventually(argoCDInstance, "5m", "5s").Should(argocdFixture.BeAvailable())

			// For short CR names full "application-controller" suffix should be preserved
			expectedStatefulSetName := argoutil.NameWithSuffixForStatefulSet(argoCDInstance.ObjectMeta, "application-controller")
			statefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedStatefulSetName,
					Namespace: ns.Name,
				},
			}

			By(fmt.Sprintf("verifying StatefulSet preserves original suffix: %s", expectedStatefulSetName))
			Eventually(statefulSet, "2m", "5s").Should(k8sFixture.ExistByName())

			// Verify it uses the full suffix, not abbreviated
			Expect(expectedStatefulSetName).To(Equal(crName + "-application-controller"))

			By("verifying no migration occurred (exactly one application controller StatefulSet)")
			statefulSetList := &appsv1.StatefulSetList{}
			Eventually(func() int {
				listOpts := []client.ListOption{
					client.InNamespace(ns.Name),
					client.MatchingLabels{"app.kubernetes.io/component": "application-controller"},
				}
				err := k8sClient.List(ctx, statefulSetList, listOpts...)
				if err != nil {
					return -1
				}
				return len(statefulSetList.Items)
			}, "1m", "5s").Should(Equal(1), "Should have exactly one application controller StatefulSet (no migration)")

			By("verifying StatefulSet has correct pod template labels")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: expectedStatefulSetName, Namespace: ns.Name}, statefulSet)
				if err != nil {
					return false
				}
				podName := statefulSet.Spec.Template.Labels["app.kubernetes.io/name"]
				GinkgoWriter.Printf("Pod label 'app.kubernetes.io/name': %s (length: %d)\n", podName, len(podName))
				return len(podName) <= 63 && len(podName) > 0
			}, "1m", "2s").Should(BeTrue())

			By("verifying application controller pod is running")
			Eventually(func() bool {
				podList := &corev1.PodList{}
				listOpts := []client.ListOption{
					client.InNamespace(ns.Name),
					client.MatchingLabels{"app.kubernetes.io/name": expectedStatefulSetName},
				}
				err := k8sClient.List(ctx, podList, listOpts...)
				if err != nil {
					return false
				}
				if len(podList.Items) == 0 {
					return false
				}
				for _, pod := range podList.Items {
					if pod.Status.Phase == corev1.PodRunning {
						GinkgoWriter.Printf("Application controller pod is running: %s\n", pod.Name)
						return true
					}
				}
				return false
			}, "5m", "5s").Should(BeTrue())
		})

		It("should abbreviate naming only for long CR names", func() {

			By("creating namespace for test")
			ns, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-long-name")
			defer cleanupFunc()

			// Use a long CR name that will trigger abbreviation
			crName := "this-name-will-push-the-char-limit"

			By("creating ArgoCD CR with long name")
			argoCDInstance := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCDInstance)).To(Succeed())

			By("waiting for operator to reconcile and create StatefulSet with abbreviated name")
			Eventually(argoCDInstance, "5m", "5s").Should(argocdFixture.BeAvailable())

			// New StatefulSet name should use abbreviation for long CR names
			newStatefulSetName := argoutil.NameWithSuffixForStatefulSet(argoCDInstance.ObjectMeta, "application-controller")
			newStatefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      newStatefulSetName,
					Namespace: ns.Name,
				},
			}

			By(fmt.Sprintf("verifying StatefulSet uses abbreviated suffix: %s", newStatefulSetName))
			Eventually(newStatefulSet, "2m", "5s").Should(k8sFixture.ExistByName())

			// Verify it uses abbreviated suffix
			Expect(newStatefulSetName).To(ContainSubstring("app-controller"))
			Expect(newStatefulSetName).NotTo(ContainSubstring("application-controller"))

			By("verifying StatefulSet has correct pod template labels (within 63 char limit)")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: newStatefulSetName, Namespace: ns.Name}, newStatefulSet)
				if err != nil {
					return false
				}
				// Verify pod labels are within Kubernetes 63 character limit
				podName := newStatefulSet.Spec.Template.Labels["app.kubernetes.io/name"]
				GinkgoWriter.Printf("Pod label 'app.kubernetes.io/name': %s (length: %d)\n", podName, len(podName))
				return len(podName) <= 63 && len(podName) > 0
			}, "1m", "2s").Should(BeTrue())

			By("verifying application controller pod is running")
			Eventually(func() bool {
				podList := &corev1.PodList{}
				listOpts := []client.ListOption{
					client.InNamespace(ns.Name),
					client.MatchingLabels{"app.kubernetes.io/name": newStatefulSetName},
				}
				err := k8sClient.List(ctx, podList, listOpts...)
				if err != nil {
					return false
				}
				if len(podList.Items) == 0 {
					return false
				}
				for _, pod := range podList.Items {
					if pod.Status.Phase == corev1.PodRunning {
						GinkgoWriter.Printf("Application controller pod is running: %s\n", pod.Name)
						return true
					}
				}
				return false
			}, "5m", "5s").Should(BeTrue())
		})
	})
})
