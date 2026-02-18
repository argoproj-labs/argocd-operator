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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-124_validate_networkpolicies", func() {

		var (
			ctx       context.Context
			k8sClient client.Client
			nsCleanup func()
			ns        *metav1.PartialObjectMetadata
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			if nsCleanup != nil {
				nsCleanup()
			}
		})

		It("creates expected NetworkPolicies with correct pod selectors and ingress rules", func() {

			By("creating namespace-scoped Argo CD with notifications, dex, and applicationset enabled")
			nsObj, cleanup := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			nsCleanup = cleanup

			// Track namespace for debug (same pattern as other tests)
			ns = &metav1.PartialObjectMetadata{}
			ns.SetName(nsObj.Name)

			argocd := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: nsObj.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
					Notifications: argov1beta1api.ArgoCDNotifications{
						Enabled: true,
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(true),
					},
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeDex,
						Dex: &argov1beta1api.ArgoCDDexSpec{
							Config: "test-config",
							Volumes: []corev1.Volume{
								{Name: "empty-dir-volume", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "empty-dir-volume", MountPath: "/etc/test"},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argocd)).To(Succeed())

			By("waiting for Argo CD to become available")
			Eventually(argocd, "6m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying NetworkPolicies exist")
			expectedNPs := []string{
				"example-argocd-redis-network-policy",
				"example-argocd-redis-ha-network-policy",
				"example-argocd-notifications-controller-network-policy",
				"example-argocd-dex-server-network-policy",
				"example-argocd-applicationset-controller-network-policy",
				"example-argocd-server-network-policy",
				"example-argocd-application-controller-network-policy",
				"example-argocd-repo-server-network-policy",
			}
			for _, npName := range expectedNPs {
				Eventually(&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: npName, Namespace: nsObj.Name}}, "3m", "5s").Should(k8sFixture.ExistByName())
			}

			By("verifying repo-server NetworkPolicy ingress peers/ports")
			repoNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-repo-server-network-policy", Namespace: nsObj.Name}}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoNP), repoNP); err != nil {
					return false
				}
				if repoNP.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName] != "example-argocd-repo-server" {
					return false
				}
				if len(repoNP.Spec.Ingress) != 2 {
					return false
				}
				// Rule 1: internal components on 8081
				if len(repoNP.Spec.Ingress[0].Ports) != 1 || repoNP.Spec.Ingress[0].Ports[0].Port == nil || repoNP.Spec.Ingress[0].Ports[0].Port.IntVal != 8081 {
					return false
				}
				// Expect applicationset fixed label to be present
				foundAppSet := false
				for _, peer := range repoNP.Spec.Ingress[0].From {
					if peer.PodSelector != nil && peer.PodSelector.MatchLabels != nil && peer.PodSelector.MatchLabels[common.ArgoCDKeyName] == "argocd-applicationset-controller" {
						foundAppSet = true
					}
				}
				if !foundAppSet {
					return false
				}
				// Rule 2: any namespace on 8084 (metrics)
				if repoNP.Spec.Ingress[1].From == nil || len(repoNP.Spec.Ingress[1].From) != 1 || repoNP.Spec.Ingress[1].From[0].NamespaceSelector == nil {
					return false
				}
				if len(repoNP.Spec.Ingress[1].Ports) != 1 || repoNP.Spec.Ingress[1].Ports[0].Port == nil || repoNP.Spec.Ingress[1].Ports[0].Port.IntVal != 8084 {
					return false
				}
				return true
			}, "3m", "5s").Should(BeTrue())

			By("verifying dex-server NetworkPolicy ports")
			dexNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-dex-server-network-policy", Namespace: nsObj.Name}}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(dexNP), dexNP); err != nil {
					return false
				}
				if dexNP.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName] != "example-argocd-dex-server" {
					return false
				}
				if len(dexNP.Spec.Ingress) != 2 {
					return false
				}
				if len(dexNP.Spec.Ingress[0].Ports) != 2 {
					return false
				}
				if dexNP.Spec.Ingress[0].Ports[0].Port == nil || dexNP.Spec.Ingress[0].Ports[0].Port.IntVal != common.ArgoCDDefaultDexHTTPPort {
					return false
				}
				if dexNP.Spec.Ingress[0].Ports[1].Port == nil || dexNP.Spec.Ingress[0].Ports[1].Port.IntVal != common.ArgoCDDefaultDexGRPCPort {
					return false
				}
				if len(dexNP.Spec.Ingress[1].Ports) != 1 || dexNP.Spec.Ingress[1].Ports[0].Port == nil || dexNP.Spec.Ingress[1].Ports[0].Port.IntVal != common.ArgoCDDefaultDexMetricsPort {
					return false
				}
				return true
			}, "3m", "5s").Should(BeTrue())

			By("verifying notifications-controller NetworkPolicy ports")
			notifNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-notifications-controller-network-policy", Namespace: nsObj.Name}}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(notifNP), notifNP); err != nil {
					return false
				}
				if notifNP.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName] != "example-argocd-notifications-controller" {
					return false
				}
				if len(notifNP.Spec.Ingress) != 1 {
					return false
				}
				if notifNP.Spec.Ingress[0].From == nil || len(notifNP.Spec.Ingress[0].From) != 1 || notifNP.Spec.Ingress[0].From[0].NamespaceSelector == nil {
					return false
				}
				if len(notifNP.Spec.Ingress[0].Ports) != 1 || notifNP.Spec.Ingress[0].Ports[0].Port == nil || notifNP.Spec.Ingress[0].Ports[0].Port.IntVal != 9001 {
					return false
				}
				return true
			}, "3m", "5s").Should(BeTrue())

			By("verifying applicationset-controller NetworkPolicy ports")
			appSetNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-applicationset-controller-network-policy", Namespace: nsObj.Name}}
			expectedAppSetSelectorLabel := "argocd-applicationset-controller"
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appSetNP), appSetNP); err != nil {
					return false
				}
				if appSetNP.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName] != expectedAppSetSelectorLabel {
					return false
				}
				if len(appSetNP.Spec.Ingress) != 1 {
					return false
				}
				if appSetNP.Spec.Ingress[0].From == nil || len(appSetNP.Spec.Ingress[0].From) != 1 || appSetNP.Spec.Ingress[0].From[0].NamespaceSelector == nil {
					return false
				}
				if len(appSetNP.Spec.Ingress[0].Ports) != 2 {
					return false
				}
				if appSetNP.Spec.Ingress[0].Ports[0].Port == nil || appSetNP.Spec.Ingress[0].Ports[0].Port.IntVal != 7000 {
					return false
				}
				if appSetNP.Spec.Ingress[0].Ports[1].Port == nil || appSetNP.Spec.Ingress[0].Ports[1].Port.IntVal != 8080 {
					return false
				}
				return true
			}, "3m", "5s").Should(BeTrue())

			By("verifying argocd-server NetworkPolicy shape")
			serverNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-server-network-policy", Namespace: nsObj.Name}}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverNP), serverNP); err != nil {
					return false
				}
				if serverNP.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName] != "example-argocd-server" {
					return false
				}
				if len(serverNP.Spec.Ingress) != 1 {
					return false
				}
				// empty ingress rule (allow all)
				if len(serverNP.Spec.Ingress[0].From) != 0 || len(serverNP.Spec.Ingress[0].Ports) != 0 {
					return false
				}
				return true
			}, "3m", "5s").Should(BeTrue())

			By("verifying application-controller NetworkPolicy ports")
			appControllerNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-application-controller-network-policy", Namespace: nsObj.Name}}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(appControllerNP), appControllerNP); err != nil {
					return false
				}
				if appControllerNP.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName] != "example-argocd-application-controller" {
					return false
				}
				if len(appControllerNP.Spec.Ingress) != 1 {
					return false
				}
				if appControllerNP.Spec.Ingress[0].From == nil || len(appControllerNP.Spec.Ingress[0].From) != 1 || appControllerNP.Spec.Ingress[0].From[0].NamespaceSelector == nil {
					return false
				}
				if len(appControllerNP.Spec.Ingress[0].Ports) != 1 || appControllerNP.Spec.Ingress[0].Ports[0].Port == nil || appControllerNP.Spec.Ingress[0].Ports[0].Port.IntVal != 8082 {
					return false
				}
				return true
			}, "3m", "5s").Should(BeTrue())

			By("verifying redis NetworkPolicy ports")
			redisNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-redis-network-policy", Namespace: nsObj.Name}}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(redisNP), redisNP); err != nil {
					return false
				}
				if redisNP.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName] != "example-argocd-redis" {
					return false
				}
				if len(redisNP.Spec.Ingress) != 1 {
					return false
				}
				if len(redisNP.Spec.Ingress[0].Ports) != 1 || redisNP.Spec.Ingress[0].Ports[0].Port == nil || redisNP.Spec.Ingress[0].Ports[0].Port.IntVal != 6379 {
					return false
				}
				return true
			}, "3m", "5s").Should(BeTrue())

			By("verifying redis-ha NetworkPolicy ports")
			redisHANP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-redis-ha-network-policy", Namespace: nsObj.Name}}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(redisHANP), redisHANP); err != nil {
					return false
				}
				if redisHANP.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName] != "example-argocd-redis-ha-haproxy" {
					return false
				}
				if len(redisHANP.Spec.Ingress) != 1 {
					return false
				}
				if len(redisHANP.Spec.Ingress[0].Ports) != 2 {
					return false
				}
				if redisHANP.Spec.Ingress[0].Ports[0].Port == nil || redisHANP.Spec.Ingress[0].Ports[0].Port.IntVal != 6379 {
					return false
				}
				if redisHANP.Spec.Ingress[0].Ports[1].Port == nil || redisHANP.Spec.Ingress[0].Ports[1].Port.IntVal != 26379 {
					return false
				}
				return true
			}, "3m", "5s").Should(BeTrue())
		})

		It("reconciles drifted NetworkPolicy and respects disabling networkPolicy.enabled", func() {
			By("creating namespace-scoped Argo CD instance")
			nsObj, cleanup := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanup()

			argocd := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: nsObj.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argocd)).To(Succeed())
			Eventually(argocd, "6m", "5s").Should(argocdFixture.BeAvailable())

			By("waiting for repo-server NetworkPolicy to exist")
			repoNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-repo-server-network-policy", Namespace: nsObj.Name}}
			Eventually(repoNP, "3m", "5s").Should(k8sFixture.ExistByName())

			By("introducing drift into the repo-server NetworkPolicy")
			k8sFixture.Update(repoNP, func(obj client.Object) {
				np := obj.(*networkingv1.NetworkPolicy)
				if np.Spec.PodSelector.MatchLabels == nil {
					np.Spec.PodSelector.MatchLabels = map[string]string{}
				}
				np.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName] = "wrong"
			})

			By("verifying the operator reconciles NetworkPolicy drift")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(repoNP), repoNP); err != nil {
					return false
				}
				return repoNP.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName] == "example-argocd-repo-server"
			}, "3m", "5s").Should(BeTrue())

			By("disabling networkPolicy.enabled")
			disabled := false
			argocdFixture.Update(argocd, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.NetworkPolicy.Enabled = &disabled
			})

			By("verifying NetworkPolicies are deleted and not recreated while disabled")
			coreNPs := []string{
				"example-argocd-repo-server-network-policy",
				"example-argocd-server-network-policy",
				"example-argocd-application-controller-network-policy",
			}
			for _, npName := range coreNPs {
				Eventually(&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: npName, Namespace: nsObj.Name}}, "3m", "5s").Should(k8sFixture.NotExistByName())
				Consistently(&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: npName, Namespace: nsObj.Name}}, "30s", "5s").Should(k8sFixture.NotExistByName())
			}
		})
	})
})
