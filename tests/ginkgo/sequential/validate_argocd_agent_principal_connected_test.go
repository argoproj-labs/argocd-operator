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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argocdv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	agentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/agent"
	appFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

/*
### Namespace Hierarchy for this test:

Test Cluster (Has a Hub and two Spokes (Managed and Autonomous) simulated)
│
├─ 🏛️ Hub Cluster
│   ├─ Namespace: ns-hosting-principal
│   │   ├─ ArgoCD: argocd-hub (Principal enabled)
│   │   ├─ Deployment: argocd-hub-agent-principal
│   │   ├─ Service: argocd-hub-agent-principal (Type LoadBalancer)
│   │   ├─ Secrets: TLS, JWT, CA, Cluster registration secrets
│   │   └─ AppProject: agent-app-project ("Source of truth" for managed agent, delivered to agent by principal)
│   │
│   ├─ Namespace: managed-cluster-in-hub (Logical representation of managed cluster in hub)
│   │   └─ Application: app-managed ("Source of truth" for managed agent, delivered to agent by principal)
│   │
│   │
│   └─ Namespace: autonomous-cluster-in-hub (Logical representation of autonomous cluster in hub)
|       └─ Application: app-autonomous ("Source of truth" is autonomous agent, delivered to principal by agent)
│
├─ 🔵 Managed Spoke Cluster
│   ├─ Namespace: ns-hosting-managed-agent
│   │   ├─ ArgoCD: argocd-spoke (Agent enabled, Managed mode)
│   │   ├─ Deployment: argocd-spoke-agent-agent
│   │   ├─ Secrets: Client TLS, CA
|   |   └─ Application: app-managed ("Source of truth" is principal, but Reconciled and deployed in spoke by agent)
│   │
│   └─ Namespace: ns-hosting-app-in-managed-cluster
│       └─ Pod/Service/Route: spring-petclinic (Application resources deployed by agent in spoke)
│
└─ 🔵 Autonomous Spoke Cluster
    ├─ Namespace: ns-hosting-autonomous-agent
    │   ├─ ArgoCD: argocd-spoke (Agent enabled, Autonomous mode)
    │   ├─ Deployment: argocd-spoke-agent-agent
    │   ├─ Secrets: Client TLS, CA
    │   ├─ AppProject: agent-app-project ("Source of truth" is autonomous agent, delivered to principal by agent)
    │   └─ Application: app-autonomous ("Source of truth" is autonomous agent, delivered to principal by agent, Reconciled and deployed in spoke by agent)
    │
    └─ Namespace: ns-hosting-app-in-autonomous-cluster
        └─ Pod/Service/Route: spring-petclinic (Application resources deployed by agent in spoke)
*/

const (
	// ArgoCD instance names
	argoCDInstanceNamePrincipal = "argocd-hub"
	argoCDInstanceNameAgent     = "argocd-spoke"

	// Agent and Principal deployment names
	deploymentNamePrincipal = "argocd-hub-agent-principal"
	deploymentNameAgent     = "argocd-spoke-agent-agent"

	// Names given to clusters in hub
	managedClusterName    = "managed-cluster-in-hub"
	autonomousClusterName = "autonomous-cluster-in-hub"

	// Application names
	applicationNameManaged    = "app-managed"
	applicationNameAutonomous = "app-autonomous"

	// AppProject names
	appProjectName = "agent-app-project"

	// Namespaces hosting the principal and agent deployments
	namespacePrincipal       = "ns-hosting-principal"
	namespaceManagedAgent    = "ns-hosting-managed-agent"
	namespaceAutonomousAgent = "ns-hosting-autonomous-agent"

	// Namespaces hosting application resources in managed and autonomous clusters
	managedApplicationNamespace    = "ns-hosting-app-in-managed-cluster"
	autonomousApplicationNamespace = "ns-hosting-app-in-autonomous-cluster"

	// Secret names
	JWTSecretName              = "argocd-agent-jwt"
	principalTLSSecretName     = "argocd-agent-principal-tls"
	rootCASecretName           = "argocd-agent-ca"
	clientTLSSecretName        = "argocd-agent-client-tls"
	resourceProxyTLSSecretName = "argocd-agent-resource-proxy-tls"
)

var (
	principalStartupLogs = []string{
		"Starting metrics server",
		"Redis proxy started",
		"Application informer synced and ready",
		"AppProject informer synced and ready",
		"Resource proxy started",
		"Namespace informer synced and ready",
		"Starting healthz server",
	}

	agentStartupLogs = []string{
		"Starting metrics server",
		"Starting healthz server",
		"Authentication successful",
		"Connected to argocd-agent",
		"Starting event writer",
		"Starting to send events to event stream",
		"Starting to receive events from event stream",
	}
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {
	Context("validate_argocd_agent_principal_connected", func() {
		var (
			k8sClient       client.Client
			ctx             context.Context
			cleanupFuncs    []func()
			registerCleanup func(func())
		)

		BeforeEach(func() {
			fixture.EnsureSequentialCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
			cleanupFuncs = nil
			registerCleanup = func(fn func()) {
				if fn != nil {
					cleanupFuncs = append(cleanupFuncs, fn)
				}
			}

			// Create required namespaces and cleanup functions
			_, cleanupFuncClusterManaged := fixture.CreateNamespaceWithCleanupFunc(managedClusterName)
			registerCleanup(cleanupFuncClusterManaged)

			_, cleanupFuncClusterAutonomous := fixture.CreateNamespaceWithCleanupFunc(autonomousClusterName)
			registerCleanup(cleanupFuncClusterAutonomous)

			_, cleanupFuncManagedApplication := fixture.CreateClusterScopedManagedNamespaceWithCleanupFunc(managedApplicationNamespace, argoCDInstanceNameAgent)
			registerCleanup(cleanupFuncManagedApplication)

			_, cleanupFuncAutonomousApplication := fixture.CreateClusterScopedManagedNamespaceWithCleanupFunc(autonomousApplicationNamespace, argoCDInstanceNameAgent)
			registerCleanup(cleanupFuncAutonomousApplication)
		})

		// This function checks principal logs to verify it has connected to both agents.
		validatePrincipalAndAgentConnection := func() {
			By("Verify principal is connected to the both agents")

			agentFixture.VerifyLogs(deploymentNamePrincipal, namespacePrincipal, []string{
				fmt.Sprintf("Mapped cluster %s to agent %s", managedClusterName, managedClusterName),
				fmt.Sprintf("Mapped cluster %s to agent %s", autonomousClusterName, autonomousClusterName),
				fmt.Sprintf("Updated connection status to 'Successful' in Cluster: '%s' mapped with Agent: '%s'", managedClusterName, managedClusterName),
				fmt.Sprintf("Updated connection status to 'Successful' in Cluster: '%s' mapped with Agent: '%s'", autonomousClusterName, autonomousClusterName),
				"Processing clusterCacheInfoUpdate event",
				"Updated cluster cache stats in cluster.",
			})
		}

		// This function deploys an application and verifies it is healthy and synced.
		deployAndValidateApplication := func(application *argocdv1alpha1.Application) {

			By("Deploy application: " + application.Name + " in namespace: " + application.Namespace)
			Expect(k8sClient.Create(ctx, application)).To(Succeed())

			By("Verify application: " + application.Name + " in namespace: " + application.Namespace + " is healthy and synced")
			Eventually(application, "180s", "5s").Should(appFixture.HaveSyncStatusCode(argocdv1alpha1.SyncStatusCodeSynced), "Application should be synced")
			Eventually(application, "180s", "5s").Should(appFixture.HaveHealthStatusCode(health.HealthStatusHealthy), "Application should be healthy")
		}

		// This test verifies that:
		// 1. A cluster-scoped ArgoCD instance with principal component enabled and a cluster-scoped ArgoCD instance
		// with agent component enabled are deployed in both "managed" and "autonomous" modes.
		// 2. Each agent successfully connects to the principal.
		// 3. Applications can be deployed in both modes, and are verified to be healthy and in sync.
		// This validates the core connectivity and basic workflow of agent-principal architecture, including RBAC, connection, and application propagation.
		It("Should deploy ArgoCD principal and agent instances in both modes and verify they are working as expected", func() {

			By("Deploy principal and verify it starts successfully")
			deployPrincipal(ctx, k8sClient, registerCleanup)

			By("Deploy managed agent and verify it starts successfully")
			deployAgent(ctx, k8sClient, registerCleanup, argov1beta1api.AgentModeManaged)

			By("Deploy autonomous agent and verify it starts successfully")
			deployAgent(ctx, k8sClient, registerCleanup, argov1beta1api.AgentModeAutonomous)

			By("Validate both agents are connected to the principal")
			validatePrincipalAndAgentConnection()

			By("Create AppProject for managed agent in " + namespacePrincipal)
			Expect(k8sClient.Create(ctx, buildAppProjectResource(namespacePrincipal, argov1beta1api.AgentModeManaged))).To(Succeed())

			By("Create AppProject for autonomous agent in " + namespaceAutonomousAgent)
			Expect(k8sClient.Create(ctx, buildAppProjectResource(namespaceAutonomousAgent, argov1beta1api.AgentModeAutonomous))).To(Succeed())

			By("Deploy application for managed mode")
			deployAndValidateApplication(buildApplicationResource(applicationNameManaged,
				managedClusterName, managedClusterName, argoCDInstanceNameAgent, argov1beta1api.AgentModeManaged))

			By("Deploy application for autonomous mode")
			deployAndValidateApplication(buildApplicationResource(applicationNameAutonomous,
				namespaceAutonomousAgent, autonomousClusterName, argoCDInstanceNameAgent, argov1beta1api.AgentModeAutonomous))
		})

		AfterEach(func() {
			By("Cleanup namespaces created in this test")
			for i := len(cleanupFuncs) - 1; i >= 0; i-- {
				cleanupFuncs[i]()
			}
		})

	})
})

// This function deploys the principal ArgoCD instance and waits for it to be ready.
// It creates the required secrets for the principal and verifies that the principal deployment is in Ready state.
// It also verifies that the principal logs contain the expected messages.
func deployPrincipal(ctx context.Context, k8sClient client.Client, registerCleanup func(func())) {
	GinkgoHelper()

	nsPrincipal, cleanup := fixture.CreateNamespaceWithCleanupFunc(namespacePrincipal)
	registerCleanup(cleanup)

	By("Create ArgoCD instance with principal component enabled")

	argoCDInstance := buildArgoCDResource(argoCDInstanceNamePrincipal, argov1beta1api.AgentComponentTypePrincipal, "", nsPrincipal)
	waitForLoadBalancer := true
	if !fixture.RunningOnOpenShift() {
		argoCDInstance.Spec.ArgoCDAgent.Principal.Server.Service.Type = corev1.ServiceTypeClusterIP
		waitForLoadBalancer = false
	}

	Expect(k8sClient.Create(ctx, argoCDInstance)).To(Succeed())

	By("Wait for principal service to be ready and use LoadBalancer hostname/IP when available")

	additionalSANs := []string{}
	if waitForLoadBalancer {
		principalService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentNamePrincipal,
				Namespace: nsPrincipal.Name,
			},
		}

		err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
			if pollErr := k8sClient.Get(ctx, client.ObjectKeyFromObject(principalService), principalService); pollErr != nil {
				return false, nil
			}

			for _, ingress := range principalService.Status.LoadBalancer.Ingress {
				switch {
				case ingress.Hostname != "":
					additionalSANs = append(additionalSANs, ingress.Hostname)
					return true, nil
				case ingress.IP != "":
					additionalSANs = append(additionalSANs, ingress.IP)
					return true, nil
				}
			}
			return false, nil
		})
		if err != nil {
			GinkgoWriter.Println("LoadBalancer ingress not available, proceeding without external SANs:", err)
		}
	} else {
		GinkgoWriter.Println("Cluster does not support LoadBalancer services; using in-cluster service DNS SANs only")
	}

	By("Create required secrets for principal")

	agentFixture.CreateRequiredSecrets(agentFixture.PrincipalSecretsConfig{
		PrincipalNamespaceName:     namespacePrincipal,
		PrincipalServiceName:       deploymentNamePrincipal,
		ResourceProxyServiceName:   fmt.Sprintf("%s-agent-principal-resource-proxy", argoCDInstanceNamePrincipal),
		JWTSecretName:              JWTSecretName,
		PrincipalTLSSecretName:     principalTLSSecretName,
		RootCASecretName:           rootCASecretName,
		ResourceProxyTLSSecretName: resourceProxyTLSSecretName,
		AdditionalPrincipalSANs:    additionalSANs,
	})

	By("Verify that principal deployment is in Ready state")

	Eventually(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      deploymentNamePrincipal,
		Namespace: nsPrincipal.Name}}, "120s", "5s").Should(deploymentFixture.HaveReadyReplicas(1))

	By("Verify principal logs contain expected messages")

	agentFixture.VerifyLogs(deploymentNamePrincipal, nsPrincipal.Name, principalStartupLogs)
}

// This function deploys the agent ArgoCD instance and waits for it to be ready.
// It creates the required secrets for the agent and verifies that the agent deployment is in Ready state.
// It also verifies that the agent logs contain the expected messages.
func deployAgent(ctx context.Context, k8sClient client.Client, registerCleanup func(func()), agentMode argov1beta1api.AgentMode) {
	GinkgoHelper()

	var (
		nsAgent   *corev1.Namespace
		agentName string
	)

	if agentMode == argov1beta1api.AgentModeManaged {
		var cleanup func()
		nsAgent, cleanup = fixture.CreateNamespaceWithCleanupFunc(namespaceManagedAgent)
		registerCleanup(cleanup)
		agentName = managedClusterName
	} else {
		var cleanup func()
		nsAgent, cleanup = fixture.CreateNamespaceWithCleanupFunc(namespaceAutonomousAgent)
		registerCleanup(cleanup)
		agentName = autonomousClusterName
	}

	By("Create required secrets for " + string(agentMode) + " agent")

	agentFixture.CreateRequiredAgentSecrets(agentFixture.AgentSecretsConfig{
		AgentNamespace:            nsAgent,
		PrincipalNamespaceName:    namespacePrincipal,
		PrincipalRootCASecretName: rootCASecretName,
		AgentRootCASecretName:     rootCASecretName,
		ClientTLSSecretName:       clientTLSSecretName,
		ClientCommonName:          agentName,
	})

	By("Create cluster registration secret for " + string(agentMode) + " agent")

	agentFixture.CreateClusterRegistrationSecret(agentFixture.ClusterRegistrationSecretConfig{
		PrincipalNamespaceName:    namespacePrincipal,
		AgentNamespaceName:        nsAgent.Name,
		AgentName:                 agentName,
		ResourceProxyServiceName:  fmt.Sprintf("%s-agent-principal-resource-proxy", argoCDInstanceNamePrincipal),
		ResourceProxyPort:         9090,
		PrincipalRootCASecretName: rootCASecretName,
		AgentTLSSecretName:        clientTLSSecretName,
	})

	By("Deploy " + string(agentMode) + " agent ArgoCD instance")

	argoCDInstanceAgent := buildArgoCDResource(argoCDInstanceNameAgent, argov1beta1api.AgentComponentTypeAgent, agentMode, nsAgent)
	// Set the principal server address
	argoCDInstanceAgent.Spec.ArgoCDAgent.Agent.Client.PrincipalServerAddress = fmt.Sprintf("%s.%s.svc", deploymentNamePrincipal, namespacePrincipal)
	Expect(k8sClient.Create(ctx, argoCDInstanceAgent)).To(Succeed())

	By("Verifying " + string(agentMode) + " agent deployment is in Ready state")

	Eventually(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentNameAgent, Namespace: nsAgent.Name}}, "120s", "5s").
		Should(deploymentFixture.HaveReadyReplicas(1))

	By("Verifying " + string(agentMode) + " agent logs contain expected messages")

	agentFixture.VerifyLogs(deploymentNameAgent, nsAgent.Name, agentStartupLogs)
}

// This function builds the ArgoCD instance for the principal or agent based on the component name.
func buildArgoCDResource(argoCDName string, componentType argov1beta1api.AgentComponentType,
	agentMode argov1beta1api.AgentMode, ns *corev1.Namespace) *argov1beta1api.ArgoCD {

	argoCD := &argov1beta1api.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      argoCDName,
			Namespace: ns.Name,
		},
	}

	// Principal configurations
	if componentType == argov1beta1api.AgentComponentTypePrincipal {
		argoCD.Spec = argov1beta1api.ArgoCDSpec{
			Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
				Enabled: ptr.To(false),
			},
			ArgoCDAgent: &argov1beta1api.ArgoCDAgentSpec{
				Principal: &argov1beta1api.PrincipalSpec{
					Enabled:  ptr.To(true),
					Auth:     "mtls:CN=([^,]+)",
					LogLevel: "info",
					Image:    common.ArgoCDAgentPrincipalDefaultImageName,
					Namespace: &argov1beta1api.PrincipalNamespaceSpec{
						AllowedNamespaces: []string{
							managedClusterName,
							autonomousClusterName,
						},
					},
					TLS: &argov1beta1api.PrincipalTLSSpec{
						InsecureGenerate: ptr.To(false),
					},
					JWT: &argov1beta1api.PrincipalJWTSpec{
						InsecureGenerate: ptr.To(false),
					},
					Server: &argov1beta1api.PrincipalServerSpec{
						KeepAliveMinInterval: "30s",
						Route: argov1beta1api.ArgoCDAgentPrincipalRouteSpec{
							Enabled: ptr.To(false),
						},
						Service: argov1beta1api.ArgoCDAgentPrincipalServiceSpec{
							Type: corev1.ServiceTypeLoadBalancer,
						},
					},
				},
				Agent: &argov1beta1api.AgentSpec{
					Enabled: ptr.To(false),
				},
			},
			SourceNamespaces: []string{
				managedClusterName,
				autonomousClusterName,
			},
		}
	} else {
		// Agent configurations
		argoCD.Spec = argov1beta1api.ArgoCDSpec{
			Server: argov1beta1api.ArgoCDServerSpec{
				Enabled: ptr.To(false),
			},
			ArgoCDAgent: &argov1beta1api.ArgoCDAgentSpec{
				Principal: &argov1beta1api.PrincipalSpec{
					Enabled: ptr.To(false),
				},
				Agent: &argov1beta1api.AgentSpec{
					Enabled:  ptr.To(true),
					Creds:    "mtls:any",
					LogLevel: "info",
					Image:    common.ArgoCDAgentAgentDefaultImageName,
					Client: &argov1beta1api.AgentClientSpec{
						PrincipalServerAddress: "", // will be set in the test
						PrincipalServerPort:    "443",
						KeepAliveInterval:      "50s",
						Mode:                   string(agentMode),
					},
					Redis: &argov1beta1api.AgentRedisSpec{
						ServerAddress: fmt.Sprintf("%s-%s:%d", argoCDInstanceNameAgent, "redis", common.ArgoCDDefaultRedisPort),
					},
					TLS: &argov1beta1api.AgentTLSSpec{
						SecretName:       clientTLSSecretName,
						RootCASecretName: rootCASecretName,
						Insecure:         ptr.To(false),
					},
				},
			},
		}
	}

	return argoCD
}

// This function builds the AppProject resource for the managed or autonomous agent.
func buildAppProjectResource(nsName string, agentMode argov1beta1api.AgentMode) *argocdv1alpha1.AppProject {
	appProject := &argocdv1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appProjectName,
			Namespace: nsName,
		},
		Spec: argocdv1alpha1.AppProjectSpec{
			ClusterResourceWhitelist: []metav1.GroupKind{{
				Group: "*",
				Kind:  "*",
			}},
			SourceRepos: []string{"*"},
		},
	}

	if agentMode == argov1beta1api.AgentModeManaged {
		appProject.Spec.SourceNamespaces = []string{
			managedClusterName,
			autonomousClusterName,
		}
		appProject.Spec.Destinations = []argocdv1alpha1.ApplicationDestination{{
			Name:      managedClusterName,
			Namespace: managedApplicationNamespace,
			Server:    "*",
		}}
	} else {
		appProject.Spec.Destinations = []argocdv1alpha1.ApplicationDestination{{
			Namespace: autonomousApplicationNamespace,
			Server:    "*",
		}}
	}
	return appProject
}

// This function builds the Application resource for the managed or autonomous agent.
func buildApplicationResource(applicationName, nsName, agentName, argocdInstanceName string,
	agentMode argov1beta1api.AgentMode) *argocdv1alpha1.Application {

	application := &argocdv1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      applicationName,
			Namespace: nsName,
		},
		Spec: argocdv1alpha1.ApplicationSpec{
			Project: appProjectName,
			Source: &argocdv1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/argoproj/argocd-example-apps",
				TargetRevision: "HEAD",
				Path:           "guestbook",
			},
			SyncPolicy: &argocdv1alpha1.SyncPolicy{
				Automated: &argocdv1alpha1.SyncPolicyAutomated{
					Prune:    true,
					SelfHeal: true,
				},
				ManagedNamespaceMetadata: &argocdv1alpha1.ManagedNamespaceMetadata{
					Labels: map[string]string{
						"argocd.argoproj.io/managed-by": argocdInstanceName,
					},
				},
			},
		},
	}

	// Set the destination based on the agent mode
	if agentMode == argov1beta1api.AgentModeManaged {
		application.Spec.Destination = argocdv1alpha1.ApplicationDestination{
			Name:      agentName,
			Namespace: managedApplicationNamespace,
		}
	} else {
		application.Spec.Destination = argocdv1alpha1.ApplicationDestination{
			Server:    "https://kubernetes.default.svc",
			Namespace: autonomousApplicationNamespace,
		}
	}
	return application
}
