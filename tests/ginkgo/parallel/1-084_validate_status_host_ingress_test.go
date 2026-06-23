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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-084_validate_status_host_ingress", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that when Argo CD Server is exposed via an Ingress, that the ingress is created and ArgoCD CR has the correct status information", func() {

			// This test supersedes '1-002_verify_hostname_with_ingress'

			By("creating simple Argo CD instance with API Server exposed via ingress")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Host: "test-crane.apps.rh-4.12-111111.dev.openshift.org",
						GRPC: argov1beta1api.ArgoCDServerGRPCSpec{
							Ingress: argov1beta1api.ArgoCDIngressSpec{
								Enabled: true,
							},
						},
						Ingress: argov1beta1api.ArgoCDIngressSpec{
							Enabled: true,
							TLS:     []networkingv1.IngressTLS{{Hosts: []string{"test-crane"}}},
						},
					},
				}}

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying Ingress is created")
			serverIngress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-server",
					Namespace: ns.Name,
				},
			}
			Eventually(serverIngress).Should(k8sFixture.ExistByName())

			if fixture.RunningOnOpenShift() {
				By("verifying ArgoCD CR .status references the host from the ArgoCD CR .spec")
				Eventually(argoCD).Should(argocdFixture.HaveHost("test-crane.apps.rh-4.12-111111.dev.openshift.org"))
			}

		})

	})
})
