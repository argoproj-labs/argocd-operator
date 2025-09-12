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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-043_validate_log_level_format", func() {

		var (
			k8sClient   client.Client
			ctx         context.Context
			ns          *corev1.Namespace
			cleanupFunc func()
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		AfterEach(func() {
			fixture.OutputDebugOnFail(ns)

			if cleanupFunc != nil {
				cleanupFunc()
			}

		})

		It("verifies ArgoCD .spec loglevel/logformat fields are set on corresponding Deployment/StatefulSet", func() {

			By("creating simple namespace-scoped Argo CD instance")
			ns, cleanupFunc = fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("enabling debug loglevel and json logformat on various Argo CD workloads")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.LogLevel = "debug"
				ac.Spec.Server.LogFormat = "json"
				ac.Spec.Repo.LogLevel = "debug"
				ac.Spec.Repo.LogFormat = "json"
				ac.Spec.Controller.LogLevel = "debug"
				ac.Spec.Controller.LogFormat = "json"
			})

			// Ensure the given PodTemplate has the expected loglevel/logformat settings
			podTemplateHasLogSettings := func(name string, template corev1.PodTemplateSpec) bool {

				container := template.Spec.Containers[0]

				containerCommand := strings.Join(container.Command, " ")

				GinkgoWriter.Println(name, "has container command:", containerCommand)
				return strings.Contains(containerCommand, "--loglevel debug") && strings.Contains(containerCommand, "--logformat json")
			}

			deployments := []string{"argocd-server", "argocd-repo-server"}
			for _, deployment := range deployments {
				depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deployment, Namespace: ns.Name}}

				By("verifying the Deployment " + depl.Name + " has expected log settings")

				Eventually(func() bool {
					if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(depl), depl); err != nil {
						GinkgoWriter.Println(err)
						return false
					}
					return podTemplateHasLogSettings(depl.Name, depl.Spec.Template)
				}, "60s", "1s").Should(BeTrue())
			}

			statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "argocd-application-controller", Namespace: ns.Name}}
			By("verifying the StatefulSet " + statefulSet.Name + " has expected log settings")
			Eventually(statefulSet).Should(k8sFixture.ExistByName())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				return podTemplateHasLogSettings(statefulSet.Name, statefulSet.Spec.Template)
			}).Should(BeTrue())

		})
	})
})
