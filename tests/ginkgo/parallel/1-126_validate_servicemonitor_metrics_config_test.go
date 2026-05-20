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
	"k8s.io/utils/ptr"
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
						Metrics: &argov1beta1api.ArgoCDMetricsSpec{
							Interval:      "30s",
							ScrapeTimeout: "10s",
						},
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Metrics: &argov1beta1api.ArgoCDMetricsSpec{
							Interval:      "45s",
							ScrapeTimeout: "15s",
						},
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Metrics: &argov1beta1api.ArgoCDMetricsSpec{
							Interval:      "60s",
							ScrapeTimeout: "20s",
						},
					},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled: true,
						Metrics: &argov1beta1api.ArgoCDMetricsSpec{
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
				ac.Spec.Controller.Metrics = &argov1beta1api.ArgoCDMetricsSpec{
					Interval: "120s", ScrapeTimeout: "50s",
				}
				ac.Spec.Repo.Metrics = &argov1beta1api.ArgoCDMetricsSpec{
					Interval: "150s", ScrapeTimeout: "60s",
				}
				ac.Spec.Server.Metrics = &argov1beta1api.ArgoCDMetricsSpec{
					Interval: "180s", ScrapeTimeout: "70s",
				}
				ac.Spec.Notifications.Metrics = &argov1beta1api.ArgoCDMetricsSpec{
					Interval: "200s", ScrapeTimeout: "80s",
				}
			})

			By("verifying controller ServiceMonitor is updated")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSM), controllerSM); err != nil {
					return false
				}
				if len(controllerSM.Spec.Endpoints) == 0 {
					return false
				}
				return controllerSM.Spec.Endpoints[0].Interval == monitoringv1.Duration("120s") &&
					controllerSM.Spec.Endpoints[0].ScrapeTimeout == monitoringv1.Duration("50s")
			}, "2m", "5s").Should(BeTrue())

			By("verifying repo-server ServiceMonitor is updated")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoSM), repoSM); err != nil {
					return false
				}
				if len(repoSM.Spec.Endpoints) == 0 {
					return false
				}
				return repoSM.Spec.Endpoints[0].Interval == monitoringv1.Duration("150s") &&
					repoSM.Spec.Endpoints[0].ScrapeTimeout == monitoringv1.Duration("60s")
			}, "2m", "5s").Should(BeTrue())

			By("verifying server ServiceMonitor is updated")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverSM), serverSM); err != nil {
					return false
				}
				if len(serverSM.Spec.Endpoints) == 0 {
					return false
				}
				return serverSM.Spec.Endpoints[0].Interval == monitoringv1.Duration("180s") &&
					serverSM.Spec.Endpoints[0].ScrapeTimeout == monitoringv1.Duration("70s")
			}, "2m", "5s").Should(BeTrue())

			By("verifying notifications ServiceMonitor is updated")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notifSM), notifSM); err != nil {
					return false
				}
				if len(notifSM.Spec.Endpoints) == 0 {
					return false
				}
				return notifSM.Spec.Endpoints[0].Interval == monitoringv1.Duration("200s") &&
					notifSM.Spec.Endpoints[0].ScrapeTimeout == monitoringv1.Duration("80s")
			}, "2m", "5s").Should(BeTrue())

			By("Case 3: Clear metrics from all components")

			By("clearing metrics config from all components")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.Metrics = nil
				ac.Spec.Repo.Metrics = nil
				ac.Spec.Server.Metrics = nil
				ac.Spec.Notifications.Metrics = nil
			})

			By("verifying controller ServiceMonitor has empty interval and scrapeTimeout")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerSM), controllerSM); err != nil {
					return false
				}
				if len(controllerSM.Spec.Endpoints) == 0 {
					return false
				}
				return controllerSM.Spec.Endpoints[0].Interval == "" &&
					controllerSM.Spec.Endpoints[0].ScrapeTimeout == ""
			}, "2m", "5s").Should(BeTrue())

			By("verifying repo-server ServiceMonitor has empty interval and scrapeTimeout")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoSM), repoSM); err != nil {
					return false
				}
				if len(repoSM.Spec.Endpoints) == 0 {
					return false
				}
				return repoSM.Spec.Endpoints[0].Interval == "" &&
					repoSM.Spec.Endpoints[0].ScrapeTimeout == ""
			}, "2m", "5s").Should(BeTrue())

			By("verifying server ServiceMonitor has empty interval and scrapeTimeout")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverSM), serverSM); err != nil {
					return false
				}
				if len(serverSM.Spec.Endpoints) == 0 {
					return false
				}
				return serverSM.Spec.Endpoints[0].Interval == "" &&
					serverSM.Spec.Endpoints[0].ScrapeTimeout == ""
			}, "2m", "5s").Should(BeTrue())

			By("verifying notifications ServiceMonitor has empty interval and scrapeTimeout")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notifSM), notifSM); err != nil {
					return false
				}
				if len(notifSM.Spec.Endpoints) == 0 {
					return false
				}
				return notifSM.Spec.Endpoints[0].Interval == "" &&
					notifSM.Spec.Endpoints[0].ScrapeTimeout == ""
			}, "2m", "5s").Should(BeTrue())
		})

		It("verifies principal metrics config is applied to ServiceMonitor", func() {

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCDName := "example-argocd"
			principalSMName := argoCDName + "-agent-principal-metrics"

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Prometheus: argov1beta1api.ArgoCDPrometheusSpec{Enabled: true},
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{Enabled: ptr.To(false)},
					ArgoCDAgent: &argov1beta1api.ArgoCDAgentSpec{
						Principal: &argov1beta1api.PrincipalSpec{
							Enabled: ptr.To(true),
							Metrics: &argov1beta1api.ArgoCDMetricsSpec{
								Interval:      "40s",
								ScrapeTimeout: "12s",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			principalSM := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{Name: principalSMName, Namespace: ns.Name},
			}
			Eventually(principalSM, "2m", "5s").Should(k8sFixture.ExistByName())
			Expect(principalSM.Spec.Endpoints).To(HaveLen(1))
			Expect(principalSM.Spec.Endpoints[0].Interval).To(Equal(monitoringv1.Duration("40s")))
			Expect(principalSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("12s")))

			By("updating principal metrics config")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Principal.Metrics = &argov1beta1api.ArgoCDMetricsSpec{
					Interval: "130s", ScrapeTimeout: "55s",
				}
			})

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(principalSM), principalSM); err != nil {
					return false
				}
				if len(principalSM.Spec.Endpoints) == 0 {
					return false
				}
				return principalSM.Spec.Endpoints[0].Interval == monitoringv1.Duration("130s") &&
					principalSM.Spec.Endpoints[0].ScrapeTimeout == monitoringv1.Duration("55s")
			}, "2m", "5s").Should(BeTrue())

			By("clearing principal metrics config")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Principal.Metrics = nil
			})

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(principalSM), principalSM); err != nil {
					return false
				}
				if len(principalSM.Spec.Endpoints) == 0 {
					return false
				}
				return principalSM.Spec.Endpoints[0].Interval == "" &&
					principalSM.Spec.Endpoints[0].ScrapeTimeout == ""
			}, "2m", "5s").Should(BeTrue())
		})

		It("verifies agent metrics config is applied to ServiceMonitor", func() {

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCDName := "example-argocd"
			agentSMName := argoCDName + "-agent-agent-metrics"

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Prometheus: argov1beta1api.ArgoCDPrometheusSpec{Enabled: true},
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{Enabled: ptr.To(false)},
					Server:     argov1beta1api.ArgoCDServerSpec{Enabled: ptr.To(false)},
					ArgoCDAgent: &argov1beta1api.ArgoCDAgentSpec{
						Agent: &argov1beta1api.AgentSpec{
							Enabled: ptr.To(true),
							Metrics: &argov1beta1api.ArgoCDMetricsSpec{
								Interval:      "50s",
								ScrapeTimeout: "18s",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			agentSM := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{Name: agentSMName, Namespace: ns.Name},
			}
			Eventually(agentSM, "2m", "5s").Should(k8sFixture.ExistByName())
			Expect(agentSM.Spec.Endpoints).To(HaveLen(1))
			Expect(agentSM.Spec.Endpoints[0].Interval).To(Equal(monitoringv1.Duration("50s")))
			Expect(agentSM.Spec.Endpoints[0].ScrapeTimeout).To(Equal(monitoringv1.Duration("18s")))

			By("updating agent metrics config")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Agent.Metrics = &argov1beta1api.ArgoCDMetricsSpec{
					Interval: "160s", ScrapeTimeout: "65s",
				}
			})

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(agentSM), agentSM); err != nil {
					return false
				}
				if len(agentSM.Spec.Endpoints) == 0 {
					return false
				}
				return agentSM.Spec.Endpoints[0].Interval == monitoringv1.Duration("160s") &&
					agentSM.Spec.Endpoints[0].ScrapeTimeout == monitoringv1.Duration("65s")
			}, "2m", "5s").Should(BeTrue())

			By("clearing agent metrics config")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Agent.Metrics = nil
			})

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(agentSM), agentSM); err != nil {
					return false
				}
				if len(agentSM.Spec.Endpoints) == 0 {
					return false
				}
				return agentSM.Spec.Endpoints[0].Interval == "" &&
					agentSM.Spec.Endpoints[0].ScrapeTimeout == ""
			}, "2m", "5s").Should(BeTrue())
		})
	})
})
