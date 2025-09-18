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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-120_verify_argocd_status_consistency", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()

		})

		It("cycle throughs each component of .status.phase, and ensures that enabling/disabling the components will affect the .status.phase as expected.", func() {
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			type componentEnabler struct {
				// name of the component to be enabled/disabled
				name string
				// disable function disables the component via ArgoCD CR (it should not be running after being set)
				disable func(acd *argov1beta1api.ArgoCD)
				// enables function enables the component via ArgoCD CR: if shouldFail is true, then an error (for example, a bad image) will be injected into component
				enable func(acd *argov1beta1api.ArgoCD, shouldFail bool)
				// verify verifies the component has the expected status
				verify func(acd *argov1beta1api.ArgoCD, expectedStatus string)
			}

			enablers := []componentEnabler{
				{
					name: "redis",
					disable: func(acd *argov1beta1api.ArgoCD) {
						argocdFixture.Update(acd, func(ac *argov1beta1api.ArgoCD) {
							ac.Spec.Redis = argov1beta1api.ArgoCDRedisSpec{Enabled: ptr.To(false)}
						})
					},
					enable: func(acd *argov1beta1api.ArgoCD, shouldFail bool) {
						argocdFixture.Update(acd, func(ac *argov1beta1api.ArgoCD) {
							if shouldFail {
								ac.Spec.Redis = argov1beta1api.ArgoCDRedisSpec{Enabled: ptr.To(true), Image: "quay.io/argoprojlabs/argocd-operator-does-not-exist:latest"}
							} else {
								ac.Spec.Redis = argov1beta1api.ArgoCDRedisSpec{Enabled: ptr.To(true)}
							}
						})
					},
					verify: func(acd *argov1beta1api.ArgoCD, expectedStatus string) {
						Eventually(acd, "3m", "5s").Should(argocdFixture.HaveRedisStatus(expectedStatus))
						Consistently(acd).Should(argocdFixture.HaveRedisStatus(expectedStatus))
					},
				},
				{
					name: "app-controller",
					disable: func(acd *argov1beta1api.ArgoCD) {
						argocdFixture.Update(acd, func(ac *argov1beta1api.ArgoCD) {
							ac.Spec.Controller = argov1beta1api.ArgoCDApplicationControllerSpec{Enabled: ptr.To(false)}
						})
					},
					enable: func(acd *argov1beta1api.ArgoCD, shouldFail bool) {
						argocdFixture.Update(acd, func(ac *argov1beta1api.ArgoCD) {
							if shouldFail {
								ac.Spec.Controller = argov1beta1api.ArgoCDApplicationControllerSpec{Enabled: ptr.To(true), ExtraCommandArgs: []string{"--fake-param"}}
							} else {
								ac.Spec.Controller = argov1beta1api.ArgoCDApplicationControllerSpec{Enabled: ptr.To(true)}
							}
						})
					},
					verify: func(acd *argov1beta1api.ArgoCD, expectedStatus string) {
						Eventually(acd, "3m", "5s").Should(argocdFixture.HaveApplicationControllerStatus(expectedStatus))
						Consistently(acd).Should(argocdFixture.HaveApplicationControllerStatus(expectedStatus))
					},
				},
				{
					name: "repo-server",
					disable: func(acd *argov1beta1api.ArgoCD) {
						argocdFixture.Update(acd, func(ac *argov1beta1api.ArgoCD) {
							ac.Spec.Repo = argov1beta1api.ArgoCDRepoSpec{Enabled: ptr.To(false)}
						})
					},
					enable: func(acd *argov1beta1api.ArgoCD, shouldFail bool) {
						argocdFixture.Update(acd, func(ac *argov1beta1api.ArgoCD) {
							if shouldFail {
								ac.Spec.Repo = argov1beta1api.ArgoCDRepoSpec{Enabled: ptr.To(true), Image: "quay.io/argoprojlabs/argocd-operator-does-not-exist:latest"}
							} else {
								ac.Spec.Repo = argov1beta1api.ArgoCDRepoSpec{Enabled: ptr.To(true)}
							}
						})
					},
					verify: func(acd *argov1beta1api.ArgoCD, expectedStatus string) {
						Eventually(acd, "3m", "5s").Should(argocdFixture.HaveRepoStatus(expectedStatus))
						Consistently(acd).Should(argocdFixture.HaveRepoStatus(expectedStatus))
					},
				},
				{
					name: "server",
					disable: func(acd *argov1beta1api.ArgoCD) {
						argocdFixture.Update(acd, func(ac *argov1beta1api.ArgoCD) {
							ac.Spec.Server = argov1beta1api.ArgoCDServerSpec{Enabled: ptr.To(false)}
						})
					},
					enable: func(acd *argov1beta1api.ArgoCD, shouldFail bool) {
						argocdFixture.Update(acd, func(ac *argov1beta1api.ArgoCD) {
							if shouldFail {
								ac.Spec.Server = argov1beta1api.ArgoCDServerSpec{Enabled: ptr.To(true), ExtraCommandArgs: []string{"--not-a-real-param"}}
							} else {
								ac.Spec.Server = argov1beta1api.ArgoCDServerSpec{Enabled: ptr.To(true)}
							}
						})
					},
					verify: func(acd *argov1beta1api.ArgoCD, expectedStatus string) {
						Eventually(acd, "3m", "5s").Should(argocdFixture.HaveServerStatus(expectedStatus))
						Consistently(acd).Should(argocdFixture.HaveServerStatus(expectedStatus))
					},
				},
				{
					name: "dex-sso",
					disable: func(acd *argov1beta1api.ArgoCD) {
						argocdFixture.Update(acd, func(ac *argov1beta1api.ArgoCD) {
							ac.Spec.SSO = nil
						})
					},
					enable: func(acd *argov1beta1api.ArgoCD, shouldFail bool) {
						argocdFixture.Update(acd, func(ac *argov1beta1api.ArgoCD) {
							if shouldFail {
								ac.Spec.SSO = &argov1beta1api.ArgoCDSSOSpec{
									Provider: argov1beta1api.SSOProviderTypeDex,
									Dex: &argov1beta1api.ArgoCDDexSpec{
										Config: "hi",
										Image:  "quay.io/argoprojlabs/argocd-operator-does-not-exist:latest",
									},
								}
							} else {
								ac.Spec.SSO = &argov1beta1api.ArgoCDSSOSpec{
									Provider: argov1beta1api.SSOProviderTypeDex,
									Dex: &argov1beta1api.ArgoCDDexSpec{
										Config: "hi",
									},
								}
							}
						})
					},
					verify: func(acd *argov1beta1api.ArgoCD, expectedStatus string) {
						Eventually(acd, "3m", "5s").Should(argocdFixture.HaveSSOStatus(expectedStatus))
						Consistently(acd).Should(argocdFixture.HaveSSOStatus(expectedStatus))
					},
				},
			}

			By("creating basic Argo CD instance")

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller:     argov1beta1api.ArgoCDApplicationControllerSpec{},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{},
					Server:         argov1beta1api.ArgoCDServerSpec{},
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeDex,
						Dex: &argov1beta1api.ArgoCDDexSpec{
							Config: "hi",
						},
					},
					Repo:  argov1beta1api.ArgoCDRepoSpec{},
					Redis: argov1beta1api.ArgoCDRedisSpec{},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			for idx, enabler := range enablers {
				By(fmt.Sprintf("%d) verifying %s ", idx, enabler.name))
				Expect(argoCD).Should(k8sFixture.ExistByName())

				By("disable " + enabler.name)
				enabler.disable(argoCD)
				By("verify 'Unknown' " + enabler.name)
				enabler.verify(argoCD, "Unknown")

				By("verifying ArgoCD CR has phase Available")
				Expect(argoCD).Should(argocdFixture.HavePhase("Available"))

				By("enable " + enabler.name + ", setting it to succeed")
				enabler.enable(argoCD, false)
				By("verify 'Running' " + enabler.name)
				enabler.verify(argoCD, "Running")
				By("verifying ArgoCD CR has phase Available")
				Expect(argoCD).Should(argocdFixture.HavePhase("Available"))

				By("disable " + enabler.name)
				enabler.disable(argoCD)
				By("verify 'Unknown' " + enabler.name)
				enabler.verify(argoCD, "Unknown")
				By("verifying ArgoCD CR has phase Available")
				Expect(argoCD).Should(argocdFixture.HavePhase("Available"))

				By("enable " + enabler.name + ", setting it to fail")
				enabler.enable(argoCD, true)
				By("verify 'Pending' " + enabler.name)
				enabler.verify(argoCD, "Pending")
				By("verifying ArgoCD CR has phase Pending")
				Eventually(argoCD).Should(argocdFixture.HavePhase("Pending"))

				By("disable " + enabler.name)
				enabler.disable(argoCD)
				By("verify 'Unknown' " + enabler.name)
				enabler.verify(argoCD, "Unknown")
				By("verifying ArgoCD CR has phase Available")
				Eventually(argoCD).Should(argocdFixture.HavePhase("Available"))

			}

		})

	})
})
