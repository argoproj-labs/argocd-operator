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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocdagent"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Sequential E2E Tests", func() {

	const (
		argoCDName               = "example"
		argoCDAgentPrincipalName = "example-agent-principal" // argoCDName + "-agent-principal"
	)

	Context("1-051_validate_argocd_agent_principal", func() {

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
			principalDeployment  *appsv1.Deployment
			expectedEnvVariables map[string]string
			secretNames          []string
			principalRoute       *routev1.Route
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
							Enabled: ptr.To(true),
							Server: &argov1beta1api.PrincipalServerSpec{
								Auth:     "mtls:CN=([^,]+)",
								LogLevel: "info",
							},
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
			secretNames = []string{
				"argocd-agent-jwt",
				"argocd-agent-principal-tls",
				"argocd-agent-ca",
				"argocd-agent-resource-proxy-tls",
				"example-redis-initial-password",
			}

			serviceNames = []string{argoCDAgentPrincipalName, fmt.Sprintf("%s-agent-principal-metrics", argoCDName), fmt.Sprintf("%s-redis", argoCDName), fmt.Sprintf("%s-repo-server", argoCDName), fmt.Sprintf("%s-server", argoCDName), fmt.Sprintf("%s-agent-principal-redisproxy", argoCDName), fmt.Sprintf("%s-agent-principal-resource-proxy", argoCDName), fmt.Sprintf("%s-agent-principal-healthz", argoCDName)}
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
				argocdagent.EnvArgoCDPrincipalTLSSecretName:             "argocd-agent-principal-tls",
				argocdagent.EnvArgoCDPrincipalTLSServerRootCASecretName: "argocd-agent-ca",
				argocdagent.EnvArgoCDPrincipalResourceProxySecretName:   "argocd-agent-resource-proxy-tls",
				argocdagent.EnvArgoCDPrincipalResourceProxyCaSecretName: "argocd-agent-ca",
				argocdagent.EnvArgoCDPrincipalJwtSecretName:             "argocd-agent-jwt",
			}
		})

		AfterEach(func() {
			By("Cleanup namespace")
			if cleanupFunc != nil {
				cleanupFunc()
			}
		})

		// generateTLSCertificateAndJWTKey creates a self-signed certificate and JWT signing key for testing
		generateTLSCertificateAndJWTKey := func() ([]byte, []byte, []byte, error) {
			// Generate private key for TLS certificate
			privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				GinkgoWriter.Println("Error generating private key: ", err)
				return nil, nil, nil, err
			}

			// Create certificate template
			template := x509.Certificate{
				SerialNumber: big.NewInt(1),
				Subject: pkix.Name{
					CommonName: "test",
				},
				NotBefore:   time.Now(),
				NotAfter:    time.Now().Add(10 * time.Minute),
				KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
				ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
			}

			// Create certificate
			certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
			if err != nil {
				GinkgoWriter.Println("Error creating certificate: ", err)
				return nil, nil, nil, err
			}

			// Encode certificate to PEM
			certPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "CERTIFICATE",
				Bytes: certDER,
			})

			// Encode private key to PEM
			privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
			if err != nil {
				GinkgoWriter.Println("Error marshalling private key: ", err)
				return nil, nil, nil, err
			}

			keyPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: privateKeyDER,
			})

			// Generate separate RSA private key for JWT signing
			jwtPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				GinkgoWriter.Println("Error generating JWT signing key: ", err)
				return nil, nil, nil, err
			}

			// Encode JWT private key to PEM format
			jwtPrivateKeyDER, err := x509.MarshalPKCS8PrivateKey(jwtPrivateKey)
			if err != nil {
				GinkgoWriter.Println("Error marshalling JWT signing key: ", err)
				return nil, nil, nil, err
			}

			jwtKeyPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: jwtPrivateKeyDER,
			})

			return certPEM, keyPEM, jwtKeyPEM, nil
		}

		// createRequiredSecrets creates all the secrets needed for the principal pod to start properly
		createRequiredSecrets := func(ns *corev1.Namespace) {

			By("creating required secrets for principal pod")

			// Generate TLS certificate and JWT signing key
			certPEM, keyPEM, jwtKeyPEM, err := generateTLSCertificateAndJWTKey()
			Expect(err).ToNot(HaveOccurred())

			// Create argocd-agent-jwt secret
			jwtSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretNames[0],
					Namespace: ns.Name,
				},
				Data: map[string][]byte{
					"jwt.key": jwtKeyPEM,
				},
			}
			Expect(k8sClient.Create(ctx, jwtSecret)).To(Succeed())

			// Create TLS secrets
			for i := 1; i <= 3; i++ {
				tlsSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretNames[i],
						Namespace: ns.Name,
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						"tls.crt": certPEM,
						"tls.key": keyPEM,
					},
				}
				Expect(k8sClient.Create(ctx, tlsSecret)).To(Succeed())
			}
		}

		// verifyExpectedResourcesExist will verify that the resources that are created for principal and ArgoCD are created.
		// expectRoute is optional - defaults to true if not provided
		verifyExpectedResourcesExist := func(ns *corev1.Namespace, expectRoute ...bool) {
			shouldExpectRoute := true
			if len(expectRoute) > 0 {
				shouldExpectRoute = expectRoute[0]
			}

			By("verifying expected resources exist")
			Eventually(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretNames[4], Namespace: ns.Name,
				}}, "30s", "2s").Should(k8sFixture.ExistByName())
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
				Eventually(service).Should(k8sFixture.ExistByName(), "Service '%s' should exist in namespace '%s'", serviceName, ns.Name)

				// skip principal service
				if serviceName != argoCDAgentPrincipalName {
					Expect(string(service.Spec.Type)).To(Equal("ClusterIP"), "Service '%s' should have ClusterIP type, got '%s'", serviceName, service.Spec.Type)
				}
			}

			if shouldExpectRoute {
				// Check if running on OpenShift and route should exist
				if fixture.RunningOnOpenShift() {
					By("verifying Route for principal exists on OpenShift")
					Eventually(principalRoute).Should(k8sFixture.ExistByName())
				}
			}

			for _, deploymentName := range deploymentNames {
				depl := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deploymentName,
						Namespace: ns.Name,
					},
				}
				Eventually(depl).Should(k8sFixture.ExistByName(), "Deployment '%s' should exist in namespace '%s'", deploymentName, ns.Name)
			}

			By("verifying primary principal Deployment has expected values")

			Eventually(principalDeployment).Should(k8sFixture.ExistByName())
			Eventually(principalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/component", "principal"))
			Eventually(principalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/managed-by", argoCDName))
			Eventually(principalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/name", argoCDAgentPrincipalName))
			Eventually(principalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/part-of", "argocd-agent"))
		}

		// verifyResourcesDeleted will verify that the various resources that are created for principal are deleted.
		verifyResourcesDeleted := func() {

			By("verifying resources are deleted for principal pod")

			Eventually(serviceAccount).Should(k8sFixture.NotExistByName())
			Eventually(role).Should(k8sFixture.NotExistByName())
			Eventually(roleBinding).Should(k8sFixture.NotExistByName())
			Eventually(clusterRole).Should(k8sFixture.NotExistByName())
			Eventually(clusterRoleBinding).Should(k8sFixture.NotExistByName())
			Eventually(principalDeployment).Should(k8sFixture.NotExistByName())

			for _, serviceName := range []string{argoCDAgentPrincipalName, fmt.Sprintf("%s-agent-principal-metrics", argoCDName), fmt.Sprintf("%s-agent-principal-redisproxy", argoCDName), fmt.Sprintf("%s-agent-principal-resource-proxy", argoCDName), fmt.Sprintf("%s-agent-principal-healthz", argoCDName)} {
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: ns.Name,
					},
				}
				Eventually(service).Should(k8sFixture.NotExistByName(), "Service '%s' should not exist in namespace '%s'", serviceName, ns.Name)
			}

			// Verify route is deleted on OpenShift
			if fixture.RunningOnOpenShift() {
				Eventually(principalRoute).Should(k8sFixture.NotExistByName())
			}
		}

		It("should create argocd agent principal resources, but pod should fail to start as image does not exist", func() {
			// Change log level to trace and custom image name
			argoCD.Spec.ArgoCDAgent.Principal.Server.LogLevel = "trace"
			argoCD.Spec.ArgoCDAgent.Principal.Server.Image = "quay.io/user/argocd-agent:v1"

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
			argoCD.Spec.ArgoCDAgent.Principal.Server.Env = []corev1.EnvVar{{Name: "TEST_ENV", Value: "test_value"}}

			By("Create ArgoCD instance")

			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns)

			By("Verify principal uses the default agent image")

			container := deploymentFixture.GetTemplateSpecContainerByName(argoCDAgentPrincipalName, *principalDeployment)
			Expect(container).ToNot(BeNil())
			Expect(container.Image).To(Equal("quay.io/argoprojlabs/argocd-agent:v0.3.2"))

			By("Create required secrets and certificates for principal pod to start properly")

			createRequiredSecrets(ns)

			By("Verify principal pod starts successfully by checking logs")

			Eventually(func() bool {
				logOutput, err := osFixture.ExecCommandWithOutputParam(false, "kubectl", "logs",
					"deployment/"+argoCDAgentPrincipalName, "-n", ns.Name, "--tail=200")
				if err != nil {
					GinkgoWriter.Println("Error getting logs: ", err)
					return false
				}

				expectedMessages := []string{
					"Starting metrics server",
					"Redis proxy started",
					"Application informer synced and ready",
					"AppProject informer synced and ready",
					"Resource proxy started",
					"Namespace informer synced and ready",
					"Starting healthz server",
				}

				for _, message := range expectedMessages {
					if !strings.Contains(logOutput, message) {
						GinkgoWriter.Println("Expected message: '", message, "' not found in logs")
						return false
					}
				}
				return true
			}, "180s", "5s").Should(BeTrue(), "Pod should start successfully")

			By("verify that deployment is in Ready state")

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: argoCDAgentPrincipalName, Namespace: ns.Name}, principalDeployment)
				if err != nil {
					GinkgoWriter.Println("Error getting deployment: ", err)
					return false
				}
				return principalDeployment.Status.ReadyReplicas == 1
			}, "120s", "5s").Should(BeTrue(), "Principal deployment should become ready")

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

			argoCD.Spec.ArgoCDAgent.Principal.Server.Image = "quay.io/jparsai/argocd-agent:test"
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("Verify expected resources are created for principal pod")

			verifyExpectedResourcesExist(ns)

			By("Verify principal has the custom image we specified in ArgoCD CR")

			container := deploymentFixture.GetTemplateSpecContainerByName(argoCDAgentPrincipalName, *principalDeployment)
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

				ac.Spec.ArgoCDAgent.Principal.Server.LogLevel = "trace"
				ac.Spec.ArgoCDAgent.Principal.Server.LogFormat = "json"
				ac.Spec.ArgoCDAgent.Principal.Server.KeepAliveMinInterval = "60s"
				ac.Spec.ArgoCDAgent.Principal.Server.EnableWebSocket = ptr.To(true)
				ac.Spec.ArgoCDAgent.Principal.Server.Image = "quay.io/jparsai/argocd-agent:test1"

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
			secretNames = []string{"argocd-agent-jwt-v2", "argocd-agent-principal-tls-v2", "argocd-agent-ca-v2", "argocd-agent-resource-proxy-tls-v2"}
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
					return container.Image == "quay.io/jparsai/argocd-agent:test1"
				}, "120s", "5s").Should(BeTrue(), "Principal deployment should have the updated image")

			By("verify that deployment is in Ready state")

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: argoCDAgentPrincipalName, Namespace: ns.Name}, principalDeployment)
				if err != nil {
					GinkgoWriter.Println("Error getting deployment: ", err)
					return false
				}
				return principalDeployment.Status.ReadyReplicas == 1
			}, "120s", "5s").Should(BeTrue(), "Principal deployment should become ready")

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
				fmt.Sprintf("%s-agent-principal-metrics", argoCDName),
				fmt.Sprintf("%s-agent-principal-redisproxy", argoCDName),
				fmt.Sprintf("%s-agent-principal-resource-proxy", argoCDName),
				fmt.Sprintf("%s-agent-principal-healthz", argoCDName),
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
	})
})
