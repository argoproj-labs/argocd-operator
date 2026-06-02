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
	"k8s.io/utils/ptr"
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

		It("should preserve Redis HA naming for short CR names (backward compatibility)", func() {
			By("creating namespace for test")
			ns, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-redis-short-name")
			defer cleanupFunc()

			crName := "example-argocd"

			By("creating ArgoCD CR with short name and HA enabled")
			argoCDInstance := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					HA: argov1beta1api.ArgoCDHASpec{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCDInstance)).To(Succeed())

			// For short CR names full "redis-ha-server" suffix should be preserved
			expectedStatefulSetName := argoutil.NameWithSuffixForStatefulSet(argoCDInstance.ObjectMeta, "redis-ha-server")
			statefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      expectedStatefulSetName,
					Namespace: ns.Name,
				},
			}

			By("waiting for operator to create Redis HA StatefulSet with preserved name")
			Eventually(statefulSet, "5m", "5s").Should(k8sFixture.ExistByName())

			// Verify it uses the full suffix, not abbreviated
			Expect(expectedStatefulSetName).To(Equal(crName + "-redis-ha-server"))

			By("verifying no migration occurred (exactly one Redis HA StatefulSet)")
			statefulSetList := &appsv1.StatefulSetList{}
			Eventually(func() int {
				listOpts := []client.ListOption{
					client.InNamespace(ns.Name),
					client.MatchingLabels{"app.kubernetes.io/component": "redis"},
				}
				err := k8sClient.List(ctx, statefulSetList, listOpts...)
				if err != nil {
					return -1
				}
				return len(statefulSetList.Items)
			}, "2m", "5s").Should(Equal(1))

			By("verifying StatefulSet name is within 52 character limit")
			Expect(len(expectedStatefulSetName)).To(BeNumerically("<=", 52))
		})

		It("should abbreviate Redis HA naming for long CR names", func() {
			By("creating namespace for test")
			ns, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-redis-long-name")
			defer cleanupFunc()

			// Use a CR name long enough that {cr}-redis-ha-server exceeds the 52-char StatefulSet limit
			crName := "this-is-a-very-long-argocd-cr-name-for-testing"

			By("creating ArgoCD CR with long name and HA enabled")
			argoCDInstance := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					HA: argov1beta1api.ArgoCDHASpec{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCDInstance)).To(Succeed())

			// New StatefulSet name should be truncated to fit within the 52-char limit
			newStatefulSetName := argoutil.NameWithSuffixForStatefulSet(argoCDInstance.ObjectMeta, "redis-ha-server")
			newStatefulSet := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      newStatefulSetName,
					Namespace: ns.Name,
				},
			}

			By(fmt.Sprintf("waiting for operator to create truncated Redis HA StatefulSet: %s", newStatefulSetName))
			Eventually(newStatefulSet, "5m", "5s").Should(k8sFixture.ExistByName())

			// Verify STS name is hash-truncated to fit within the 52-char limit
			fullName := fmt.Sprintf("%s-redis-ha-server", crName)
			Expect(newStatefulSetName).NotTo(Equal(fullName), "Name should be truncated")
			Expect(len(newStatefulSetName)).To(BeNumerically("<=", 52), "Redis HA StatefulSet name should not exceed 52 characters")

			By("verifying pod template label uses truncated redis-ha component name")
			expectedRedisLabel := argoutil.NameWithSuffix(argoCDInstance.ObjectMeta, "redis-ha")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: newStatefulSetName, Namespace: ns.Name}, newStatefulSet)
				if err != nil {
					return false
				}
				podName := newStatefulSet.Spec.Template.Labels["app.kubernetes.io/name"]
				GinkgoWriter.Printf("Pod label 'app.kubernetes.io/name': %s (length: %d)\n", podName, len(podName))
				return podName == expectedRedisLabel && len(podName) <= 63
			}, "2m", "5s").Should(BeTrue())

			By("verifying no old-style StatefulSet exists (exactly one Redis HA StatefulSet)")
			statefulSetList := &appsv1.StatefulSetList{}
			Eventually(func() int {
				listOpts := []client.ListOption{
					client.InNamespace(ns.Name),
					client.MatchingLabels{"app.kubernetes.io/component": "redis"},
				}
				err := k8sClient.List(ctx, statefulSetList, listOpts...)
				if err != nil {
					return -1
				}
				return len(statefulSetList.Items)
			}, "2m", "5s").Should(Equal(1))
		})

		It("should migrate existing application controller StatefulSet from old long name to abbreviated name", func() {
			By("creating namespace for test")
			ns, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-app-migration")
			defer cleanupFunc()

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

			// Wait briefly for ArgoCD CR to be picked up by the operator
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      argoCDInstance.Name,
					Namespace: argoCDInstance.Namespace,
				}, argoCDInstance)
			}, "30s", "2s").Should(Succeed())

			By("creating legacy StatefulSet with previous operator naming (nameWithSuffix)")
			oldStatefulSetName := argoutil.NameWithSuffix(argoCDInstance.ObjectMeta, "application-controller")
			oldSS := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      oldStatefulSetName,
					Namespace: ns.Name,
					Labels: map[string]string{
						"app.kubernetes.io/name":      oldStatefulSetName,
						"app.kubernetes.io/part-of":   "argocd",
						"app.kubernetes.io/component": "application-controller",
					},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "argoproj.io/v1beta1",
						Kind:               "ArgoCD",
						Name:               argoCDInstance.Name,
						UID:                argoCDInstance.UID,
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					}},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To(int32(1)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": oldStatefulSetName,
						},
					},
					ServiceName: oldStatefulSetName,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": oldStatefulSetName,
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "application-controller",
								Image: "quay.io/argoproj/argocd:latest",
							}},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, oldSS)).To(Succeed())

			By("verifying old StatefulSet exists")
			Eventually(oldSS).Should(k8sFixture.ExistByName())

			By("waiting for operator to detect and migrate the StatefulSet")
			newStatefulSetName := argoutil.NameWithSuffixForStatefulSet(argoCDInstance.ObjectMeta, "application-controller")

			By(fmt.Sprintf("verifying old StatefulSet is eventually deleted: %s", oldStatefulSetName))
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      oldStatefulSetName,
					Namespace: ns.Name,
				}, oldSS)
				return err != nil
			}, "3m", "5s").Should(BeTrue(), "Old StatefulSet should be deleted during migration")

			By(fmt.Sprintf("verifying new StatefulSet is created with abbreviated name: %s", newStatefulSetName))
			newSS := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      newStatefulSetName,
					Namespace: ns.Name,
				},
			}
			Eventually(newSS, "3m", "5s").Should(k8sFixture.ExistByName())

			By("verifying only one application controller StatefulSet exists")
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
			}, "2m", "5s").Should(Equal(1), "Should have exactly one application controller StatefulSet")

			By("verifying the ArgoCD instance becomes available after migration")
			Eventually(argoCDInstance, "5m", "5s").Should(argocdFixture.BeAvailable())
		})

		It("should migrate existing Redis HA StatefulSet from old long name to abbreviated name", func() {
			By("creating namespace for test")
			ns, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-e2e-redis-migration")
			defer cleanupFunc()

			crName := "this-is-a-very-long-argocd-cr-name-for-testing"

			By("creating ArgoCD CR with long name and HA enabled")
			argoCDInstance := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      crName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					HA: argov1beta1api.ArgoCDHASpec{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCDInstance)).To(Succeed())

			// Wait briefly for ArgoCD CR to be picked up
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      argoCDInstance.Name,
					Namespace: argoCDInstance.Namespace,
				}, argoCDInstance)
			}, "30s", "2s").Should(Succeed())

			By("creating legacy Redis HA StatefulSet with previous operator naming (nameWithSuffix)")
			oldStatefulSetName := argoutil.NameWithSuffix(argoCDInstance.ObjectMeta, "redis-ha-server")
			oldRedisLabel := argoutil.NameWithSuffix(argoCDInstance.ObjectMeta, "redis-ha")
			oldSS := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      oldStatefulSetName,
					Namespace: ns.Name,
					Labels: map[string]string{
						"app.kubernetes.io/name":      oldRedisLabel,
						"app.kubernetes.io/part-of":   "argocd",
						"app.kubernetes.io/component": "redis",
					},
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion:         "argoproj.io/v1beta1",
						Kind:               "ArgoCD",
						Name:               argoCDInstance.Name,
						UID:                argoCDInstance.UID,
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(true),
					}},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: ptr.To(int32(3)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": oldRedisLabel,
						},
					},
					ServiceName: oldRedisLabel,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app.kubernetes.io/name": oldRedisLabel,
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name:  "redis",
								Image: "redis:7.0.11-alpine",
							}},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, oldSS)).To(Succeed())

			By("verifying old Redis HA StatefulSet exists")
			Eventually(oldSS).Should(k8sFixture.ExistByName())

			By("waiting for operator to detect and migrate the Redis HA StatefulSet")
			newStatefulSetName := argoutil.NameWithSuffixForStatefulSet(argoCDInstance.ObjectMeta, "redis-ha-server")

			By(fmt.Sprintf("verifying old Redis HA StatefulSet is eventually deleted: %s", oldStatefulSetName))
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      oldStatefulSetName,
					Namespace: ns.Name,
				}, oldSS)
				return err != nil
			}, "3m", "5s").Should(BeTrue(), "Old Redis HA StatefulSet should be deleted during migration")

			By(fmt.Sprintf("verifying new Redis HA StatefulSet is created with abbreviated name: %s", newStatefulSetName))
			newSS := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      newStatefulSetName,
					Namespace: ns.Name,
				},
			}
			Eventually(newSS, "3m", "5s").Should(k8sFixture.ExistByName())

			By("verifying only one Redis HA StatefulSet exists")
			statefulSetList := &appsv1.StatefulSetList{}
			Eventually(func() int {
				listOpts := []client.ListOption{
					client.InNamespace(ns.Name),
					client.MatchingLabels{"app.kubernetes.io/component": "redis"},
				}
				err := k8sClient.List(ctx, statefulSetList, listOpts...)
				if err != nil {
					return -1
				}
				return len(statefulSetList.Items)
			}, "2m", "5s").Should(Equal(1), "Should have exactly one Redis HA StatefulSet")
		})
	})
})
