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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1alpha1api "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	statefulsetFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/statefulset"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-001_validate_basic", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("tests a variety of basic Argo CD functionality", func() {

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("1) creating simple namespace-scoped Argo CD instance")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name,
					Labels: map[string]string{"example": "basic"}},
				Spec: argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			Eventually(argoCD, "30s", "5s").Should(argocdFixture.HaveCondition(metav1.Condition{
				Message: "",
				Reason:  "Success",
				Status:  "True",
				Type:    "Reconciled",
			}))

			By("verifying that Argo CD Deployments have readReplicas: 1")
			deploymentNameList := []string{"example-argocd-redis", "example-argocd-repo-server", "example-argocd-server"}

			for _, deploymentName := range deploymentNameList {

				depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: ns.Name}}
				Expect(depl).To(k8sFixture.ExistByName())

				Eventually(depl, "2m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))

			}

			By("verifying Argo CD statefulset is ready replicas: 1")
			ss := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-application-controller", Namespace: argoCD.Namespace}}
			Eventually(ss).Should(k8sFixture.ExistByName())
			Eventually(ss).Should(statefulsetFixture.HaveReplicas(1))
			Eventually(ss).Should(statefulsetFixture.HaveReadyReplicas(1))

			serviceAccountsShouldExist := []string{"example-argocd-argocd-application-controller", "example-argocd-argocd-server", "example-argocd-argocd-redis-ha"}

			By("verifying service accounts exist")
			for _, serviceAccountShouldExist := range serviceAccountsShouldExist {
				Eventually(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountShouldExist, Namespace: argoCD.Namespace}}).Should(k8sFixture.ExistByName())
			}

			By("verifying rolebindings exist")
			roleBindingsShouldExist := []string{"example-argocd-argocd-application-controller", "example-argocd-argocd-server", "example-argocd-argocd-redis-ha"}

			for _, roleBindingShouldExist := range roleBindingsShouldExist {
				Eventually(&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: roleBindingShouldExist, Namespace: argoCD.Namespace}}).Should(k8sFixture.ExistByName())
			}

			deleteAndWaitForDeleted := func() {

				By("deleting previous Argo CD instance")
				Expect(k8sClient.Delete(ctx, argoCD)).To(Succeed())

				By("waiting for Argo CD instance to be deleted and resources to be cleaned")
				for _, deploymentName := range deploymentNameList {
					depl := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName, Namespace: ns.Name}}
					Eventually(depl, "2m", "5s").Should(k8sFixture.NotExistByName())
				}

				ss := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-application-controller", Namespace: argoCD.Namespace}}
				Eventually(ss).Should(k8sFixture.NotExistByName())

			}

			deleteAndWaitForDeleted()

			By("2) creating simple namespace-scoped Argo CD instance with argo cd server: grpc enabled and ingress enabled")
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name,
					Labels: map[string]string{"example": "ingress"}},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						GRPC: argov1beta1api.ArgoCDServerGRPCSpec{
							Ingress: argov1beta1api.ArgoCDIngressSpec{
								Enabled: true,
							},
						},
						Ingress: argov1beta1api.ArgoCDIngressSpec{
							Enabled: true,
						},
						Insecure: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			// TODO: It seems suspiciously like this should be a check, not a create
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-server",
					Namespace: ns.Name,
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{{
						Host: "argocd",
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "argocd-server",
												Port: networkingv1.ServiceBackendPort{
													Name: "http",
												},
											},
										},
										Path:     "/",
										PathType: ptr.To(networkingv1.PathTypeImplementationSpecific),
									},
								},
							},
						},
					}},
					TLS: []networkingv1.IngressTLS{
						{
							Hosts:      []string{"argocd"},
							SecretName: "argocd-secret",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, ingress)).To(Succeed())

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(ingress, "1m", "5s").Should(k8sFixture.ExistByName())

			By("verifying GRPC ingress exists")
			grpcIngress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-grpc",
					Namespace: ns.Name,
				},
			}
			Eventually(grpcIngress, "1m", "5s").Should(k8sFixture.ExistByName())

			deleteAndWaitForDeleted()

			By("3) creating simple namespace-scoped Argo CD instance with 'disableAdmin: true'")
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name,
					Labels: map[string]string{"example": "disable-admin"}},
				Spec: argov1beta1api.ArgoCDSpec{
					DisableAdmin: true,
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying ConfigMap argocd-cm exists and has admin.enabled: false")
			argocdCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: ns.Name,
				},
			}
			Eventually(argocdCM).Should(k8sFixture.ExistByName())
			Eventually(argocdCM).Should(configmapFixture.HaveStringDataKeyValue("admin.enabled", "false"))

			deleteAndWaitForDeleted()

			By("4) creating simple namespace-scoped Argo CD instance")
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name,
					Labels: map[string]string{"example": "export"}},
				Spec: argov1beta1api.ArgoCDSpec{},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("creating ArgoCDExport to export the Argo CD contents")
			argoCDExport := &argov1alpha1api.ArgoCDExport{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocdexport",
					Namespace: ns.Name,
					Labels: map[string]string{
						"example": "basic",
					},
				},
				Spec: argov1alpha1api.ArgoCDExportSpec{
					Argocd: "example-argocd",
				},
			}
			Expect(k8sClient.Create(ctx, argoCDExport)).To(Succeed())

			By("verifing the export Job exists and succeeded")
			exportJob := &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocdexport",
					Namespace: ns.Name,
				},
			}
			Eventually(exportJob).Should(k8sFixture.ExistByName())
			Eventually(func() bool {

				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(exportJob), exportJob); err != nil {
					GinkgoWriter.Println("unable to get job")
					return false
				}
				GinkgoWriter.Println("Job succeeded status:", exportJob.Status.Succeeded)

				return exportJob.Status.Succeeded == 1
			}, "3m", "5s").Should(BeTrue())

			deleteAndWaitForDeleted()

			By("5) By creating Argo CD instance with application set enabled, with an ingress")
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						WebhookServer: argov1beta1api.WebhookServerSpec{
							Ingress: argov1beta1api.ArgoCDIngressSpec{
								Enabled: true,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying applicationset resources all exist")

			appsetDepl := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-applicationset-controller", Namespace: ns.Name},
			}
			Eventually(appsetDepl, "2m", "5s").Should(deploymentFixture.HaveReadyReplicas(1))

			appsetSA := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-applicationset-controller", Namespace: ns.Name},
			}
			Eventually(appsetSA).Should(k8sFixture.ExistByName())

			appsetRB := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-applicationset-controller", Namespace: ns.Name},
			}
			Eventually(appsetRB).Should(k8sFixture.ExistByName())

			appsetRole := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-applicationset-controller", Namespace: ns.Name},
			}
			Eventually(appsetRole).Should(k8sFixture.ExistByName())

			ingress = &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example-argocd-applicationset-controller",
					Namespace: ns.Name,
				},
			}
			Eventually(ingress).Should(k8sFixture.ExistByName())

			By("verifying application set webhook server ingress has correct value")
			Eventually(func() bool {

				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(ingress), ingress); err != nil {
					GinkgoWriter.Println("unable to get ingress", err)
					return false
				}

				return reflect.DeepEqual(ingress.Spec.Rules, []networkingv1.IngressRule{{
					Host: "example-argocd",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "example-argocd-applicationset-controller",
											Port: networkingv1.ServiceBackendPort{
												Name: "webhook",
											},
										},
									},
									Path:     "/api/webhook",
									PathType: ptr.To(networkingv1.PathTypeImplementationSpecific),
								},
							},
						},
					},
				}})

			}, "3m", "5s").Should(BeTrue())

			By("disabling application set webhook ingress")
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ApplicationSet.WebhookServer.Ingress.Enabled = false
			})

			By("verifying the ingress is deleted")
			Eventually(ingress).Should(k8sFixture.NotExistByName())

		})

	})
})
