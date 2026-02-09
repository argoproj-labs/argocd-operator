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
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	argocdv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"

	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	agentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/agent"
	appFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/application"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
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
	argoCDAgentInstanceNamePrincipal = "argocd-hub"
	argoCDAgentInstanceNameAgent     = "argocd-spoke"

	// Agent and Principal deployment names
	deploymentNameAgentPrincipal = "argocd-hub-agent-principal"
	deploymentNameAgent          = "argocd-spoke-agent-agent"

	// Names given to clusters in hub
	managedAgentClusterName    = "managed-cluster-in-hub"
	autonomousAgentClusterName = "autonomous-cluster-in-hub"

	// Application names
	applicationNameManagedAgent    = "app-managed"
	applicationNameAutonomousAgent = "app-autonomous"

	// AppProject names
	agentAppProjectName = "agent-app-project"

	// Namespaces hosting the principal and agent deployments
	namespaceAgentPrincipal  = "ns-hosting-principal"
	namespaceManagedAgent    = "ns-hosting-managed-agent"
	namespaceAutonomousAgent = "ns-hosting-autonomous-agent"

	// Namespaces hosting application resources in managed and autonomous clusters (e.g. this is where the deployments etc, are deployed by Argo CD)
	managedAgentApplicationNamespace    = "ns-hosting-app-in-managed-cluster"
	autonomousAgentApplicationNamespace = "ns-hosting-app-in-autonomous-cluster"

	// Secret names
	agentJWTSecretName              = "argocd-agent-jwt"
	agentPrincipalTLSSecretName     = "argocd-agent-principal-tls"
	agentRootCASecretName           = "argocd-agent-ca"
	agentClientTLSSecretName        = "argocd-agent-client-tls"
	agentResourceProxyTLSSecretName = "argocd-agent-resource-proxy-tls"
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
	Context("1-053_validate_argocd_agent_principal_connected_test", func() {
		var (
			k8sClient                         client.Client
			ctx                               context.Context
			cleanupFuncs                      []func()
			registerCleanup                   func(func())
			clusterRolePrincipal              *rbacv1.ClusterRole
			clusterRoleBindingPrincipal       *rbacv1.ClusterRoleBinding
			clusterRoleManagedAgent           *rbacv1.ClusterRole
			clusterRoleBindingManagedAgent    *rbacv1.ClusterRoleBinding
			clusterRoleAutonomousAgent        *rbacv1.ClusterRole
			clusterRoleBindingAutonomousAgent *rbacv1.ClusterRoleBinding
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

			clusterRolePrincipal = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-agent-principal", argoCDAgentInstanceNamePrincipal, namespaceAgentPrincipal),
				},
			}
			clusterRoleBindingPrincipal = &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-agent-principal", argoCDAgentInstanceNamePrincipal, namespaceAgentPrincipal),
				},
			}

			clusterRoleManagedAgent = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-agent-agent", argoCDAgentInstanceNameAgent, namespaceManagedAgent),
				},
			}
			clusterRoleBindingManagedAgent = &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-agent-agent", argoCDAgentInstanceNameAgent, namespaceManagedAgent),
				},
			}

			clusterRoleAutonomousAgent = &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-agent-agent", argoCDAgentInstanceNameAgent, namespaceAutonomousAgent),
				},
			}
			clusterRoleBindingAutonomousAgent = &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-%s-agent-agent", argoCDAgentInstanceNameAgent, namespaceAutonomousAgent),
				},
			}

			// Create required namespaces and cleanup functions
			_, cleanupFuncClusterManaged := fixture.CreateNamespaceWithCleanupFunc(managedAgentClusterName)
			registerCleanup(cleanupFuncClusterManaged)

			_, cleanupFuncClusterAutonomous := fixture.CreateNamespaceWithCleanupFunc(autonomousAgentClusterName)
			registerCleanup(cleanupFuncClusterAutonomous)

			_, cleanupFuncManagedApplication := fixture.CreateClusterScopedManagedNamespaceWithCleanupFunc(managedAgentApplicationNamespace, argoCDAgentInstanceNameAgent)
			registerCleanup(cleanupFuncManagedApplication)

			_, cleanupFuncAutonomousApplication := fixture.CreateClusterScopedManagedNamespaceWithCleanupFunc(autonomousAgentApplicationNamespace, argoCDAgentInstanceNameAgent)
			registerCleanup(cleanupFuncAutonomousApplication)
		})

		// This function checks principal logs to verify it has connected to both agents.
		validatePrincipalAndAgentConnection := func() {
			By("Verify principal is connected to the both agents")

			agentFixture.VerifyLogs(deploymentNameAgentPrincipal, namespaceAgentPrincipal, []string{
				fmt.Sprintf("Mapped cluster %s to agent %s", managedAgentClusterName, managedAgentClusterName),
				fmt.Sprintf("Mapped cluster %s to agent %s", autonomousAgentClusterName, autonomousAgentClusterName),
				fmt.Sprintf("Updated connection status to 'Successful' in Cluster: '%s' mapped with Agent: '%s'", managedAgentClusterName, managedAgentClusterName),
				fmt.Sprintf("Updated connection status to 'Successful' in Cluster: '%s' mapped with Agent: '%s'", autonomousAgentClusterName, autonomousAgentClusterName),
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

		// runRedisTest is based on redis_proxy_test.go E2E test in argocd-agent
		// - This test will verify argo cd resourcetree API shows child resources (e.g. pods), which is only possible if redis proxy is working as expected.
		runRedisTest := func(argoEndpoint string, password string, managedAgent bool, appOnPrincipal argocdv1alpha1.Application, agentK8sClient client.Client) {

			argocdClient, sessionToken, closer, err := argocdFixture.CreateArgoCDAPIClient(context.Background(), argoEndpoint, password)
			Expect(err).ToNot(HaveOccurred())
			defer closer.Close()

			closer, appClient, err := argocdClient.NewApplicationClient()
			Expect(err).ToNot(HaveOccurred())
			defer closer.Close()

			cancellableContext, cancelFunc := context.WithCancel(context.Background())
			defer cancelFunc()

			resourceTreeURL := "https://" + argoEndpoint + "/api/v1/stream/applications/" + appOnPrincipal.Name + "/resource-tree?appNamespace=" + appOnPrincipal.Namespace

			// Wait for successful connection to resource tree event source API, on principal Argo CD
			// - this allows us to stream an application's resource change events (e.g. pod created/deleted)
			var msgChan chan string
			Eventually(func() bool {
				var err error
				msgChan, err = argocdFixture.StreamFromArgoCDEventSourceURL(cancellableContext, resourceTreeURL, sessionToken)
				if err != nil {
					GinkgoWriter.Println("streamFromEventSource returned error:", err)
					return false
				}
				return true

			}, 5*time.Minute, 5*time.Second).Should(BeTrue())

			Expect(msgChan).ToNot(BeNil())

			deploymentNamespace := autonomousAgentApplicationNamespace
			if managedAgent {
				deploymentNamespace = managedAgentApplicationNamespace
			}

			// Find pod (deployed by Argo CD ) in agent deployment namespace
			var podList corev1.PodList
			Eventually(func() bool {
				err := agentK8sClient.List(context.Background(), &podList, client.InNamespace(deploymentNamespace))
				if err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				numPods := len(podList.Items)
				// should (only be) one guestbook pod
				if numPods != 1 {
					GinkgoWriter.Println("Waiting for 1 pods: ", numPods)
				}
				return numPods == 1

			}, "30s", "5s").Should(BeTrue())

			// Locate guestbook pod
			var oldPod corev1.Pod
			for idx := range podList.Items {
				pod := podList.Items[idx]
				if strings.Contains(pod.Name, "guestbook") {
					oldPod = pod
					break
				}
			}
			Expect(oldPod.Name).ToNot(BeEmpty())

			// Ensure that the pod appears in the resource tree value returned by Argo CD server (this will only be true if redis proxy is working)
			Eventually(func() bool {
				tree, err := appClient.ResourceTree(context.Background(), &application.ResourcesQuery{
					ApplicationName: &appOnPrincipal.Name,
					AppNamespace:    &appOnPrincipal.Namespace,
				})

				if err != nil {
					GinkgoWriter.Println("error on ResourceTree:", err)
					return false
				}
				if tree == nil {
					GinkgoWriter.Println("tree is nil")
					return false
				}

				for _, node := range tree.Nodes {
					if node.Kind == "Pod" && node.Name == oldPod.Name {
						return true
					}
				}
				return false
			}, time.Second*60, time.Second*5).Should(BeTrue())

			// Delete pod on managed agent cluster
			err = agentK8sClient.Delete(context.Background(), &oldPod)
			Expect(err).ToNot(HaveOccurred())

			// Wait for new pod to be created, to replace the old one that was deleted
			var newPod corev1.Pod
			Eventually(func() bool {
				var podList corev1.PodList
				err := agentK8sClient.List(context.Background(), &podList, client.InNamespace(deploymentNamespace))
				if err != nil {
					GinkgoWriter.Println("error on list:", err)
					return false
				}

				for idx := range podList.Items {
					pod := podList.Items[idx]
					if strings.Contains(pod.Name, "guestbook") && pod.Name != oldPod.Name {
						newPod = pod
						break
					}
				}

				return newPod.Name != ""

			}, time.Second*30, time.Second*5).Should(BeTrue())

			// Verify the name of the new pod exists in what has been sent from the channel (this will only be true if redis proxy subscription is working)
			Eventually(func() bool {
				for {
					// drain channel looking for name of new pod
					select {
					case msg := <-msgChan:
						GinkgoWriter.Println("Processing message:", msg)
						if strings.Contains(msg, newPod.Name) {
							GinkgoWriter.Println("new pod name found:", newPod.Name)
							return true
						}
					default:
						return false
					}
				}
			}, time.Second*30, time.Second*5).Should(BeTrue())

			// Ensure that the pod appears in the new resource tree value returned by Argo CD server
			tree, err := appClient.ResourceTree(context.Background(), &application.ResourcesQuery{
				ApplicationName: &appOnPrincipal.Name,
				AppNamespace:    &appOnPrincipal.Namespace,
				Project:         &appOnPrincipal.Spec.Project,
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(tree).ToNot(BeNil())

			matchFound := false
			for _, node := range tree.Nodes {
				if node.Kind == "Pod" && node.Name == newPod.Name {
					matchFound = true
					break
				}
			}
			Expect(matchFound).To(BeTrue())

		}

		// This test verifies that:
		// 1. A cluster-scoped ArgoCD instance with principal component enabled and a cluster-scoped ArgoCD instance
		// with agent component enabled are deployed in both "managed" and "autonomous" modes.
		// 2. Each agent successfully connects to the principal.
		// 3. Applications can be deployed in both modes, and are verified to be healthy and in sync.
		// 4. Redis proxy can be accessed, and it contains data from child resources (e.g. pod), for both managed, and autonomous.
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

			By("Create AppProject for managed agent in " + namespaceAgentPrincipal)
			Expect(k8sClient.Create(ctx, buildAppProjectResource(namespaceAgentPrincipal, argov1beta1api.AgentModeManaged))).To(Succeed())

			By("Create AppProject for autonomous agent in " + namespaceAutonomousAgent)
			Expect(k8sClient.Create(ctx, buildAppProjectResource(namespaceAutonomousAgent, argov1beta1api.AgentModeAutonomous))).To(Succeed())

			applicationOfManagedAgent := buildApplicationResource(applicationNameManagedAgent,
				managedAgentClusterName, managedAgentClusterName, argoCDAgentInstanceNameAgent, argov1beta1api.AgentModeManaged)
			By("Deploy application for managed mode")
			deployAndValidateApplication(applicationOfManagedAgent)

			portForwardCleanup := portForward(namespaceAgentPrincipal, "service/argocd-hub-server", "8443:https")
			cleanupFuncs = append(cleanupFuncs, portForwardCleanup)

			principalArgocdPassword := argocdFixture.GetInitialAdminSecretPassword(argoCDAgentInstanceNamePrincipal, namespaceAgentPrincipal, k8sClient)

			By("Running redis test for managed")
			runRedisTest("127.0.0.1:8443", principalArgocdPassword, true, *applicationOfManagedAgent, k8sClient)

			applicationOfAutonomousAgent := buildApplicationResource(applicationNameAutonomousAgent,
				namespaceAutonomousAgent, autonomousAgentClusterName, argoCDAgentInstanceNameAgent, argov1beta1api.AgentModeAutonomous)

			By("Deploy application for autonomous mode")
			deployAndValidateApplication(applicationOfAutonomousAgent)

			// The principal's application is the same as 'applicationOfAutonomousAgent', but in a different namespace. (The spec isn't needed)
			appOnPrincipal := applicationOfAutonomousAgent.DeepCopy()
			appOnPrincipal.Namespace = "autonomous-cluster-in-hub"
			appOnPrincipal.Spec = argocdv1alpha1.ApplicationSpec{}

			By("Running redis test for autonomous")
			runRedisTest("127.0.0.1:8443", principalArgocdPassword, false, *appOnPrincipal, k8sClient)

		})

		AfterEach(func() {

			fixture.OutputDebugOnFail(namespaceAgentPrincipal, namespaceManagedAgent, namespaceAutonomousAgent, managedAgentClusterName, autonomousAgentClusterName, managedAgentApplicationNamespace, autonomousAgentApplicationNamespace)

			By("Cleanup cluster-scoped resources")
			_ = k8sClient.Delete(ctx, clusterRolePrincipal)
			_ = k8sClient.Delete(ctx, clusterRoleBindingPrincipal)

			_ = k8sClient.Delete(ctx, clusterRoleManagedAgent)
			_ = k8sClient.Delete(ctx, clusterRoleBindingManagedAgent)

			_ = k8sClient.Delete(ctx, clusterRoleAutonomousAgent)
			_ = k8sClient.Delete(ctx, clusterRoleBindingAutonomousAgent)

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

	nsPrincipal, cleanup := fixture.CreateNamespaceWithCleanupFunc(namespaceAgentPrincipal)
	registerCleanup(cleanup)

	By("Create ArgoCD instance with principal component enabled")

	argoCDInstance := buildArgoCDResource(argoCDAgentInstanceNamePrincipal, argov1beta1api.AgentComponentTypePrincipal, "", nsPrincipal)
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
				Name:      deploymentNameAgentPrincipal,
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
		PrincipalNamespaceName:     namespaceAgentPrincipal,
		PrincipalServiceName:       deploymentNameAgentPrincipal,
		ResourceProxyServiceName:   fmt.Sprintf("%s-agent-principal-resource-proxy", argoCDAgentInstanceNamePrincipal),
		JWTSecretName:              agentJWTSecretName,
		PrincipalTLSSecretName:     agentPrincipalTLSSecretName,
		RootCASecretName:           agentRootCASecretName,
		ResourceProxyTLSSecretName: agentResourceProxyTLSSecretName,
		AdditionalPrincipalSANs:    additionalSANs,
	})

	By("Verify that principal deployment is in Ready state")

	Eventually(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      deploymentNameAgentPrincipal,
		Namespace: nsPrincipal.Name}}, "120s", "5s").Should(deploymentFixture.HaveReadyReplicas(1))

	By("Verify principal logs contain expected messages")

	agentFixture.VerifyLogs(deploymentNameAgentPrincipal, nsPrincipal.Name, principalStartupLogs)
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
		agentName = managedAgentClusterName
	} else {
		var cleanup func()
		nsAgent, cleanup = fixture.CreateNamespaceWithCleanupFunc(namespaceAutonomousAgent)
		registerCleanup(cleanup)
		agentName = autonomousAgentClusterName
	}

	By("Create required secrets for " + string(agentMode) + " agent")

	agentFixture.CreateRequiredAgentSecrets(agentFixture.AgentSecretsConfig{
		AgentNamespace:            nsAgent,
		PrincipalNamespaceName:    namespaceAgentPrincipal,
		PrincipalRootCASecretName: agentRootCASecretName,
		AgentRootCASecretName:     agentRootCASecretName,
		ClientTLSSecretName:       agentClientTLSSecretName,
		ClientCommonName:          agentName,
	})

	By("Create cluster registration secret for " + string(agentMode) + " agent")

	agentFixture.CreateClusterRegistrationSecret(agentFixture.ClusterRegistrationSecretConfig{
		PrincipalNamespaceName:    namespaceAgentPrincipal,
		AgentNamespaceName:        nsAgent.Name,
		AgentName:                 agentName,
		ResourceProxyServiceName:  fmt.Sprintf("%s-agent-principal-resource-proxy", argoCDAgentInstanceNamePrincipal),
		ResourceProxyPort:         9090,
		PrincipalRootCASecretName: agentRootCASecretName,
		AgentTLSSecretName:        agentClientTLSSecretName,
	})

	By("Deploy " + string(agentMode) + " agent ArgoCD instance")

	argoCDInstanceAgent := buildArgoCDResource(argoCDAgentInstanceNameAgent, argov1beta1api.AgentComponentTypeAgent, agentMode, nsAgent)
	// Set the principal server address
	argoCDInstanceAgent.Spec.ArgoCDAgent.Agent.Client.PrincipalServerAddress = fmt.Sprintf("%s.%s.svc", deploymentNameAgentPrincipal, namespaceAgentPrincipal)
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
							managedAgentClusterName,
							autonomousAgentClusterName,
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
				managedAgentClusterName,
				autonomousAgentClusterName,
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
						ServerAddress: fmt.Sprintf("%s-%s:%d", argoCDAgentInstanceNameAgent, "redis", common.ArgoCDDefaultRedisPort),
					},
					TLS: &argov1beta1api.AgentTLSSpec{
						SecretName:       agentClientTLSSecretName,
						RootCASecretName: agentRootCASecretName,
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
			Name:      agentAppProjectName,
			Namespace: nsName,
		},
		Spec: argocdv1alpha1.AppProjectSpec{
			ClusterResourceWhitelist: []argocdv1alpha1.ClusterResourceRestrictionItem{{
				Group: "*",
				Kind:  "*",
			}},
			SourceRepos: []string{"*"},
		},
	}

	if agentMode == argov1beta1api.AgentModeManaged {
		appProject.Spec.SourceNamespaces = []string{
			managedAgentClusterName,
			autonomousAgentClusterName,
		}
		appProject.Spec.Destinations = []argocdv1alpha1.ApplicationDestination{{
			Name:      managedAgentClusterName,
			Namespace: managedAgentApplicationNamespace,
			Server:    "*",
		}}
	} else {
		appProject.Spec.Destinations = []argocdv1alpha1.ApplicationDestination{{
			Namespace: autonomousAgentApplicationNamespace,
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
			Project: agentAppProjectName,
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
			Namespace: managedAgentApplicationNamespace,
		}
	} else {
		application.Spec.Destination = argocdv1alpha1.ApplicationDestination{
			Server:    "https://kubernetes.default.svc",
			Namespace: autonomousAgentApplicationNamespace,
		}
	}
	return application
}

func portForward(namespace string, subject string, port string) func() {

	cmdArgs := []string{"kubectl", "port-forward", "-n", namespace, subject, port}

	GinkgoWriter.Println("executing command:", cmdArgs)

	// #nosec G204
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	// Create pipes for stdout and stderr to stream output in real-time
	stdout, err := cmd.StdoutPipe()
	Expect(err).ToNot(HaveOccurred())

	stderr, err := cmd.StderrPipe()
	Expect(err).ToNot(HaveOccurred())

	// Channel to signal when port-forward is ready (after seeing "Forwarding from" messages)
	ready := make(chan struct{})

	// streamOutput reads from a pipe and writes to GinkgoWriter in real-time.
	// It signals readiness when it sees the expected "Forwarding from" message.
	streamOutput := func(pipe io.Reader, signalReady func()) {
		defer GinkgoRecover()

		// 'kubectl port-forward' will print this output indicating it has successfully started port-forwarding:
		// Forwarding from 127.0.0.1:8443 -> 8080
		// Forwarding from [::1]:8443 -> 8080

		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			line := scanner.Text()
			GinkgoWriter.Println("port-forward:", line)

			// Signal ready when we see the first "Forwarding from" message
			if signalReady != nil && strings.HasPrefix(line, "Forwarding from") {
				signalReady()
				signalReady = nil // Only signal once
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			GinkgoWriter.Println("port-forward scanner error:", scanErr)
		}
	}

	// Start the command
	err = cmd.Start()
	Expect(err).ToNot(HaveOccurred())

	// Stream stdout (with ready signaling) and stderr in separate goroutines
	go streamOutput(stdout, func() { close(ready) })
	go streamOutput(stderr, nil)

	// Wait for the process to complete in a separate goroutine
	go func() {
		defer GinkgoRecover()

		err := cmd.Wait()
		if err != nil && !strings.Contains(err.Error(), "killed") && !strings.Contains(err.Error(), "signal: killed") {
			GinkgoWriter.Println("port-forward process error:", err)
		}
	}()

	// Wait for the port-forward to be ready before returning
	select {
	case <-ready:
		GinkgoWriter.Println("port-forward is ready")
	case <-time.After(60 * time.Second):
		Fail("timed out waiting for port-forward to be ready")
	}

	return func() {

		GinkgoWriter.Println("terminating port forward")

		if cmd.Process != nil {
			err := cmd.Process.Kill()
			if err != nil && !strings.Contains(err.Error(), "process already finished") {
				GinkgoWriter.Println("error on process kill:", err)
			}
		}
	}

}
