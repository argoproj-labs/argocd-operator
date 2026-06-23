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
	_ "embed"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	certFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/certificate"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	routev1 "github.com/openshift/api/route/v1"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-125_validate_server_serving_cert_annotation_restore", func() {

		var (
			ctx       context.Context
			k8sClient client.Client
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("restores service.beta.openshift.io/serving-cert-secret-name when Route TLS returns from passthrough to default reencrypt", func() {

			fixture.EnsureRunningOnOpenShift()

			ns, nsCleanup := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer nsCleanup()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example",
					Namespace: ns.Name,
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD, "3m", "5s").Should(argocdFixture.HaveServerStatus("Running"))

			serverSvc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-server",
					Namespace: ns.Name,
				},
			}
			Eventually(serverSvc, "3m", "5s").Should(k8sFixture.ExistByName())
			Eventually(serverSvc, "3m", "5s").Should(
				k8sFixture.HaveAnnotationWithValue(common.AnnotationOpenShiftServiceCA, common.ArgoCDServerTLSSecretName))

			By("setting Route TLS termination to passthrough so AutoTLS is disabled and the serving-cert annotation is removed")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Route.TLS = &routev1.TLSConfig{
					Termination: routev1.TLSTerminationPassthrough,
				}
			})

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverSvc), serverSvc); err != nil {
					GinkgoWriter.Println(err)
					return false
				}
				if serverSvc.Annotations == nil {
					return true
				}
				_, present := serverSvc.Annotations[common.AnnotationOpenShiftServiceCA]
				return !present
			}, "3m", "5s").Should(BeTrue(), "serving-cert annotation should be removed under passthrough")

			By("clearing Route TLS so the operator defaults to reencrypt again")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Server.Route.TLS = nil
			})

			By("verifying the serving-cert annotation is restored")
			Eventually(serverSvc, "3m", "5s").Should(
				k8sFixture.HaveAnnotationWithValue(common.AnnotationOpenShiftServiceCA, common.ArgoCDServerTLSSecretName))

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD, "3m", "5s").Should(argocdFixture.HaveServerStatus("Running"))

			By("simulating a stale Service after upgrade: remove serving-cert annotation only")

			k8sFixture.Update(serverSvc, func(obj client.Object) {
				s := obj.(*corev1.Service)
				if s.Annotations != nil {
					delete(s.Annotations, common.AnnotationOpenShiftServiceCA)
				}
			})

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverSvc), serverSvc); err != nil {
					GinkgoWriter.Println(err)
					return false
				}
				if serverSvc.Annotations == nil {
					return true
				}
				_, present := serverSvc.Annotations[common.AnnotationOpenShiftServiceCA]
				return !present
			}, "1m", "2s").Should(BeTrue())

			By("triggering reconciliation via a no-op ArgoCD metadata change")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				if ac.Annotations == nil {
					ac.Annotations = map[string]string{}
				}
				ac.Annotations["argocds.argoproj.io/e2e-serving-cert-reconcile-touch"] = "1"
			})

			Eventually(serverSvc, "3m", "5s").Should(
				k8sFixture.HaveAnnotationWithValue(common.AnnotationOpenShiftServiceCA, common.ArgoCDServerTLSSecretName))

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD, "3m", "5s").Should(argocdFixture.HaveServerStatus("Running"))

		})

		It("does not add serving-cert annotation when argocd-server-tls already exists as a user-managed secret (not Service CA)", func() {
			certPem, keyPem, err := certFixture.GenerateCert()
			Expect(err).NotTo(HaveOccurred())

			fixture.EnsureRunningOnOpenShift()

			ns, nsCleanup := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer nsCleanup()

			By("pre-creating argocd-server-tls without OpenShift Service CA annotations or ownerReferences")

			customTLS := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      common.ArgoCDServerTLSSecretName,
					Namespace: ns.Name,
				},
				Type: corev1.SecretTypeTLS,
				StringData: map[string]string{
					"tls.crt": string(certPem),
					"tls.key": string(keyPem),
				},
			}
			Expect(k8sClient.Create(ctx, customTLS)).To(Succeed())

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							TLS: &routev1.TLSConfig{
								Termination: routev1.TLSTerminationReencrypt,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD, "3m", "5s").Should(argocdFixture.HaveServerStatus("Running"))

			serverSvc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-server",
					Namespace: ns.Name,
				},
			}
			Eventually(serverSvc, "3m", "5s").Should(k8sFixture.ExistByName())

			By("verifying the server Service never gets the OpenShift serving-cert annotation")

			Consistently(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverSvc), serverSvc); err != nil {
					GinkgoWriter.Println(err)
					return false
				}
				if serverSvc.Annotations == nil {
					return true
				}
				_, present := serverSvc.Annotations[common.AnnotationOpenShiftServiceCA]
				return !present
			}, "2m", "5s").Should(BeTrue())

			By("verifying annotation stays absent after an ArgoCD reconcile trigger")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				if ac.Annotations == nil {
					ac.Annotations = map[string]string{}
				}
				ac.Annotations["argocds.argoproj.io/e2e-custom-tls-no-svc-ca-touch"] = "1"
			})

			Consistently(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverSvc), serverSvc); err != nil {
					GinkgoWriter.Println(err)
					return false
				}
				if serverSvc.Annotations == nil {
					return true
				}
				_, present := serverSvc.Annotations[common.AnnotationOpenShiftServiceCA]
				return !present
			}, "2m", "5s").Should(BeTrue())
		})
	})
})
