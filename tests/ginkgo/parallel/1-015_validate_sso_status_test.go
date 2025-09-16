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
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-015_validate_sso_status", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()

			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that .status field of ArgoCD reflects the expected status based on SSO configuration, and verifies that expected dex resources are created when dex is enabled", func() {

			By("creating simple namespace-scoped Argo CD instance")
			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec:       argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready, but SSO should be unknown because we haven't defined SSO")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.HaveSSOStatus("Unknown"))

			By("updating SSO provider to dex, but not providing any configuration details")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.SSO = &argov1beta1api.ArgoCDSSOSpec{
					Provider: argov1beta1api.SSOProviderTypeDex,
				}
			})

			By("verifying Argo CD is pending and SSO is failed, because we have not provided configuration details")
			Eventually(argoCD).Should(argocdFixture.HavePhase("Failed"))
			Consistently(argoCD, "15s", "1s").Should(argocdFixture.HavePhase("Failed"))
			Eventually(argoCD).Should(argocdFixture.HaveSSOStatus("Failed"))

			By("verifying dex is not ready for same reason")
			depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-dex-server", Namespace: ns.Name}}
			Consistently(depl).ShouldNot(deployment.HaveReadyReplicas(1))

			By("adding a simple placeholder configuration dex config field")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.SSO.Dex = &argov1beta1api.ArgoCDDexSpec{
					Config: "test-config",
				}
			})

			By("verifying Dex and Argo CD start as expected, and are marked as Available and Running, now that we have added a test configuration")
			Eventually(argoCD, "3m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD).Should(argocdFixture.HaveSSOStatus("Running"))

			By("verifying dex Deployment is now running")
			depl = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "argocd-dex-server", Namespace: ns.Name}}
			Eventually(depl).Should(k8sFixture.ExistByName())
			Eventually(depl).Should(deployment.HaveReadyReplicas(1))

			By("verifying other Dex-related resources have been created")
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-argocd-dex-server",
					Namespace: ns.Name,
				},
			}
			Eventually(sa).Should(k8sFixture.ExistByName())

			rb := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-argocd-dex-server",
					Namespace: ns.Name,
				},
			}
			Eventually(rb).Should(k8sFixture.ExistByName())

			r := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-argocd-dex-server",
					Namespace: ns.Name,
				},
			}
			Eventually(r).Should(k8sFixture.ExistByName())

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-dex-server",
					Namespace: ns.Name,
				},
			}
			Eventually(service).Should(k8sFixture.ExistByName())

			By("removing SSO entirely from ArgoCD CR")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.SSO = nil
			})

			By("verifying ArgoCD status goes back to Unknown SSO status, but Argo CD is still available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.HaveSSOStatus("Unknown"))

			Consistently(argoCD).Should(argocdFixture.HaveSSOStatus("Unknown"))
			Consistently(argoCD).Should(argocdFixture.BeAvailable())

			By("verifying the various dex resources have been deleted")
			Eventually(depl).ShouldNot(k8sFixture.ExistByName())
			Consistently(depl).ShouldNot(k8sFixture.ExistByName())

			Eventually(sa).ShouldNot(k8sFixture.ExistByName())
			Consistently(sa).ShouldNot(k8sFixture.ExistByName())

			Eventually(rb).ShouldNot(k8sFixture.ExistByName())
			Consistently(rb).ShouldNot(k8sFixture.ExistByName())

			Eventually(r).ShouldNot(k8sFixture.ExistByName())
			Consistently(r).ShouldNot(k8sFixture.ExistByName())

			Eventually(service).ShouldNot(k8sFixture.ExistByName())
			Consistently(service).ShouldNot(k8sFixture.ExistByName())

			By("enabling keycloak provider, but setting dex config configuration")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.SSO = &argov1beta1api.ArgoCDSSOSpec{
					Provider: argov1beta1api.SSOProviderTypeKeycloak,
					Dex: &argov1beta1api.ArgoCDDexSpec{
						Config: "test",
					},
				}
			})

			By("Argo CD should be Failed/Failed, as this is an invalid configuration")
			Eventually(argoCD).Should(argocdFixture.HavePhase("Failed"))
			Eventually(argoCD).Should(argocdFixture.HaveSSOStatus("Failed"))

			By("creating a new SSO secret")

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argo-workflows-sso",
					Namespace: ns.Name,
				},
				StringData: map[string]string{
					"client-id":     "YXJnby13b3JrZmxvd3Mtc3Nv",
					"client-secret": "aGk=",
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("updating ArgoCD CR to Dex with a valid client secret")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec = argov1beta1api.ArgoCDSpec{
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeDex,
						Dex: &argov1beta1api.ArgoCDDexSpec{
							Config: "test-config",
							Env: []corev1.EnvVar{
								{
									Name: "ARGO_WORKFLOWS_SSO_CLIENT_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "argo-workflows-sso",
											},
											Key: "client-secret",
										},
									},
								},
							},
						},
					},
				}
			})

			By("verifying Argo CD becomes available and SSO is running")
			Eventually(argoCD).Should(argocdFixture.HavePhase("Available"))
			Eventually(argoCD).Should(argocdFixture.HaveSSOStatus("Running"))

			Eventually(depl, "2m", "5s").Should(deployment.HaveReadyReplicas(1))

			By("verifying Dex Deployment now has the environment variable reference we defined in the ArgoCD CR")
			Eventually(func() bool {

				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(depl), depl); err != nil {
					GinkgoWriter.Println("unable to get depl", err)
					return false
				}
				temp := deployment.GetTemplateSpecContainerByName("dex", *depl)
				if temp == nil {
					GinkgoWriter.Println("unable to find container by name")
					return false
				}

				if temp.Name != "dex" {
					GinkgoWriter.Println("container does not have expected name")
					return false
				}

				return reflect.DeepEqual(temp.Env, []corev1.EnvVar{
					{
						Name: "ARGO_WORKFLOWS_SSO_CLIENT_SECRET",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "argo-workflows-sso",
								},
								Key: "client-secret",
							},
						},
					}})

			}).Should(BeTrue())

		})

	})
})
