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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	statefulsetFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-122_validate_argocd_reconciliation_timeout environment variable", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("should set ARGOCD_RECONCILIATION_TIMEOUT with appSync value", func() {

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating ArgoCD CR")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						AppSync: &metav1.Duration{Duration: time.Minute * 10},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			ss := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-application-controller",
					Namespace: ns.Name,
				},
			}
			Eventually(ss).Should(k8sFixture.ExistByName())
			Eventually(ss).Should(statefulsetFixture.HaveReplicas(1))
			Eventually(ss).Should(statefulsetFixture.HaveReadyReplicas(1))

			By("verifying env var is added to argocd-application-controller, and that other env vars are still present")
			Eventually(ss).Should(statefulsetFixture.HaveContainerWithEnvVar("ARGOCD_RECONCILIATION_TIMEOUT", "600s", 0))

			Expect(len(ss.Spec.Template.Spec.Containers[0].Env)).To(BeNumerically(">", 1))

			By("Updating appSync to 5 minutes")
			argoCD.Spec.Controller.AppSync = &metav1.Duration{Duration: time.Minute * 5}
			Expect(k8sClient.Update(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(ss).Should(statefulsetFixture.HaveContainerWithEnvVar("ARGOCD_RECONCILIATION_TIMEOUT", "300s", 0))

		})

		It("should set ARGOCD_RECONCILIATION_TIMEOUT with timeout.reconciliation value in configmap", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating ArgoCD CR with timeout.reconciliation value in configmap")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					ExtraConfig: map[string]string{"timeout.reconciliation": "10m"},
				},
			}

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Fetching ConfigMap and verifying timeout.reconciliation value")
			argocdCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-cm", Namespace: ns.Name},
			}
			Eventually(argocdCM).Should(configmapFixture.HaveStringDataKeyValue("timeout.reconciliation", "10m"))

			By("waiting for StatefulSet to be ready")
			ss := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-application-controller",
					Namespace: ns.Name,
				},
			}
			Eventually(ss).Should(k8sFixture.ExistByName())
			Eventually(ss).Should(statefulsetFixture.HaveReplicas(1))
			Eventually(ss).Should(statefulsetFixture.HaveReadyReplicas(1))

			By("verifying env var is added to example-argocd-application-controller, and that other env vars are still present")
			Eventually(ss).Should(statefulsetFixture.HaveContainerWithEnvVarFromConfigMap(
				"ARGOCD_RECONCILIATION_TIMEOUT",
				"argocd-cm",
				"timeout.reconciliation",
				0,
			))

			By("Updating timeout.reconciliation to 5 minutes and waiting for ArgoCD CR to be reconciled and the instance to be ready")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ExtraConfig["timeout.reconciliation"] = "5m"
			})
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			Eventually(argocdCM).Should(configmapFixture.HaveStringDataKeyValue("timeout.reconciliation", "5m"))

			By("waiting for StatefulSet to be ready after restart")
			statefulsetFixture.Restart(ss)
			Eventually(ss, "3m", "5s").Should(statefulsetFixture.HaveReadyReplicas(1))
			ss = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-application-controller",
					Namespace: ns.Name,
				},
			}
			Eventually(ss).Should(statefulsetFixture.HaveContainerWithEnvVarFromConfigMap(
				"ARGOCD_RECONCILIATION_TIMEOUT",
				"argocd-cm",
				"timeout.reconciliation",
				0,
			))
		})

		It("Validate the precedence of the appSync value over the timeout.reconciliation value in configmap", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating ArgoCD CR with appSync value and timeout.reconciliation value in configmap")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						AppSync: &metav1.Duration{Duration: time.Minute * 10},
					},
					ExtraConfig: map[string]string{"timeout.reconciliation": "15m"},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			ss := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-application-controller",
					Namespace: ns.Name,
				},
			}
			Eventually(ss).Should(k8sFixture.ExistByName())
			Eventually(ss).Should(statefulsetFixture.HaveReadyReplicas(1))

			By("verifying env var is added to example-argocd-application-controller and the appSync value is used")
			Eventually(ss).Should(statefulsetFixture.HaveContainerWithEnvVar("ARGOCD_RECONCILIATION_TIMEOUT", "600s", 0))

			By("Removing appSync value and waiting for ArgoCD CR to be reconciled and the instance to be ready")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Controller.AppSync = nil
			})
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			statefulsetFixture.Restart(ss)
			Eventually(ss, "3m", "5s").Should(statefulsetFixture.HaveReadyReplicas(1))
			By("Fetching ConfigMap and verifying timeout.reconciliation value")
			argocdCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-cm", Namespace: ns.Name},
			}
			Eventually(argocdCM).Should(configmapFixture.HaveStringDataKeyValue("timeout.reconciliation", "15m"))
			By("verifying env var is added to example-argocd-application-controller and the timeout.reconciliation value is used")
			Eventually(ss).Should(statefulsetFixture.HaveContainerWithEnvVarFromConfigMap(
				"ARGOCD_RECONCILIATION_TIMEOUT",
				"argocd-cm",
				"timeout.reconciliation",
				0,
			))

		})
	})
})
