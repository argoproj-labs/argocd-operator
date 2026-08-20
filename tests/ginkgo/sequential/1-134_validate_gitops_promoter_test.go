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

package sequential

import (
	"context"
	"fmt"
	"reflect"

	promoter "github.com/argoproj-labs/gitops-promoter/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argoCDFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	promoterFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/promoter"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

const (
	promoterControllerResourceName              = "test-promoter-controller-manager"
	promoterAPIServerResourceName               = "test-promoter-apiserver"
	promoterControllerResourceNameWithNamespace = "test-%s-promoter-controller-manager"
	promoterAPIServerResourceNameWithNamespace  = "test-%s-promoter-apiserver"
	apiServerTLSSecretName                      = "test-tls-secret"
	apiServerCABundleSecretName                 = "test-cabundle-secret"
	apiServerCABundleSecretKey                  = "ca.crt"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {
	Context("1-134_validate_gitops_promoter", func() {
		const (
			argoCDName = "test"
		)

		var (
			k8sClient   client.Client
			ctx         context.Context
			argoCD      *argov1beta1api.ArgoCD
			ns          *corev1.Namespace
			cleanupFunc func()

			controllerServiceAccount          *corev1.ServiceAccount
			controllerDeployment              *appsv1.Deployment
			controllerService                 *corev1.Service
			controllerConfig                  *promoter.ControllerConfiguration
			controllerClusterRoleBindingNames []string
			controllerClusterRoleNames        []string

			apiServerServiceAccount          *corev1.ServiceAccount
			apiServerDeployment              *appsv1.Deployment
			apiServerService                 *corev1.Service
			apiServerClusterRoleBindingNames []string
			apiServerClusterRoleNames        []string
			apiServerAPIService              *apiregistrationv1.APIService
			apiServerRoleBinding             *rbacv1.RoleBinding
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
			ns, cleanupFunc = fixture.CreateNamespaceWithCleanupFunc("gitops-promoter-1-134")

			// Deploy an ArgoCD CR with only the Promoter enabled and nothing else
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Promoter: &argov1beta1api.PromoterSpec{
						Enabled: ptr.To(true),
					},
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(false),
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Enabled: ptr.To(false),
					},
					Repo: argov1beta1api.ArgoCDRepoSpec{
						Enabled: ptr.To(false),
					},
					Redis: argov1beta1api.ArgoCDRedisSpec{
						Enabled: ptr.To(false),
					},
				},
			}

			controllerClusterScopedName := fmt.Sprintf(promoterControllerResourceNameWithNamespace, ns.Name)
			apiServerClusterScopedName := fmt.Sprintf(promoterAPIServerResourceNameWithNamespace, ns.Name)

			controllerServiceAccount = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promoterControllerResourceName,
					Namespace: ns.Name,
				},
			}

			apiServerServiceAccount = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promoterAPIServerResourceName,
					Namespace: ns.Name,
				},
			}

			controllerDeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promoterControllerResourceName,
					Namespace: ns.Name,
				},
			}

			apiServerDeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promoterAPIServerResourceName,
					Namespace: ns.Name,
				},
			}

			controllerService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promoterControllerResourceName,
					Namespace: ns.Name,
				},
			}

			apiServerService = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      promoterAPIServerResourceName,
					Namespace: ns.Name,
				},
			}

			controllerConfig = &promoter.ControllerConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "promoter-controller-configuration",
					Namespace: ns.Name,
				},
			}

			controllerClusterRoleNames = []string{controllerClusterScopedName}
			controllerClusterRoleBindingNames = []string{controllerClusterScopedName}

			apiServerClusterRoleNames = []string{apiServerClusterScopedName, apiServerClusterScopedName + "-promotionstrategydetails-viewer"}
			apiServerClusterRoleBindingNames = []string{apiServerClusterScopedName, apiServerClusterScopedName + "-promotionstrategydetails-viewer", apiServerClusterScopedName + "-auth-delegator"}

			apiServerAPIService = &apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "v1alpha1.view.promoter.argoproj.io",
				},
			}
			apiServerRoleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      apiServerClusterScopedName + "-extension-auth-reader",
					Namespace: "kube-system",
				},
			}
		})

		AfterEach(func() {
			By("Cleaning up namespace")
			if cleanupFunc != nil {
				cleanupFunc()
			}
		})

		createAPIServerTLSSecrets := func(namespace *corev1.Namespace) {
			promoterFixture.CreateAPIServerTLSSecrets(promoterFixture.PromoterAPIServerTLSSecretConfig{
				PromoterNamespace:       namespace.Name,
				APIServerCertSecretName: apiServerTLSSecretName,
				APIServerServiceName:    promoterAPIServerResourceName,
				CABundleSecretName:      apiServerCABundleSecretName,
				CABundleSecretKey:       apiServerCABundleSecretKey,
			})
		}

		It("should create all controller and api server resources when enabled and delete them all when disabled", func() {
			By("create Argo CD Instance with Promoter enabled")

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			controllerResources := promoterFixture.VerifyExpectedResourcesExistParams{
				Namespace:               ns,
				Deployment:              controllerDeployment,
				Service:                 nil,
				ControllerConfiguration: controllerConfig,
				ClusterRoleNames:        controllerClusterRoleNames,
				ClusterRoleBindingNames: controllerClusterRoleBindingNames,
				RoleBinding:             nil,
				ServiceAccount:          controllerServiceAccount,
				APIService:              nil,
			}

			apiServerResources := promoterFixture.VerifyExpectedResourcesExistParams{
				Namespace:               ns,
				Deployment:              apiServerDeployment,
				Service:                 apiServerService,
				ControllerConfiguration: nil,
				ClusterRoleNames:        apiServerClusterRoleNames,
				ClusterRoleBindingNames: apiServerClusterRoleBindingNames,
				RoleBinding:             apiServerRoleBinding,
				ServiceAccount:          apiServerServiceAccount,
				APIService:              apiServerAPIService,
			}

			By("verify that Controller resources were reconciled")
			promoterFixture.VerifyExpectedResourcesExist(controllerResources)

			By("Verify that API Server resources were reconciled")
			promoterFixture.VerifyExpectedResourcesExist(apiServerResources)

			argoCDFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Promoter.Enabled = ptr.To(false)
			})

			By("Verify that Controller resources were deleted")
			promoterFixture.VerifyExpectedResourcesDontExist(controllerResources)

			By("Verify that API Server resources were deleted")
			promoterFixture.VerifyExpectedResourcesDontExist(apiServerResources)
		})

		It("API server is enabled by default and can be disabled", func() {
			By("Create Argo CD Instance with Promoter's API Server enabled")

			argoCD.Spec.Promoter.APIServer = &argov1beta1api.PromoterAPIServerSpec{}
			argoCD.Spec.Promoter.APIServer.Enabled = ptr.To(true)
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			apiServerResources := promoterFixture.VerifyExpectedResourcesExistParams{
				Namespace:               ns,
				Deployment:              apiServerDeployment,
				Service:                 apiServerService,
				ControllerConfiguration: nil,
				ClusterRoleNames:        apiServerClusterRoleNames,
				ClusterRoleBindingNames: apiServerClusterRoleBindingNames,
				RoleBinding:             apiServerRoleBinding,
				ServiceAccount:          apiServerServiceAccount,
				APIService:              apiServerAPIService,
			}

			By("Verify that API Server were created")
			promoterFixture.VerifyExpectedResourcesExist(apiServerResources)

			By("Disable API server and verify that the resources are deleted")

			argoCDFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Promoter.APIServer = &argov1beta1api.PromoterAPIServerSpec{
					Enabled: ptr.To(false),
				}
			})
			promoterFixture.VerifyExpectedResourcesDontExist(apiServerResources)

			By("Enable API server again and make sure resources are created again")

			argoCDFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Promoter.APIServer = &argov1beta1api.PromoterAPIServerSpec{
					Enabled: ptr.To(true),
				}
			})
			promoterFixture.VerifyExpectedResourcesExist(apiServerResources)
		})

		It("Webhook resources get created when enabled and are configurable", func() {
			By("Create Argo CD Instance without Promoter webhook to ensure its not present by default")

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			webhookResources := promoterFixture.VerifyExpectedResourcesExistParams{
				Service: controllerService,
			}
			promoterFixture.VerifyExpectedResourcesDontExist(webhookResources)

			By("Enable webhook and make sure that the service is created")

			argoCDFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Promoter.Webhook = &argov1beta1api.PromoterControllerWebhookSpec{
					Enabled: ptr.To(true),
				}
			})
			promoterFixture.VerifyExpectedResourcesExist(webhookResources)

			By("Can change the webhook service type in the CR and it gets reflected in the service")

			argoCDFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Promoter.Webhook = &argov1beta1api.PromoterControllerWebhookSpec{
					Enabled:     ptr.To(true),
					ServiceType: "NodePort",
				}
			})
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(controllerService), controllerService); err != nil {
					return false
				}
				return controllerService.Spec.Type == corev1.ServiceTypeNodePort
			}, "60s", "5s").Should(BeTrue())

			By("Disabling the webhook deletes its resources")

			argoCDFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Promoter.Webhook = &argov1beta1api.PromoterControllerWebhookSpec{
					Enabled: ptr.To(false),
				}
			})
			promoterFixture.VerifyExpectedResourcesDontExist(webhookResources)
		})

		It("API Server TLS settings can configured correctly", func() {
			By("Create Argo CD Instance where the Promoter's API Server is configured with correct TLS settings")

			argoCD.Spec.Promoter.APIServer = &argov1beta1api.PromoterAPIServerSpec{
				TLS: &argov1beta1api.PromoterAPIServerTLSSpec{
					CABundleSecretName: apiServerCABundleSecretName,
					CABundleSecretKey:  apiServerCABundleSecretKey,
					CertSecretName:     apiServerTLSSecretName,
				},
				Enabled: ptr.To(true),
			}

			createAPIServerTLSSecrets(ns)
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("API Service should have CA Bundle")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(apiServerAPIService), apiServerAPIService); err != nil {
					return false
				}

				caBundleSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      apiServerCABundleSecretName,
						Namespace: ns.Name,
					},
				}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(caBundleSecret), caBundleSecret); err != nil {
					return false
				}

				return reflect.DeepEqual(apiServerAPIService.Spec.CABundle, caBundleSecret.Data[apiServerCABundleSecretKey])
			}, "60s", "5s").Should(BeTrue())

			By("API Server's Deployment Volume references correct secret")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(apiServerDeployment), apiServerDeployment); err != nil {
					return false
				}

				for _, volume := range apiServerDeployment.Spec.Template.Spec.Volumes {
					if volume.Secret != nil && volume.Secret.SecretName == apiServerTLSSecretName {
						return true
					}
				}
				return false
			}, "60s", "5s").Should(BeTrue())
		})

		It("GitOps Promoter Argo CD UI Extension gets added to the ArgoCD Server Deployment", func() {
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Promoter: &argov1beta1api.PromoterSpec{
						Enabled:                  ptr.To(true),
						ArgoCDUIExtensionEnabled: true,
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verifying that the argocd-server exists")
			argoCDServer := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-server",
					Namespace: ns.Name,
				},
			}
			Eventually(argoCDServer, "60s", "5s").Should(k8sFixture.ExistByName())

			By("Verifying that argocd-server has expected extensions volume")
			Expect(argoCDServer).Should(deploymentFixture.HaveSpecTemplateSpecVolume(corev1.Volume{
				Name: "argo-cd-operator-ui-extensions",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}))

			By("verify that the init container is as expected")
			initContainer := deploymentFixture.GetTemplateSpecInitContainerByName("promoter-extension", *argoCDServer)
			Expect(initContainer).ToNot(BeNil())

			Expect(initContainer.Image).To(Equal(common.ArgoCDExtensionInstallerImage))

			Expect(initContainer.Env).To(Equal([]corev1.EnvVar{
				{Name: "EXTENSION_URL", Value: common.GitopsPromoterExtensionURL},
			}))

			By("verify that argocd-server gets the extension volume")
			container := deploymentFixture.GetTemplateSpecContainerByName("argocd-server", *argoCDServer)
			Expect(container).ToNot(BeNil())

			expectedVolumeMount := corev1.VolumeMount{
				Name:      "argo-cd-operator-ui-extensions",
				MountPath: "/tmp/extensions/",
			}

			match := false
			for _, volumeMount := range container.VolumeMounts {
				if reflect.DeepEqual(volumeMount, expectedVolumeMount) {
					match = true
				}
			}
			Expect(match).To(BeTrue())

			By("verify that disabling the extension cleans up its settings")
			argoCDFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.Promoter = &argov1beta1api.PromoterSpec{
					Enabled:                  ptr.To(true),
					ArgoCDUIExtensionEnabled: false,
				}
			})
			Eventually(argoCDServer, "60s", "5s").Should(k8sFixture.ExistByName())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(argoCDServer), argoCDServer); err != nil {
					GinkgoWriter.Println(err)
					return false
				}
				return len(argoCDServer.Spec.Template.Spec.InitContainers) == 0
			}).Should(BeTrue())

			match = false
			for _, volume := range argoCDServer.Spec.Template.Spec.Volumes {
				if volume.Name == "argo-cd-operator-ui-extensions" {
					match = true
				}
			}
			Expect(match).To(BeFalse())

			container = deploymentFixture.GetTemplateSpecContainerByName("argocd-server", *argoCDServer)
			Expect(container).ToNot(BeNil())

			match = false
			for _, volumeMount := range container.VolumeMounts {
				if volumeMount.Name == "argo-cd-operator-ui-extensions" {
					match = true
				}
			}
			Expect(match).To(BeFalse())
		})
	})
})
