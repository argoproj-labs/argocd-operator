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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	routev1 "github.com/openshift/api/route/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

// containsNoKindMatch checks if error message indicates API unavailability
func containsNoKindMatch(errMsg string) bool {
	return strings.Contains(errMsg, "no matches for kind")
}

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-123_validate_prometheus_cr_deleted", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies Prometheus CR is NOT created and deleted if it exists", func() {

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating ArgoCD instance with prometheus.enabled: true")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Prometheus: argov1beta1api.ArgoCDPrometheusSpec{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying Prometheus CR is NOT created as it is deprecated")
			prometheusCR := &monitoringv1.Prometheus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: ns.Name,
				},
			}
			Eventually(prometheusCR, "1m", "5s").Should(k8sFixture.NotExistByName())
			Consistently(prometheusCR, "30s", "5s").Should(k8sFixture.NotExistByName())

			By("verifying ServiceMonitor for application-controller metrics is created")
			metricsServiceMonitor := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-metrics",
					Namespace: ns.Name,
				},
			}
			Eventually(metricsServiceMonitor, "2m", "5s").Should(k8sFixture.ExistByName())

			By("verifying ServiceMonitor for repo-server is created")
			repoServerServiceMonitor := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-repo-server-metrics",
					Namespace: ns.Name,
				},
			}
			Eventually(repoServerServiceMonitor, "2m", "5s").Should(k8sFixture.ExistByName())

			By("verifying ServiceMonitor for server is created")
			serverServiceMonitor := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-server-metrics",
					Namespace: ns.Name,
				},
			}
			Eventually(serverServiceMonitor, "2m", "5s").Should(k8sFixture.ExistByName())

			By("manually creating a Prometheus CR to simulate existing installation")
			// Simulate a Prometheus CR that might have existed from a previous version
			prometheusCR = &monitoringv1.Prometheus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: ns.Name,
					Labels: map[string]string{
						"app.kubernetes.io/name":    "prometheus",
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Spec: monitoringv1.PrometheusSpec{
					CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
						Replicas: func() *int32 { r := int32(1); return &r }(),
					},
				},
			}
			Expect(k8sClient.Create(ctx, prometheusCR)).To(Succeed())

			By("verifying the manually created Prometheus CR exists")
			Eventually(prometheusCR, "30s", "5s").Should(k8sFixture.ExistByName())

			By("triggering reconciliation by updating ArgoCD")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				if ac.Annotations == nil {
					ac.Annotations = make(map[string]string)
				}
				ac.Annotations["test.trigger"] = "reconcile"
			})

			By("verifying Prometheus CR is deleted by the operator")
			Eventually(prometheusCR, "2m", "5s").Should(k8sFixture.NotExistByName())

			By("verifying ServiceMonitors are still present after Prometheus CR deletion")
			Eventually(metricsServiceMonitor, "30s", "5s").Should(k8sFixture.ExistByName())
			Eventually(repoServerServiceMonitor, "30s", "5s").Should(k8sFixture.ExistByName())
			Eventually(serverServiceMonitor, "30s", "5s").Should(k8sFixture.ExistByName())

			By("testing with prometheus.enabled: false")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Prometheus.Enabled = false
			})

			By("verifying ServiceMonitors are deleted when prometheus is disabled")
			Eventually(metricsServiceMonitor, "2m", "5s").Should(k8sFixture.NotExistByName())
			Eventually(repoServerServiceMonitor, "2m", "5s").Should(k8sFixture.NotExistByName())
			Eventually(serverServiceMonitor, "2m", "5s").Should(k8sFixture.NotExistByName())

			By("re-enabling prometheus and testing with monitoring enabled")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Prometheus.Enabled = true
				ac.Spec.Monitoring = argov1beta1api.ArgoCDMonitoringSpec{
					Enabled: true,
				}
			})

			By("verifying ServiceMonitors are recreated")
			Eventually(metricsServiceMonitor, "2m", "5s").Should(k8sFixture.ExistByName())
			Eventually(repoServerServiceMonitor, "2m", "5s").Should(k8sFixture.ExistByName())
			Eventually(serverServiceMonitor, "2m", "5s").Should(k8sFixture.ExistByName())

			By("verifying PrometheusRule is created when monitoring is enabled")
			prometheusRule := &monitoringv1.PrometheusRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-component-status-alert",
					Namespace: ns.Name,
				},
			}
			Eventually(prometheusRule, "2m", "5s").Should(k8sFixture.ExistByName())

			By("verifying Prometheus CR is still NOT created even with monitoring enabled")
			Eventually(prometheusCR, "1m", "5s").Should(k8sFixture.NotExistByName())
			Consistently(prometheusCR, "30s", "5s").Should(k8sFixture.NotExistByName())
		})

		It("verifies Prometheus Route and Ingress are deleted if they exist", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating ArgoCD instance with prometheus enabled")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Prometheus: argov1beta1api.ArgoCDPrometheusSpec{
						Enabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying Prometheus Ingress is NOT created")
			prometheusIngress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-prometheus",
					Namespace: ns.Name,
				},
			}
			Eventually(prometheusIngress, "1m", "5s").Should(k8sFixture.NotExistByName())
			Consistently(prometheusIngress, "30s", "5s").Should(k8sFixture.NotExistByName())

			By("manually creating a Prometheus Route to simulate existing installation")
			prometheusRoute := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-prometheus",
					Namespace: ns.Name,
					Labels: map[string]string{
						"app.kubernetes.io/name":    "prometheus",
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Spec: routev1.RouteSpec{
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "example-argocd-prometheus",
					},
				},
			}

			// Try to create Route , skip deletion test if Route API not available (not Openshift cluster)
			err := k8sClient.Create(ctx, prometheusRoute)
			if err != nil && containsNoKindMatch(err.Error()) {
				GinkgoWriter.Println("Route API not available(not Openshift cluster), skipping Route deletion test")
			} else if err != nil && !errors.IsAlreadyExists(err) {
				Expect(err).ToNot(HaveOccurred())
			} else {
				// Route created successfully, test deletion
				By("verifying the manually created Prometheus Route exists")
				Eventually(prometheusRoute, "30s", "5s").Should(k8sFixture.ExistByName())

				By("triggering reconciliation by updating ArgoCD")
				argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
					if ac.Annotations == nil {
						ac.Annotations = make(map[string]string)
					}
					ac.Annotations["test.trigger"] = "reconcile-route"
				})

				By("verifying the Prometheus Route is deleted by the operator")
				Eventually(prometheusRoute, "2m", "5s").Should(k8sFixture.NotExistByName())
			}

			By("verifying ServiceMonitors are created")
			metricsServiceMonitor := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-metrics",
					Namespace: ns.Name,
				},
			}
			Eventually(metricsServiceMonitor, "2m", "5s").Should(k8sFixture.ExistByName())
		})
	})
})
