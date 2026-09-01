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

package parallel

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-127_validate_metrics_service_annotations", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies custom annotations are applied to metrics services", func() {

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			controllerSvcName := "example-argocd-metrics"
			repoSvcName := "example-argocd-repo-server"
			serverSvcName := "example-argocd-server-metrics"
			notifSvcName := "example-argocd-notifications-controller-metrics"

			By("Case 1: Create ArgoCD with custom annotations on all metrics services")

			By("creating ArgoCD instance with metrics annotations on all components")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Metrics: &argov1beta1api.ArgoCDMetricsSpec{
							Annotations: map[string]string{
								"ad.datadoghq.com/service.check_names":  `["openmetrics"]`,
								"ad.datadoghq.com/service.init_configs": `[{}]`,
								"ad.datadoghq.com/service.instances":    `[{"openmetrics_endpoint":"http://%%host%%:%%port%%/metrics","namespace":"argocd_controller","metrics":[".*"]}]`,
								"custom.io/controller":                  "controller-value",
							},
						},
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Metrics: &argov1beta1api.ArgoCDMetricsSpec{
							Annotations: map[string]string{
								"custom.io/repo": "repo-value",
							},
						},
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Metrics: &argov1beta1api.ArgoCDMetricsSpec{
							Annotations: map[string]string{
								"custom.io/server": "server-value",
							},
						},
					},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled: true,
						Metrics: &argov1beta1api.ArgoCDMetricsSpec{
							Annotations: map[string]string{
								"custom.io/notifications": "notifications-value",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying controller metrics service has custom annotations")
			controllerSvc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      controllerSvcName,
					Namespace: ns.Name,
				},
			}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSvc), controllerSvc); err != nil {
					return false
				}
				return controllerSvc.Annotations["ad.datadoghq.com/service.check_names"] == `["openmetrics"]` &&
					controllerSvc.Annotations["ad.datadoghq.com/service.init_configs"] == `[{}]` &&
					controllerSvc.Annotations["ad.datadoghq.com/service.instances"] == `[{"openmetrics_endpoint":"http://%%host%%:%%port%%/metrics","namespace":"argocd_controller","metrics":[".*"]}]` &&
					controllerSvc.Annotations["custom.io/controller"] == "controller-value"
			}, "2m", "5s").Should(BeTrue())

			By("verifying repo server service has custom annotations")
			repoSvc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      repoSvcName,
					Namespace: ns.Name,
				},
			}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoSvc), repoSvc); err != nil {
					return false
				}
				return repoSvc.Annotations["custom.io/repo"] == "repo-value"
			}, "2m", "5s").Should(BeTrue())

			By("verifying server metrics service has custom annotations")
			serverSvc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serverSvcName,
					Namespace: ns.Name,
				},
			}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverSvc), serverSvc); err != nil {
					return false
				}
				return serverSvc.Annotations["custom.io/server"] == "server-value"
			}, "2m", "5s").Should(BeTrue())

			By("verifying notifications metrics service has custom annotations")
			notifSvc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      notifSvcName,
					Namespace: ns.Name,
				},
			}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notifSvc), notifSvc); err != nil {
					return false
				}
				return notifSvc.Annotations["custom.io/notifications"] == "notifications-value"
			}, "2m", "5s").Should(BeTrue())

			By("Case 2: Update annotations and verify they are updated")

			By("updating ArgoCD instance with new annotations")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.Controller.Metrics.Annotations["custom.io/controller"] = "updated-controller-value"
			argoCD.Spec.Server.Metrics.Annotations["custom.io/server"] = "updated-server-value"
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())

			By("verifying controller metrics service annotations are updated")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSvc), controllerSvc); err != nil {
					return false
				}
				return controllerSvc.Annotations["custom.io/controller"] == "updated-controller-value"
			}, "2m", "5s").Should(BeTrue())

			By("verifying server metrics service annotations are updated")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverSvc), serverSvc); err != nil {
					return false
				}
				return serverSvc.Annotations["custom.io/server"] == "updated-server-value"
			}, "2m", "5s").Should(BeTrue())

			By("Case 3: Remove annotations from spec and verify they are removed")

			By("updating ArgoCD instance to remove metrics annotation config")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCD), argoCD)).To(Succeed())
			argoCD.Spec.Notifications.Metrics.Annotations = nil
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())

			By("verifying notifications service annotations are removed")
			// Annotations should be removed when deleted from spec (except k8s-managed annotations)
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notifSvc), notifSvc); err != nil {
					return false
				}
				// Custom annotation should be removed
				_, hasAnnotation := notifSvc.Annotations["custom.io/notifications"]
				return !hasAnnotation
			}, "2m", "5s").Should(BeTrue())
		})
	})
})
