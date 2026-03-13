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

package sequential

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocdagent"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	agentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/agent"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	const (
		argoCDName                    = "example"
		argoCDAgentPrincipalName      = "example-agent-principal" // argoCDName + "-agent-principal"
		principalMetricsServiceFmt    = "%s-agent-principal-metrics"
		principalRedisProxyServiceFmt = "%s-agent-principal-redisproxy"
		principalHealthzServiceFmt    = "%s-agent-principal-healthz"
	)

	Context("1-051_validate_argocd_agent_principal", func() {

		var (
			k8sClient                client.Client
			ctx                      context.Context
			argoCD                   *argov1beta1api.ArgoCD
			ns                       *corev1.Namespace
			cleanupFunc              func()
			serviceAccount           *corev1.ServiceAccount
			role                     *rbacv1.Role
			roleBinding              *rbacv1.RoleBinding
			clusterRole              *rbacv1.ClusterRole
			clusterRoleBinding       *rbacv1.ClusterRoleBinding
			serviceNames             []string
			deploymentNames          []string
			principalDeployment      *appsv1.Deployment
			expectedEnvVariables     map[string]string
			secretNames              agentFixture.AgentSecretNames
			principalRoute           *routev1.Route
			principalNetworkPolicy   *networkingv1.NetworkPolicy
			resourceProxyServiceName string
			principalResources       agentFixture.PrincipalResources
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
			ns, cleanupFunc = fixture.CreateNamespaceWithCleanupFunc("argocd-agent-principal-1-051")

			// Define ArgoCD CR with principal enabled
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(false),
					},
					ArgoCDAgent: &argov1beta1api.ArgoCDAgentSpec{
						Principal: &argov1beta1api.PrincipalSpec{
							Enabled:  ptr.To(true),
							Auth:     "mtls:CN=([^,]+)",
							LogLevel: "info",
							Namespace: &argov1beta1api.PrincipalNamespaceSpec{
								AllowedNamespaces: []string{
									"*",
								},
							},
							TLS: &argov1beta1api.PrincipalTLSSpec{
								InsecureGenerate: ptr.To(true),
							},
							JWT: &argov1beta1api.PrincipalJWTSpec{
								InsecureGenerate: ptr.To(true),
							},
							Server: &argov1beta1api.PrincipalServerSpec{
								KeepAliveMinInterval: "30s",
							},
						},
					},
					SourceNamespaces: []string{
						"agent-managed",
						"agent-autonomous",
					},
				},
			}

			// Define required resources for principal pod
			serviceAccount = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentPrincipalName,
					Namespace: ns.Name,
				},
			}

			role = &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentPrincipalName,
					Namespace: ns.Name,
				},
			}

			roleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentPrincipalName,
					Namespace: ns.Name,
				},
			}

			clusterRole = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-agent-principal", argoCDName, ns.Name),
				},
			}

			clusterRoleBinding = &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-agent-principal", argoCDName, ns.Name),
				},
			}

			// List required secrets for principal pod
			secretNames = agentFixture.AgentSecretNames{
				JWTSecretName:                  agentJWTSecretName,
				PrincipalTLSSecretName:         agentPrincipalTLSSecretName,
				RootCASecretName:               agentRootCASecretName,
				ResourceProxyTLSSecretName:     agentResourceProxyTLSSecretName,
				RedisInitialPasswordSecretName: "example-redis-initial-password",
			}

			resourceProxyServiceName = fmt.Sprintf("%s-agent-principal-resource-proxy", argoCDName)
			serviceNames = []string{
				argoCDAgentPrincipalName,
				fmt.Sprintf(principalMetricsServiceFmt, argoCDName),
				fmt.Sprintf("%s-redis", argoCDName),
				fmt.Sprintf("%s-repo-server", argoCDName),
				fmt.Sprintf("%s-server", argoCDName),
				fmt.Sprintf(principalRedisProxyServiceFmt, argoCDName),
				resourceProxyServiceName,
				fmt.Sprintf(principalHealthzServiceFmt, argoCDName),
			}
			deploymentNames = []string{fmt.Sprintf("%s-redis", argoCDName), fmt.Sprintf("%s-repo-server", argoCDName), fmt.Sprintf("%s-server", argoCDName)}

			principalDeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentPrincipalName,
					Namespace: ns.Name,
				},
			}

			principalRoute = &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-agent-principal", argoCDName),
					Namespace: ns.Name,
				},
			}

			principalNetworkPolicy = &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-agent-principal-network-policy", argoCDName),
					Namespace: ns.Name,
				},
			}

			// List environment variables with expected values for the principal deployment
			expectedEnvVariables = map[string]string{
				argocdagent.EnvArgoCDPrincipalLogLevel:                  "info",
				argocdagent.EnvArgoCDPrincipalNamespace:                 ns.Name,
				argocdagent.EnvArgoCDPrincipalAllowedNamespaces:         "*",
				argocdagent.EnvArgoCDPrincipalNamespaceCreateEnable:     "false",
				argocdagent.EnvArgoCDPrincipalNamespaceCreatePattern:    "",
				argocdagent.EnvArgoCDPrincipalNamespaceCreateLabels:     "",
				argocdagent.EnvArgoCDPrincipalTLSServerAllowGenerate:    "true",
				argocdagent.EnvArgoCDPrincipalJWTAllowGenerate:          "true",
				argocdagent.EnvArgoCDPrincipalAuth:                      "mtls:CN=([^,]+)",
				argocdagent.EnvArgoCDPrincipalEnableResourceProxy:       "true",
				argocdagent.EnvArgoCDPrincipalKeepAliveMinInterval:      "30s",
				argocdagent.EnvArgoCDPrincipalRedisServerAddress:        fmt.Sprintf("%s-%s:%d", argoCDName, "redis", common.ArgoCDDefaultRedisPort),
				argocdagent.EnvArgoCDPrincipalRedisCompressionType:      "gzip",
				argocdagent.EnvArgoCDPrincipalLogFormat:                 "text",
				argocdagent.EnvArgoCDPrincipalEnableWebSocket:           "false",
				argocdagent.EnvArgoCDPrincipalTLSSecretName:             agentPrincipalTLSSecretName,
				argocdagent.EnvArgoCDPrincipalTLSServerRootCASecretName: agentRootCASecretName,
				argocdagent.EnvArgoCDPrincipalResourceProxySecretName:   agentResourceProxyTLSSecretName,
				argocdagent.EnvArgoCDPrincipalResourceProxyCaSecretName: agentRootCASecretName,
				argocdagent.EnvArgoCDPrincipalJwtSecretName:             agentJWTSecretName,
			}

			principalResources = agentFixture.PrincipalResources{
				PrincipalNamespaceName:   ns.Name,
				ArgoCDAgentPrincipalName: argoCDAgentPrincipalName,
				ArgoCDName:               argoCDName,
				ServiceAccount:           serviceAccount,
				Role:                     role,
				RoleBinding:              roleBinding,
				ClusterRole:              clusterRole,
				ClusterRoleBinding:       clusterRoleBinding,
				PrincipalDeployment:      principalDeployment,
				PrincipalRoute:           principalRoute,
				PrincipalNetworkPolicy:   principalNetworkPolicy,
				ServicesToDelete: []string{
					argoCDAgentPrincipalName,
					fmt.Sprintf(principalMetricsServiceFmt, argoCDName),
					fmt.Sprintf(principalRedisProxyServiceFmt, argoCDName),
					resourceProxyServiceName,
					fmt.Sprintf(principalHealthzServiceFmt, argoCDName),
				},
			}
		})

		AfterEach(func() {
			By("Cleanup cluster-scoped resources")
			_ = k8sClient.Delete(ctx, clusterRole)
			_ = k8sClient.Delete(ctx, clusterRoleBinding)

			By("Cleanup namespace")
			if cleanupFunc != nil {
				cleanupFunc()
			}
		})

		createRequiredSecrets := func(namespace *corev1.Namespace, additionalPrincipalSANs ...string) {
			agentFixture.CreateRequiredSecrets(agentFixture.PrincipalSecretsConfig{
				PrincipalNamespaceName:     namespace.Name,
				PrincipalServiceName:       argoCDAgentPrincipalName,
				ResourceProxyServiceName:   resourceProxyServiceName,
				JWTSecretName:              secretNames.JWTSecretName,
				PrincipalTLSSecretName:     secretNames.PrincipalTLSSecretName,
				RootCASecretName:           secretNames.RootCASecretName,
				ResourceProxyTLSSecretName: secretNames.ResourceProxyTLSSecretName,
				AdditionalPrincipalSANs:    additionalPrincipalSANs,
			})
		}

		verifyExpectedResourcesExist := func(namespace *corev1.Namespace, expectRoute ...bool) {
			var expectRoutePtr *bool
			if len(expectRoute) > 0 {
				expectRoutePtr = ptr.To(expectRoute[0])
			}

			agentFixture.VerifyExpectedResourcesExist(agentFixture.VerifyExpectedResourcesExistParams{
				Namespace:                namespace,
				ArgoCDAgentPrincipalName: argoCDAgentPrincipalName,
				ArgoCDName:               argoCDName,
				ServiceAccount:           serviceAccount,
				Role:                     role,
				RoleBinding:              roleBinding,
				ClusterRole:              clusterRole,
				ClusterRoleBinding:       clusterRoleBinding,
				PrincipalDeployment:      principalDeployment,
				PrincipalRoute:           principalRoute,
				SecretNames:              secretNames,
				ServiceNames:             serviceNames,
				PrincipalNetworkPolicy:   principalNetworkPolicy,
				DeploymentNames:          deploymentNames,
				ExpectRoute:              expectRoutePtr,
			})
		}

		verifyResourcesDeleted := func() {
			agentFixture.VerifyResourcesDeleted(principalResources)
		}

		It("should create argocd agent principal resources, but pod should fail to start as image does not exist", func() {
			// Change log level to trace and custom image name
			argoCD.Spec.ArgoCDAgent.Principal.LogLevel = "trace"
			argoCD.Spec.ArgoCDAgent.Principal.Image = "quay.io/user/argocd-agent:v1"

			By("Create ArgoCD instance")

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns)

			By("Verify principal has the custom image we specified in ArgoCD CR")

			container := deploymentFixture.GetTemplateSpecContainerByName(argoCDAgentPrincipalName, *principalDeployment)
			Expect(container).ToNot(BeNil())
			Expect(container.Image).To(Equal("quay.io/user/argocd-agent:v1"))

			By("Verify environment variables are set correctly")

			// update expected value in default environment variables according to ArgoCD CR in the test
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalLogLevel] = "trace"

			for key, value := range expectedEnvVariables {
				Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: key, Value: value}), "Environment variable %s should be set to %s", key, value)
			}

			By("Disable principal")

			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: argoCDName, Namespace: ns.Name}, argoCD)).To(Succeed())

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Principal.Enabled = ptr.To(false)
			})

			By("Verify principal resources are deleted")

			verifyResourcesDeleted()
		})

		It("should create argocd agent principal resources, and pod should start successfully with default image", func() {

			// Add a custom environment variable to the principal server
			argoCD.Spec.ArgoCDAgent.Principal.Env = []corev1.EnvVar{{Name: "TEST_ENV", Value: "test_value"}}

			By("Create ArgoCD instance")

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns)

			By("Verify principal uses the default agent image")

			container := deploymentFixture.GetTemplateSpecContainerByName(argoCDAgentPrincipalName, *principalDeployment)
			Expect(container).ToNot(BeNil())
			Expect(container.Image).To(Equal(common.ArgoCDAgentPrincipalDefaultImageName))

			By("Create required secrets and certificates for principal pod to start properly")

			createRequiredSecrets(ns)

			By("Verify principal pod starts successfully by checking logs")

			agentFixture.VerifyLogs(argoCDAgentPrincipalName, ns.Name, []string{
				"Starting metrics server",
				"Redis proxy started",
				"Application informer synced and ready",
				"AppProject informer synced and ready",
				"Resource proxy started",
				"Namespace informer synced and ready",
				"Starting healthz server",
			})

			By("verify that deployment is in Ready state")

			Eventually(principalDeployment, "120s", "5s").Should(deploymentFixture.HaveReadyReplicas(1), "Principal deployment should become ready")

			By("Verify environment variables are set correctly")

			for key, value := range expectedEnvVariables {
				Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: key, Value: value}), "Environment variable %s should be set to %s", key, value)
			}

			Expect(container.Env).To(ContainElement(And(
				HaveField("Name", argocdagent.EnvRedisPassword),
				HaveField("ValueFrom.SecretKeyRef", Not(BeNil())),
			)), "REDIS_PASSWORD should be set with valueFrom.secretKeyRef")

			By("Disable principal")

			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: argoCDName, Namespace: ns.Name}, argoCD)).To(Succeed())

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Principal.Enabled = nil
			})

			By("Verify principal resources are deleted")

			verifyResourcesDeleted()
		})

		It("Should reflect configuration changes from ArgoCD CR to the principal deployment", func() {

			By("Create ArgoCD instance")

			argoCD.Spec.ArgoCDAgent.Principal.Image = common.ArgoCDAgentPrincipalDefaultImageName
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns)

			By("Verify principal has the custom image we specified in ArgoCD CR")

			container := deploymentFixture.GetTemplateSpecContainerByName(argoCDAgentPrincipalName, *principalDeployment)
			Expect(container).ToNot(BeNil())
			Expect(container.Image).To(Equal(common.ArgoCDAgentPrincipalDefaultImageName))

			By("Verify environment variables are set correctly")

			// update expected value in default environment variables according to ArgoCD CR in the test
			for key, value := range expectedEnvVariables {
				Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: key, Value: value}), "Environment variable %s should be set to %s", key, value)
			}

			By("Update ArgoCD CR with new configuration")

			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: argoCDName, Namespace: ns.Name}, argoCD)).To(Succeed())

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {

				ac.Spec.ArgoCDAgent.Principal.LogLevel = "trace"
				ac.Spec.ArgoCDAgent.Principal.LogFormat = "json"
				ac.Spec.ArgoCDAgent.Principal.Server.KeepAliveMinInterval = "60s"
				ac.Spec.ArgoCDAgent.Principal.Server.EnableWebSocket = ptr.To(true)
				ac.Spec.ArgoCDAgent.Principal.Image = "quay.io/argoprojlabs/argocd-agent:v0.5.1"

				ac.Spec.ArgoCDAgent.Principal.Namespace.AllowedNamespaces = []string{"agent-managed", "agent-autonomous"}
				ac.Spec.ArgoCDAgent.Principal.Namespace.EnableNamespaceCreate = ptr.To(true)
				ac.Spec.ArgoCDAgent.Principal.Namespace.NamespaceCreatePattern = "agent-.*"
				ac.Spec.ArgoCDAgent.Principal.Namespace.NamespaceCreateLabels = []string{"environment=agent"}

				ac.Spec.ArgoCDAgent.Principal.TLS.InsecureGenerate = ptr.To(false)
				ac.Spec.ArgoCDAgent.Principal.TLS.SecretName = "argocd-agent-principal-tls-v2"
				ac.Spec.ArgoCDAgent.Principal.TLS.RootCASecretName = "argocd-agent-ca-v2"

				ac.Spec.ArgoCDAgent.Principal.JWT.InsecureGenerate = ptr.To(false)
				ac.Spec.ArgoCDAgent.Principal.JWT.SecretName = "argocd-agent-jwt-v2"

				ac.Spec.ArgoCDAgent.Principal.ResourceProxy = &argov1beta1api.PrincipalResourceProxySpec{
					SecretName:   "argocd-agent-resource-proxy-tls-v2",
					CASecretName: "argocd-agent-ca-v2",
				}

			})

			By("Create required secrets and certificates for principal pod to start properly")

			// Update secret names according to ArgoCD CR
			secretNames = agentFixture.AgentSecretNames{
				JWTSecretName:              "argocd-agent-jwt-v2",
				PrincipalTLSSecretName:     "argocd-agent-principal-tls-v2",
				RootCASecretName:           "argocd-agent-ca-v2",
				ResourceProxyTLSSecretName: "argocd-agent-resource-proxy-tls-v2",
			}
			createRequiredSecrets(ns)

			By("Verify principal has the updated image we specified in ArgoCD CR")

			Eventually(principalDeployment).Should(k8sFixture.ExistByName())
			Eventually(
				func() bool {
					// Fetch the latest deployment from the cluster
					err := k8sClient.Get(ctx, client.ObjectKey{Name: argoCDAgentPrincipalName, Namespace: ns.Name}, principalDeployment)
					if err != nil {
						GinkgoWriter.Println("Error getting deployment for image check: ", err)
						return false
					}
					container = deploymentFixture.GetTemplateSpecContainerByName(argoCDAgentPrincipalName, *principalDeployment)
					if container == nil {
						return false
					}
					return container.Image == "quay.io/argoprojlabs/argocd-agent:v0.5.1"
				}, "120s", "5s").Should(BeTrue(), "Principal deployment should have the updated image")

			By("verify that deployment is in Ready state")

			Eventually(principalDeployment, "120s", "5s").Should(deploymentFixture.HaveReadyReplicas(1), "Principal deployment should become ready")

			By("Verify environment variables are updated correctly")

			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalLogLevel] = "trace"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalLogFormat] = "json"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalKeepAliveMinInterval] = "60s"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalEnableWebSocket] = "true"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalAllowedNamespaces] = "agent-managed,agent-autonomous"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalNamespaceCreateEnable] = "true"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalNamespaceCreatePattern] = "agent-.*"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalNamespaceCreateLabels] = "environment=agent"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalTLSServerAllowGenerate] = "false"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalJWTAllowGenerate] = "false"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalResourceProxySecretName] = "argocd-agent-resource-proxy-tls-v2"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalResourceProxyCaSecretName] = "argocd-agent-ca-v2"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalTLSSecretName] = "argocd-agent-principal-tls-v2"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalTLSServerRootCASecretName] = "argocd-agent-ca-v2"
			expectedEnvVariables[argocdagent.EnvArgoCDPrincipalJwtSecretName] = "argocd-agent-jwt-v2"

			for key, value := range expectedEnvVariables {
				Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: key, Value: value}), "Environment variable %s should be set to %s", key, value)
			}
		})

		It("should handle route disabled configuration correctly", func() {

			By("Create ArgoCD instance with route disabled")

			argoCD.Spec.ArgoCDAgent.Principal.Server.Route = argov1beta1api.ArgoCDAgentPrincipalRouteSpec{
				Enabled: ptr.To(false),
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns, false)

			By("Verify Route for principal does not exist")

			if fixture.RunningOnOpenShift() {
				Consistently(principalRoute, "10s", "1s").Should(k8sFixture.NotExistByName())
			}
		})

		It("should handle route enabled configuration correctly", func() {

			By("Create ArgoCD instance with route enabled")

			argoCD.Spec.ArgoCDAgent.Principal.Server.Route = argov1beta1api.ArgoCDAgentPrincipalRouteSpec{
				Enabled: ptr.To(true),
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns)

			By("Verify Route for principal exists")

			if fixture.RunningOnOpenShift() {
				Eventually(principalRoute).Should(k8sFixture.ExistByName())
			}
		})

		It("should handle route toggle from enabled to disabled correctly", func() {

			By("Create ArgoCD instance with route enabled")

			argoCD.Spec.ArgoCDAgent.Principal.Server.Route = argov1beta1api.ArgoCDAgentPrincipalRouteSpec{
				Enabled: ptr.To(true),
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns)

			By("Verify Route for principal exists")

			if fixture.RunningOnOpenShift() {
				Eventually(principalRoute).Should(k8sFixture.ExistByName())
			}

			By("Disable route while keeping principal enabled")

			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: argoCDName, Namespace: ns.Name}, argoCD)).To(Succeed())
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Principal.Server.Route.Enabled = ptr.To(false)
			})

			By("Verify Route for principal is deleted")

			if fixture.RunningOnOpenShift() {
				Eventually(principalRoute).Should(k8sFixture.NotExistByName())
			}

			By("Verify other principal resources still exist")

			Eventually(principalDeployment).Should(k8sFixture.ExistByName())

			for _, serviceName := range []string{
				fmt.Sprintf("%s-agent-principal", argoCDName),
				fmt.Sprintf(principalMetricsServiceFmt, argoCDName),
				fmt.Sprintf(principalRedisProxyServiceFmt, argoCDName),
				fmt.Sprintf("%s-agent-principal-resource-proxy", argoCDName),
				fmt.Sprintf(principalHealthzServiceFmt, argoCDName),
			} {
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: ns.Name,
					},
				}
				Eventually(service, "30s", "2s").Should(k8sFixture.ExistByName(), "Service '%s' should exist in namespace '%s'", serviceName, ns.Name)
			}

			By("Re-enable route")

			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: argoCDName, Namespace: ns.Name}, argoCD)).To(Succeed())
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Principal.Server.Route.Enabled = ptr.To(true)
			})

			By("Verify Route for principal is recreated")

			if fixture.RunningOnOpenShift() {
				Eventually(principalRoute).Should(k8sFixture.ExistByName())
			}
		})

		It("should handle service type ClusterIP configuration correctly", func() {

			By("Create ArgoCD instance with service type ClusterIP")

			argoCD.Spec.ArgoCDAgent.Principal.Server.Service = argov1beta1api.ArgoCDAgentPrincipalServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns)

			By("Verify principal service has ClusterIP type")

			principalService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentPrincipalName,
					Namespace: ns.Name,
				},
			}
			Eventually(principalService).Should(k8sFixture.ExistByName())
			Expect(principalService.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
		})

		It("should handle service type LoadBalancer configuration correctly", func() {

			By("Create ArgoCD instance with service type LoadBalancer")

			argoCD.Spec.ArgoCDAgent.Principal.Server.Service = argov1beta1api.ArgoCDAgentPrincipalServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns)

			By("Verify principal service has LoadBalancer type")

			principalService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentPrincipalName,
					Namespace: ns.Name,
				},
			}
			Eventually(principalService).Should(k8sFixture.ExistByName())
			Expect(principalService.Spec.Type).To(Equal(corev1.ServiceTypeLoadBalancer))
		})

		It("should handle service type updates correctly", func() {

			By("Create ArgoCD instance with service type ClusterIP")

			argoCD.Spec.ArgoCDAgent.Principal.Server.Service = argov1beta1api.ArgoCDAgentPrincipalServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns)

			By("Verify principal service has ClusterIP type initially")

			principalService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentPrincipalName,
					Namespace: ns.Name,
				},
			}
			Eventually(principalService).Should(k8sFixture.ExistByName())
			Expect(principalService.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))

			By("Update service type to LoadBalancer")

			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: argoCDName, Namespace: ns.Name}, argoCD)).To(Succeed())
			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Principal.Server.Service.Type = corev1.ServiceTypeLoadBalancer
			})

			By("Verify principal service type is updated to LoadBalancer")

			Eventually(func() corev1.ServiceType {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: argoCDAgentPrincipalName, Namespace: ns.Name}, principalService)
				if err != nil {
					return ""
				}
				return principalService.Spec.Type
			}, "30s", "2s").Should(Equal(corev1.ServiceTypeLoadBalancer))
		})

		It("should deploy principal via namespace-scoped ArgoCD instance and verify cluster role and cluster role binding are not created", func() {

			By("Create namespace-scoped ArgoCD instance")

			// Create namespace for hosting namespace-scoped ArgoCD instance with principal
			nsScoped, cleanupFuncScoped := fixture.CreateNamespaceWithCleanupFunc("argocd-agent-principal-ns-scoped-1-051")
			defer cleanupFuncScoped()

			// Update namespace in ArgoCD CR
			argoCD.Namespace = nsScoped.Name

			// Update namespace in resource references
			serviceAccount.Namespace = nsScoped.Name
			role.Namespace = nsScoped.Name
			roleBinding.Namespace = nsScoped.Name
			principalDeployment.Namespace = nsScoped.Name
			principalRoute.Namespace = nsScoped.Name
			clusterRole.Name = fmt.Sprintf("%s-%s-agent-principal", argoCDName, nsScoped.Name)
			clusterRoleBinding.Name = fmt.Sprintf("%s-%s-agent-principal", argoCDName, nsScoped.Name)

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify namespace-scoped resources are created for principal")

			Eventually(serviceAccount, "30s", "2s").Should(k8sFixture.ExistByName())
			Eventually(role, "30s", "2s").Should(k8sFixture.ExistByName())
			Eventually(roleBinding, "30s", "2s").Should(k8sFixture.ExistByName())
			Eventually(principalDeployment, "30s", "2s").Should(k8sFixture.ExistByName())
			for _, serviceName := range serviceNames {
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: nsScoped.Name,
					},
				}
				Eventually(service, "30s", "2s").Should(k8sFixture.ExistByName(),
					"Service '%s' should exist in namespace '%s'", serviceName, nsScoped.Name)
			}

			By("Verify ClusterRole and ClusterRoleBinding are not created")
			Consistently(clusterRole, "10s", "1s").Should(k8sFixture.NotExistByName(),
				"ClusterRole '%s' should not exist for namespace-scoped ArgoCD instance", clusterRole.Name)

			Consistently(clusterRoleBinding, "10s", "1s").Should(k8sFixture.NotExistByName(),
				"ClusterRoleBinding '%s' should not exist for namespace-scoped ArgoCD instance", clusterRoleBinding.Name)

			By("Delete ArgoCD instance")
			Expect(k8sClient.Delete(ctx, argoCD)).To(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: argoCD.Name, Namespace: argoCD.Namespace}, argoCD)
				return err != nil
			}, "60s", "2s").Should(BeTrue(), "ArgoCD should be deleted")
		})

		It("should delete existing cluster role and cluster role binding if ArgoCD instance is namespace-scoped", func() {

			By("Create namespace-scoped ArgoCD instance namespace")

			// Create namespace for hosting namespace-scoped ArgoCD instance with principal
			nsScoped, cleanupFuncScoped := fixture.CreateNamespaceWithCleanupFunc("argocd-agent-principal-ns-scoped-1-051")
			defer cleanupFuncScoped()

			// Update namespace in ArgoCD CR
			argoCD.Namespace = nsScoped.Name

			// Update namespace in resource references
			serviceAccount.Namespace = nsScoped.Name
			role.Namespace = nsScoped.Name
			roleBinding.Namespace = nsScoped.Name
			principalDeployment.Namespace = nsScoped.Name
			principalRoute.Namespace = nsScoped.Name
			clusterRole.Name = fmt.Sprintf("%s-%s-agent-principal", argoCDName, nsScoped.Name)
			clusterRoleBinding.Name = fmt.Sprintf("%s-%s-agent-principal", argoCDName, nsScoped.Name)

			By("Pre-create ClusterRole and ClusterRoleBinding before ArgoCD instance")

			preExistingClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRole.Name,
					Labels: map[string]string{
						"app.kubernetes.io/name": "test",
					},
				},
			}
			Expect(k8sClient.Create(ctx, preExistingClusterRole)).To(Succeed())

			preExistingClusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterRoleBinding.Name,
					Labels: map[string]string{
						"app.kubernetes.io/name": "test",
					},
				},
				Subjects: []rbacv1.Subject{
					{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      "default",
						Namespace: nsScoped.Name,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     clusterRole.Name,
				},
			}
			Expect(k8sClient.Create(ctx, preExistingClusterRoleBinding)).To(Succeed())

			By("Verify pre-existing ClusterRole and ClusterRoleBinding exist")

			Eventually(clusterRole, "30s", "1s").Should(k8sFixture.ExistByName(),
				"Pre-existing ClusterRole '%s' should exist before ArgoCD instance creation", clusterRole.Name)
			Eventually(clusterRoleBinding, "30s", "1s").Should(k8sFixture.ExistByName(),
				"Pre-existing ClusterRoleBinding '%s' should exist before ArgoCD instance creation", clusterRoleBinding.Name)

			By("Create namespace-scoped ArgoCD instance with principal")

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify pre-existing ClusterRole and ClusterRoleBinding are deleted")

			Eventually(clusterRole, "60s", "2s").Should(k8sFixture.NotExistByName(),
				"ClusterRole '%s' should be deleted for namespace-scoped ArgoCD instance", clusterRole.Name)

			Eventually(clusterRoleBinding, "60s", "2s").Should(k8sFixture.NotExistByName(),
				"ClusterRoleBinding '%s' should be deleted for namespace-scoped ArgoCD instance", clusterRoleBinding.Name)

			By("Delete ArgoCD instance")
			Expect(k8sClient.Delete(ctx, argoCD)).To(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: argoCD.Name, Namespace: argoCD.Namespace}, argoCD)
				return err != nil
			}, "60s", "2s").Should(BeTrue(), "ArgoCD should be deleted")
		})

		It("should create principal NetworkPolicy if principal is enabled", func() {
			By("Create ArgoCD instance with principal enabled")

			argoCD.Spec.ArgoCDAgent.Principal.Enabled = ptr.To(true)
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			verifyExpectedResourcesExist(ns)
			By("Verify principal NetworkPolicy exists and has expected policy addresses and ports")

			Expect(principalNetworkPolicy.Spec.PodSelector.MatchLabels[common.ArgoCDKeyName]).To(Equal(argoCDAgentPrincipalName))
			Expect(principalNetworkPolicy.Spec.PolicyTypes).To(ContainElement(networkingv1.PolicyTypeIngress))
			Expect(principalNetworkPolicy.Spec.Ingress).To(HaveLen(2))

			ing := principalNetworkPolicy.Spec.Ingress[0]
			Expect(ing.From).To(HaveLen(1))
			Expect(ing.From[0].NamespaceSelector).ToNot(BeNil())
			Expect(*ing.From[0].NamespaceSelector).To(Equal(metav1.LabelSelector{}))

			// Ports the principal exposes (see argocdagent deployment/service code)
			expectedPorts := map[int32]bool{
				8443: true, // principal HTTPS target port
				8000: true, // metrics
				6379: true, // redis proxy
				9090: true, // resource proxy
				8003: true, // healthz
			}
			Expect(ing.Ports).To(HaveLen(len(expectedPorts)))
			for _, p := range ing.Ports {
				Expect(p.Port).ToNot(BeNil())
				Expect(expectedPorts[p.Port.IntVal]).To(BeTrue(), "unexpected ingress port %d", p.Port.IntVal)
			}
			Expect(principalNetworkPolicy.Spec.Ingress[1].From).To(HaveLen(1))

			Expect(*principalNetworkPolicy.Spec.Ingress[1].From[0].IPBlock).To(Equal(networkingv1.IPBlock{CIDR: "0.0.0.0/0"}))
			Expect(principalNetworkPolicy.Spec.Ingress[1].Ports).To(HaveLen(2))
			Expect(principalNetworkPolicy.Spec.Ingress[1].Ports[0].Port.IntVal).To(Equal(int32(8443)))
			Expect(principalNetworkPolicy.Spec.Ingress[1].Ports[1].Port.IntVal).To(Equal(int32(443)))

			By("Verify principal NetworkPolicy is deleted when principal instance is disabled")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Principal.Enabled = nil
			})

			Eventually(principalNetworkPolicy).Should(k8sFixture.NotExistByName())

			By("Verify principal NetworkPolicy is created when principal instance is enabled and network policy is enabled")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Principal.Enabled = ptr.To(true)
				ac.Spec.NetworkPolicy.Enabled = ptr.To(true)
			})
			Eventually(principalNetworkPolicy).Should(k8sFixture.ExistByName())

			By("Verify principal NetworkPolicy is not created when network policy is disabled")

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.NetworkPolicy.Enabled = ptr.To(false)
			})

			Eventually(principalNetworkPolicy).Should(k8sFixture.NotExistByName())
		})
	})
})
