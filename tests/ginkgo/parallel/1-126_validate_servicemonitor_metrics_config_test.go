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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-126_validate_servicemonitor_metrics_config", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies per-component metrics config is applied to ServiceMonitors", func() {

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			controllerSMName := "example-argocd-metrics"
			repoSMName := "example-argocd-repo-server-metrics"
			serverSMName := "example-argocd-server-metrics"
			notifSMName := "example-argocd-notifications-controller-metrics"

			By("Case 1: Create with both interval and scrapeTimeout on all components")

			By("creating ArgoCD instance with metrics config on all components")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Prometheus: argov1beta1api.ArgoCDPrometheusSpec{
						Enabled: true,
					},
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Metrics: argov1beta1api.ArgoCDMetricsSpec{
							Interval:      "30s",
							ScrapeTimeout: "10s",
						},
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Metrics: argov1beta1api.ArgoCDMetricsSpec{
							Interval:      "45s",
							ScrapeTimeout: "15s",
						},
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Metrics: argov1beta1api.ArgoCDMetricsSpec{
							Interval:      "60s",
							ScrapeTimeout: "20s",
						},
					},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled: true,
						Metrics: argov1beta1api.ArgoCDMetricsSpec{
							Interval:      "90s",
							ScrapeTimeout: "30s",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying controller ServiceMonitor")
			controllerSM := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{Name: controllerSMName, Namespace: ns.Name},
			}
			Eventually(controllerSM, "2m", "5s").Should(k8sFixture.ExistByName())
			Expect(controllerSM.Spec.Endpoints).To(HaveLen(1))
			Expect(controllerSM.Spec.Endpoints[0].Interval).To(Equal(monitoringv1.Duration("30s")))
			Expect(controllerSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("10s")))

			By("verifying repo-server ServiceMonitor")
			repoSM := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{Name: repoSMName, Namespace: ns.Name},
			}
			Eventually(repoSM, "2m", "5s").Should(k8sFixture.ExistByName())
			Expect(repoSM.Spec.Endpoints).To(HaveLen(1))
			Expect(repoSM.Spec.Endpoints[0].Interval).To(Equal(monitoringv1.Duration("45s")))
			Expect(repoSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("15s")))

			By("verifying server ServiceMonitor")
			serverSM := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{Name: serverSMName, Namespace: ns.Name},
			}
			Eventually(serverSM, "2m", "5s").Should(k8sFixture.ExistByName())
			Expect(serverSM.Spec.Endpoints).To(HaveLen(1))
			Expect(serverSM.Spec.Endpoints[0].Interval).To(Equal(monitoringv1.Duration("60s")))
			Expect(serverSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("20s")))

			By("verifying notifications ServiceMonitor")
			notifSM := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{Name: notifSMName, Namespace: ns.Name},
			}
			Eventually(notifSM, "2m", "5s").Should(k8sFixture.ExistByName())
			Expect(notifSM.Spec.Endpoints).To(HaveLen(1))
			Expect(notifSM.Spec.Endpoints[0].Interval).To(Equal(monitoringv1.Duration("90s")))
			Expect(notifSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("30s")))

			By("Case 2: Update components with new values")

			By("updating metrics config on all components")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.Metrics = argov1beta1api.ArgoCDMetricsSpec{
					Interval: "120s", ScrapeTimeout: "50s",
				}
				ac.Spec.Repo.Metrics = argov1beta1api.ArgoCDMetricsSpec{
					Interval: "150s", ScrapeTimeout: "60s",
				}
				ac.Spec.Server.Metrics = argov1beta1api.ArgoCDMetricsSpec{
					Interval: "180s", ScrapeTimeout: "70s",
				}
				ac.Spec.Notifications.Metrics = argov1beta1api.ArgoCDMetricsSpec{
					Interval: "200s", ScrapeTimeout: "80s",
				}
			})

			By("verifying controller ServiceMonitor is updated")
			Eventually(func() monitoringv1.Duration {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSM), controllerSM)
				if len(controllerSM.Spec.Endpoints) == 0 {
					return ""
				}
				return controllerSM.Spec.Endpoints[0].Interval
			}, "2m", "5s").Should(Equal(monitoringv1.Duration("120s")))
			Expect(controllerSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("50s")))

			By("verifying repo-server ServiceMonitor is updated")
			Eventually(func() monitoringv1.Duration {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(repoSM), repoSM)
				if len(repoSM.Spec.Endpoints) == 0 {
					return ""
				}
				return repoSM.Spec.Endpoints[0].Interval
			}, "2m", "5s").Should(Equal(monitoringv1.Duration("150s")))
			Expect(repoSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("60s")))

			By("verifying server ServiceMonitor is updated")
			Eventually(func() monitoringv1.Duration {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(serverSM), serverSM)
				if len(serverSM.Spec.Endpoints) == 0 {
					return ""
				}
				return serverSM.Spec.Endpoints[0].Interval
			}, "2m", "5s").Should(Equal(monitoringv1.Duration("180s")))
			Expect(serverSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("70s")))

			By("verifying notifications ServiceMonitor is updated")
			Eventually(func() monitoringv1.Duration {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(notifSM), notifSM)
				if len(notifSM.Spec.Endpoints) == 0 {
					return ""
				}
				return notifSM.Spec.Endpoints[0].Interval
			}, "2m", "5s").Should(Equal(monitoringv1.Duration("200s")))
			Expect(notifSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("80s")))

			By("Case 3: Clear metrics from all components")

			By("clearing metrics config from all components")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.Metrics = argov1beta1api.ArgoCDMetricsSpec{}
				ac.Spec.Repo.Metrics = argov1beta1api.ArgoCDMetricsSpec{}
				ac.Spec.Server.Metrics = argov1beta1api.ArgoCDMetricsSpec{}
				ac.Spec.Notifications.Metrics = argov1beta1api.ArgoCDMetricsSpec{}
			})

			By("verifying controller ServiceMonitor has empty interval and scrapeTimeout")
			Eventually(func() monitoringv1.Duration {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSM), controllerSM)
				if len(controllerSM.Spec.Endpoints) == 0 {
					return "pending"
				}
				return controllerSM.Spec.Endpoints[0].Interval
			}, "2m", "5s").Should(Equal(monitoringv1.Duration("")))
			Expect(controllerSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("")))

			By("verifying repo-server ServiceMonitor has empty interval and scrapeTimeout")
			Eventually(func() monitoringv1.Duration {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(repoSM), repoSM)
				if len(repoSM.Spec.Endpoints) == 0 {
					return "pending"
				}
				return repoSM.Spec.Endpoints[0].Interval
			}, "2m", "5s").Should(Equal(monitoringv1.Duration("")))
			Expect(repoSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("")))

			By("verifying server ServiceMonitor has empty interval and scrapeTimeout")
			Eventually(func() monitoringv1.Duration {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(serverSM), serverSM)
				if len(serverSM.Spec.Endpoints) == 0 {
					return "pending"
				}
				return serverSM.Spec.Endpoints[0].Interval
			}, "2m", "5s").Should(Equal(monitoringv1.Duration("")))
			Expect(serverSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("")))

			By("verifying notifications ServiceMonitor has empty interval and scrapeTimeout")
			Eventually(func() monitoringv1.Duration {
				_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(notifSM), notifSM)
				if len(notifSM.Spec.Endpoints) == 0 {
					return "pending"
				}
				return notifSM.Spec.Endpoints[0].Interval
			}, "2m", "5s").Should(Equal(monitoringv1.Duration("")))
			Expect(notifSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("")))
		})
	})
})
