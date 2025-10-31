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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocdagent/agent"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	const (
		argoCDName           = "example"
		argoCDAgentAgentName = "example-agent-agent" // argoCDName + "-agent-agent"
	)

	Context("1-052_validate_argocd_agent_agent", func() {

		var (
			k8sClient            client.Client
			ctx                  context.Context
			argoCD               *argov1beta1api.ArgoCD
			ns                   *corev1.Namespace
			cleanupFunc          func()
			serviceAccount       *corev1.ServiceAccount
			role                 *rbacv1.Role
			roleBinding          *rbacv1.RoleBinding
			clusterRole          *rbacv1.ClusterRole
			clusterRoleBinding   *rbacv1.ClusterRoleBinding
			serviceNames         []string
			deploymentNames      []string
			agentDeployment      *appsv1.Deployment
			expectedEnvVariables map[string]string
			secretNames          []string
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
			ns, cleanupFunc = fixture.CreateNamespaceWithCleanupFunc("argocd-agent-agent-1-052")

			// Define ArgoCD CR with agent enabled
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDName,
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(false),
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Enabled: ptr.To(false),
					},
					ArgoCDAgent: &argov1beta1api.ArgoCDAgentSpec{
						Agent: &argov1beta1api.AgentSpec{
							Enabled: ptr.To(true),
							Client: &argov1beta1api.AgentClientSpec{
								PrincipalServerAddress: "argocd-agent-principal.example.com",
								PrincipalServerPort:    "443",
								LogLevel:               "info",
								LogFormat:              "text",
								Mode:                   "managed",
								Creds:                  "mtls:any",
								EnableWebSocket:        ptr.To(false),
								EnableCompression:      ptr.To(false),
								KeepAliveInterval:      "30s",
							},
							TLS: &argov1beta1api.AgentTLSSpec{
								SecretName:       "argocd-agent-client-tls",
								RootCASecretName: "argocd-agent-ca",
								Insecure:         ptr.To(false),
							},
							Redis: &argov1beta1api.AgentRedisSpec{
								ServerAddress: fmt.Sprintf("%s-%s:%d", argoCDName, "redis", common.ArgoCDDefaultRedisPort),
							},
						},
					},
				},
			}

			// Define required resources for agent pod
			serviceAccount = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentAgentName,
					Namespace: ns.Name,
				},
			}

			role = &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentAgentName,
					Namespace: ns.Name,
				},
			}

			roleBinding = &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentAgentName,
					Namespace: ns.Name,
				},
			}

			clusterRole = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-agent-agent", argoCDName, ns.Name),
				},
			}

			clusterRoleBinding = &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-agent-agent", argoCDName, ns.Name),
				},
			}

			// List required secrets for agent pod
			secretNames = []string{
				"argocd-agent-client-tls",
				"argocd-agent-ca",
				"example-redis-initial-password",
			}

			serviceNames = []string{
				fmt.Sprintf("%s-agent-agent-metrics", argoCDName),
				fmt.Sprintf("%s-agent-agent-healthz", argoCDName),
				fmt.Sprintf("%s-redis", argoCDName),
			}
			deploymentNames = []string{fmt.Sprintf("%s-redis", argoCDName)}

			agentDeployment = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      argoCDAgentAgentName,
					Namespace: ns.Name,
				},
			}

			// List environment variables with expected values for the agent deployment
			expectedEnvVariables = map[string]string{
				agent.EnvArgoCDAgentLogLevel:            "info",
				agent.EnvArgoCDAgentNamespace:           ns.Name,
				agent.EnvArgoCDAgentServerAddress:       "argocd-agent-principal.example.com",
				agent.EnvArgoCDAgentServerPort:          "443",
				agent.EnvArgoCDAgentLogFormat:           "text",
				agent.EnvArgoCDAgentTLSSecretName:       "argocd-agent-client-tls",
				agent.EnvArgoCDAgentTLSInsecure:         "false",
				agent.EnvArgoCDAgentTLSRootCASecretName: "argocd-agent-ca",
				agent.EnvArgoCDAgentMode:                "managed",
				agent.EnvArgoCDAgentCreds:               "mtls:any",
				agent.EnvArgoCDAgentEnableWebSocket:     "false",
				agent.EnvArgoCDAgentEnableCompression:   "false",
				agent.EnvArgoCDAgentKeepAliveInterval:   "30s",
				agent.EnvArgoCDAgentRedisAddress:        fmt.Sprintf("%s-%s:%d", argoCDName, "redis", common.ArgoCDDefaultRedisPort),
				agent.EnvArgoCDAgentEnableResourceProxy: "true",
			}
		})

		AfterEach(func() {
			By("Cleanup namespace")
			if cleanupFunc != nil {
				cleanupFunc()
			}
		})

		// verifyExpectedResourcesExist will verify that the resources that are created for agent and ArgoCD are created.
		verifyExpectedResourcesExist := func(ns *corev1.Namespace) {

			By("verifying expected resources exist")
			Eventually(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretNames[2], Namespace: ns.Name,
				}}, "60s", "2s").Should(k8sFixture.ExistByName())
			Eventually(serviceAccount).Should(k8sFixture.ExistByName())
			Eventually(role).Should(k8sFixture.ExistByName())
			Eventually(roleBinding).Should(k8sFixture.ExistByName())
			Eventually(clusterRole).Should(k8sFixture.ExistByName())
			defer func() {
				_ = k8sClient.Delete(ctx, clusterRole)
			}()

			Eventually(clusterRoleBinding).Should(k8sFixture.ExistByName())
			defer func() {
				_ = k8sClient.Delete(ctx, clusterRoleBinding)
			}()

			for _, serviceName := range serviceNames {

				By("verifying Service '" + serviceName + "' exists and is a LoadBalancer or ClusterIP depending on which service")

				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: ns.Name,
					},
				}
				Eventually(service).Should(k8sFixture.ExistByName())
				Expect(string(service.Spec.Type)).To(Equal("ClusterIP"))
			}

			for _, deploymentName := range deploymentNames {

				By("verifying Deployment '" + deploymentName + "' exists and is ready")

				depl := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deploymentName,
						Namespace: ns.Name,
					},
				}
				Eventually(depl).Should(k8sFixture.ExistByName())
			}

			By("verifying primary agent Deployment has expected values")

			Eventually(agentDeployment).Should(k8sFixture.ExistByName())
			Eventually(agentDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/component", "agent"))
			Eventually(agentDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/managed-by", argoCDName))
			Eventually(agentDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/name", argoCDAgentAgentName))
			Eventually(agentDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/part-of", "argocd-agent"))
		}

		// verifyResourcesDeleted will verify that the various resources that are created for agent are deleted.
		verifyResourcesDeleted := func() {

			By("verifying resources are deleted for agent pod")

			Eventually(serviceAccount).Should(k8sFixture.NotExistByName())
			Eventually(role).Should(k8sFixture.NotExistByName())
			Eventually(roleBinding).Should(k8sFixture.NotExistByName())
			Eventually(clusterRole).Should(k8sFixture.NotExistByName())
			Eventually(clusterRoleBinding).Should(k8sFixture.NotExistByName())
			Eventually(agentDeployment).Should(k8sFixture.NotExistByName())

			for _, serviceName := range []string{fmt.Sprintf("%s-agent-agent-metrics", argoCDName), fmt.Sprintf("%s-agent-agent-healthz", argoCDName)} {
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: ns.Name,
					},
				}
				Eventually(service).Should(k8sFixture.NotExistByName())
			}
		}

		It("should create argocd agent agent resources, but pod should not be expected to run successfully without principal", func() {
			// Change log level to trace and custom image name
			argoCD.Spec.ArgoCDAgent.Agent.Client.LogLevel = "trace"
			argoCD.Spec.ArgoCDAgent.Agent.Client.Image = "quay.io/user/argocd-agent:v1"

			By("Create ArgoCD instance")

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for agent pod")

			verifyExpectedResourcesExist(ns)

			By("Verify agent has the custom image we specified in ArgoCD CR")

			container := deploymentFixture.GetTemplateSpecContainerByName(argoCDAgentAgentName, *agentDeployment)
			Expect(container).ToNot(BeNil())
			Expect(container.Image).To(Equal("quay.io/user/argocd-agent:v1"))

			By("Verify environment variables are set correctly")

			// update expected value in default environment variables according to ArgoCD CR in the test
			expectedEnvVariables[agent.EnvArgoCDAgentLogLevel] = "trace"

			for key, value := range expectedEnvVariables {
				Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: key, Value: value}), "Environment variable %s should be set to %s", key, value)
			}

			By("Disable agent")

			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: argoCDName, Namespace: ns.Name}, argoCD)).To(Succeed())

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Agent.Enabled = ptr.To(false)
			})

			By("Verify agent resources are deleted")

			verifyResourcesDeleted()
		})

		It("should create argocd agent agent resources with default image, but pod will not start without principal", func() {

			// Add a custom environment variable to the agent client
			argoCD.Spec.ArgoCDAgent.Agent.Client.Env = []corev1.EnvVar{{Name: "TEST_ENV", Value: "test_value"}}

			By("Create ArgoCD instance")

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for agent pod")

			verifyExpectedResourcesExist(ns)

			By("Verify agent uses the default agent image")

			container := deploymentFixture.GetTemplateSpecContainerByName(argoCDAgentAgentName, *agentDeployment)
			Expect(container).ToNot(BeNil())
			Expect(container.Image).To(Equal("quay.io/argoprojlabs/argocd-agent:v0.3.2"))

			By("Verify environment variables are set correctly")

			for key, value := range expectedEnvVariables {
				Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: key, Value: value}), "Environment variable %s should be set to %s", key, value)
			}

			Expect(container.Env).To(ContainElement(And(
				HaveField("Name", agent.EnvRedisPassword),
				HaveField("ValueFrom.SecretKeyRef", Not(BeNil())),
			)), "REDIS_PASSWORD should be set with valueFrom.secretKeyRef")

			By("Verify custom environment variable is present")

			Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: "TEST_ENV", Value: "test_value"}), "Custom environment variable TEST_ENV should be set")

			By("Disable agent")

			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: argoCDName, Namespace: ns.Name}, argoCD)).To(Succeed())

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				ac.Spec.ArgoCDAgent.Agent.Enabled = nil
			})

			By("Verify agent resources are deleted")

			verifyResourcesDeleted()
		})

		It("Should reflect configuration changes from ArgoCD CR to the agent deployment", func() {

			By("Create ArgoCD instance")

			argoCD.Spec.ArgoCDAgent.Agent.Client.Image = "quay.io/jparsai/argocd-agent:test"
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for agent pod")

			verifyExpectedResourcesExist(ns)

			By("Verify agent has the custom image we specified in ArgoCD CR")

			container := deploymentFixture.GetTemplateSpecContainerByName(argoCDAgentAgentName, *agentDeployment)
			Expect(container).ToNot(BeNil())
			Expect(container.Image).To(Equal("quay.io/jparsai/argocd-agent:test"))

			By("Verify environment variables are set correctly")

			// update expected value in default environment variables according to ArgoCD CR in the test
			for key, value := range expectedEnvVariables {
				Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: key, Value: value}), "Environment variable %s should be set to %s", key, value)
			}

			By("Update ArgoCD CR with new configuration")

			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: argoCDName, Namespace: ns.Name}, argoCD)).To(Succeed())

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {

				ac.Spec.ArgoCDAgent.Agent.Client.LogLevel = "trace"
				ac.Spec.ArgoCDAgent.Agent.Client.LogFormat = "json"
				ac.Spec.ArgoCDAgent.Agent.Client.KeepAliveInterval = "60s"
				ac.Spec.ArgoCDAgent.Agent.Client.EnableWebSocket = ptr.To(true)
				ac.Spec.ArgoCDAgent.Agent.Client.EnableCompression = ptr.To(true)
				ac.Spec.ArgoCDAgent.Agent.Client.Image = "quay.io/jparsai/argocd-agent:test1"
				ac.Spec.ArgoCDAgent.Agent.Client.Mode = "autonomous"
				ac.Spec.ArgoCDAgent.Agent.Client.PrincipalServerAddress = "argocd-agent-principal-updated.example.com"
				ac.Spec.ArgoCDAgent.Agent.Client.PrincipalServerPort = "8443"

				ac.Spec.ArgoCDAgent.Agent.TLS.Insecure = ptr.To(true)
				ac.Spec.ArgoCDAgent.Agent.TLS.SecretName = "argocd-agent-client-tls-v2"
				ac.Spec.ArgoCDAgent.Agent.TLS.RootCASecretName = "argocd-agent-ca-v2"

			})

			By("Verify agent has the updated image we specified in ArgoCD CR")

			Eventually(agentDeployment).Should(k8sFixture.ExistByName())
			Eventually(
				func() bool {
					// Fetch the latest deployment from the cluster
					err := k8sClient.Get(ctx, client.ObjectKey{Name: argoCDAgentAgentName, Namespace: ns.Name}, agentDeployment)
					if err != nil {
						GinkgoWriter.Println("Error getting deployment for image check: ", err)
						return false
					}
					container = deploymentFixture.GetTemplateSpecContainerByName(argoCDAgentAgentName, *agentDeployment)
					if container == nil {
						return false
					}
					return container.Image == "quay.io/jparsai/argocd-agent:test1"
				}, "120s", "5s").Should(BeTrue(), "Agent deployment should have the updated image")

			By("Verify environment variables are updated correctly")

			expectedEnvVariables[agent.EnvArgoCDAgentLogLevel] = "trace"
			expectedEnvVariables[agent.EnvArgoCDAgentLogFormat] = "json"
			expectedEnvVariables[agent.EnvArgoCDAgentKeepAliveInterval] = "60s"
			expectedEnvVariables[agent.EnvArgoCDAgentEnableWebSocket] = "true"
			expectedEnvVariables[agent.EnvArgoCDAgentEnableCompression] = "true"
			expectedEnvVariables[agent.EnvArgoCDAgentMode] = "autonomous"
			expectedEnvVariables[agent.EnvArgoCDAgentServerAddress] = "argocd-agent-principal-updated.example.com"
			expectedEnvVariables[agent.EnvArgoCDAgentServerPort] = "8443"
			expectedEnvVariables[agent.EnvArgoCDAgentTLSInsecure] = "true"
			expectedEnvVariables[agent.EnvArgoCDAgentTLSSecretName] = "argocd-agent-client-tls-v2"
			expectedEnvVariables[agent.EnvArgoCDAgentTLSRootCASecretName] = "argocd-agent-ca-v2"

			for key, value := range expectedEnvVariables {
				Expect(container.Env).To(ContainElement(corev1.EnvVar{Name: key, Value: value}), "Environment variable %s should be set to %s", key, value)
			}
		})
	})
})
