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

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-042_restricted_pss_compliant_test", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
			ns        *corev1.Namespace
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			Expect(ns).ToNot(BeNil())
			defer fixture.DeleteNamespace(ns)

			fixture.OutputDebugOnFail(ns.Name)

		})

		It("verifies that all Argo CD components can run with pod-security enforce, warn, and audit of 'restricted'", func() {

			// Note: Even though this test enables Redis HA, it does NOT require a cluster with multiple nodes.

			By("creating a namespace with pod-security enforce set to restricted")
			ns = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gitops-e2e-test-" + uuid.NewString()[0:13],
					Labels: map[string]string{
						"pod-security.kubernetes.io/enforce":         "restricted",
						"pod-security.kubernetes.io/enforce-version": "latest",
						"pod-security.kubernetes.io/warn":            "restricted",
						"pod-security.kubernetes.io/warn-version":    "latest",
						"pod-security.kubernetes.io/audit":           "restricted",
						"pod-security.kubernetes.io/audit-version":   "latest",
						fixture.E2ETestLabelsKey:                     fixture.E2ETestLabelsValue, // ensures that NS is GC-ed by test fixture
					},
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())

			By("creating an Argo CD instance in that namespace with various Argo CD components enabled")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(true),
					},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled: true,
					},
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeDex,
						Dex: &argov1beta1api.ArgoCDDexSpec{
							OpenShiftOAuth: true,
						},
					},
				},
			}

			if !fixture.RunningOnOpenShift() {
				argoCD.Spec.Server = argov1beta1api.ArgoCDServerSpec{
					Ingress: argov1beta1api.ArgoCDIngressSpec{Enabled: true},
				}
			}

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying all Argo CD components start as expected under restricted pod security")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD).Should(argocdFixture.HaveApplicationControllerStatus("Running"))
			Eventually(argoCD).Should(argocdFixture.HaveApplicationSetControllerStatus("Running"))
			Eventually(argoCD).Should(argocdFixture.HaveNotificationControllerStatus("Running"))
			Eventually(argoCD).Should(argocdFixture.HaveRedisStatus("Running"))
			Eventually(argoCD).Should(argocdFixture.HaveRepoStatus("Running"))
			Eventually(argoCD).Should(argocdFixture.HaveServerStatus("Running"))
			Eventually(argoCD).Should(argocdFixture.HaveSSOStatus("Running"))

			By("verifying all Argo CD component pods are created as expected, under restricted pod security")
			podsToEnsureExist := []string{
				"argocd-application-controller",
				"argocd-applicationset-controller",
				"argocd-dex-server",
				"argocd-notifications-controller",
				"argocd-redis",
				"argocd-repo-server",
				"argocd-server",
			}

			ensurePodsExistByName := func(podsToEnsureExist []string) {
				var podList corev1.PodList
				Expect(k8sClient.List(ctx, &podList, &client.ListOptions{Namespace: argoCD.Namespace})).To(Succeed())

				for _, podToEnsureExists := range podsToEnsureExist {

					match := false

					for _, podInPodList := range podList.Items {

						if strings.Contains(podInPodList.Name, podToEnsureExists) {
							match = true
						}
					}
					Expect(match).To(BeTrue(), "unable to find match for "+podToEnsureExists)
				}
			}

			ensurePodsExistByName(podsToEnsureExist)

			By("enabling Redis HA")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				argoCD.Spec.HA = argov1beta1api.ArgoCDHASpec{
					Enabled: true,
				}
			})

			if !fixture.RunningOnOpenShift() {
				By("removing SSO")
				argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
					argoCD.Spec.SSO = nil
				})

			}

			By("ensuring that various Argo CD components become available under restricted security")
			Eventually(argoCD).Should(argocdFixture.HaveApplicationControllerStatus("Running"))
			Eventually(argoCD).Should(argocdFixture.HaveServerStatus("Running"))
			Eventually(argoCD).Should(argocdFixture.HaveRepoStatus("Running"))

			By("ensuring that redis HA Deployment and StatefulSet exist, as well as their Pods")
			redisHAProxyDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-redis-ha-haproxy",
					Namespace: ns.Name,
				},
			}
			Eventually(redisHAProxyDepl, "10m", "5s").Should(k8sFixture.ExistByName())

			redisHAServer := &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-redis-ha-server",
					Namespace: ns.Name,
				},
			}
			Eventually(redisHAServer, "10m", "5s").Should(k8sFixture.ExistByName())

			ensurePodsExistByName([]string{"argocd-redis-ha-haproxy", "argocd-redis-ha-server"})

		})

	})
})
