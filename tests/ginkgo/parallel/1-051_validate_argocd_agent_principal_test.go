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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	deploymentFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deployment"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-051_validate_argocd_agent_principal", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		// verifyExpectedResourcesExist will verify that the various K8s resources that are created for principal-configured agent, and Argo CD, are created.
		verifyExpectedResourcesExist := func(ns *corev1.Namespace) *appsv1.Deployment {
			By("verifying expected resources exist")
			serviceAccount := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-agent-principal",
					Namespace: ns.Name,
				},
			}
			Eventually(serviceAccount).Should(k8sFixture.ExistByName())

			role := &rbacv1.Role{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-agent-principal",
					Namespace: ns.Name,
				},
			}
			Eventually(role).Should(k8sFixture.ExistByName())

			roleBinding := &rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-agent-principal",
					Namespace: ns.Name,
				},
			}
			Eventually(roleBinding).Should(k8sFixture.ExistByName())

			clusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argocd-argocd-agent-principal-1-051-agent-principal",
				},
			}
			Eventually(clusterRole).Should(k8sFixture.ExistByName())
			defer func() {
				_ = k8sClient.Delete(ctx, clusterRole)
			}()

			clusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "argocd-argocd-agent-principal-1-051-agent-principal",
				},
			}
			Eventually(clusterRoleBinding).Should(k8sFixture.ExistByName())
			defer func() {
				_ = k8sClient.Delete(ctx, clusterRoleBinding)
			}()

			servicesExist := []string{"argocd-agent-principal", "argocd-agent-principal-metrics", "argocd-redis", "argocd-repo-server", "argocd-server"}

			for _, serviceName := range servicesExist {
				By("verifying Service '" + serviceName + "' exists and is a LoadBalancer or ClusterIP depending on which service")
				service := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: ns.Name,
					},
				}
				Eventually(service).Should(k8sFixture.ExistByName())

				if serviceName == "argocd-agent-principal" {
					Expect(string(service.Spec.Type)).To(Equal("LoadBalancer"))
				} else {
					Expect(string(service.Spec.Type)).To(Equal("ClusterIP"))
				}

			}

			configMapYAML := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-agent-params
  namespace: argocd-e2e-cluster-config
data:
  principal.jwt.allow-generate: 'true'
  principal.jwt.secret-name: argocd-agent-jwt
  principal.log.format: text
  principal.tls.server.root-ca-path: ''
  principal.resource-proxy.tls.ca-path: ''
  principal.namespace-create.enable: 'false'
  principal.resource-proxy.tls.key-path: ''
  principal.pprof-port: '0'
  principal.keep-alive-min-interval: 30s
  principal.listen.port: '8443'
  principal.auth: 'mtls:CN=([^,]+)'
  principal.resource-proxy.secret-name: argocd-agent-resource-proxy-tls
  principal.tls.client-cert.match-subject: 'false'
  principal.tls.client-cert.require: 'false'
  principal.tls.server.key-path: ''
  principal.tls.server.root-ca-secret-name: argocd-agent-ca
  principal.metrics.port: '8000'
  principal.listen.host: ''
  principal.namespace-create.pattern: ''
  principal.tls.server.allow-generate: 'false'
  principal.redis-compression-type: gzip
  principal.tls.secret-name: argocd-agent-principal-tls
  principal.resource-proxy.ca.path: ''
  principal.tls.server.cert-path: ''
  principal.resource-proxy.ca.secret-name: argocd-agent-ca
  principal.enable-resource-proxy: 'true'
  principal.jwt.key-path: ''
  principal.log.level: trace
  principal.namespace: ` + ns.Name + `
  principal.resource-proxy.tls.cert-path: ''
  principal.redis-server-address: 'argocd-redis:6379'
  principal.allowed-namespaces: '*'
  principal.enable-websocket: 'false'
  principal.namespace-create.labels: ''`

			expectedConfigMap := &corev1.ConfigMap{}
			// We unmarshal YAML into ArgoCD CR, so that we don't have to convert it into Go structs (it would be painful)
			Expect(yaml.UnmarshalStrict([]byte(configMapYAML), &expectedConfigMap)).To(Succeed())

			argocdAgentParams := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-agent-principal-params",
					Namespace: ns.Name,
				},
			}
			Eventually(argocdAgentParams).Should(k8sFixture.ExistByName())

			for k, v := range expectedConfigMap.Data {
				By("verifying '" + k + "'/'" + v + "' exists in ConfigMap")
				Expect(argocdAgentParams).Should(configmapFixture.HaveStringDataKeyValue(k, v))
			}

			deploymentsExist := []string{"argocd-redis", "argocd-repo-server", "argocd-server"}
			for _, deploymentName := range deploymentsExist {
				By("verifying Deployment '" + deploymentName + "' exists and is ready")
				depl := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deploymentName,
						Namespace: ns.Name,
					},
				}
				Eventually(depl).Should(k8sFixture.ExistByName())
				Eventually(depl).Should(deploymentFixture.HaveReadyReplicas(1))
			}

			By("verifying primary principal Deployment has expected values")
			principalDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-agent-principal",
					Namespace: ns.Name,
				},
			}
			Eventually(principalDeployment).Should(k8sFixture.ExistByName())
			Eventually(principalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/component", "principal"))
			Eventually(principalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/managed-by", "argocd"))
			Eventually(principalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/name", "argocd-agent-principal"))
			Eventually(principalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/part-of", "argocd-agent"))

			return principalDeployment
		}

		It("verifies that enabling Argo CD agent principal via ArgoCD CR causes the correct resources to be created, and that the image of the agent container is as expected based on ArgoCD CR", func() {

			By("creating cluster-scoped Argo CD instance in a new namespace")
			ns, cleanupFunc := fixture.CreateNamespaceWithCleanupFunc("argocd-agent-principal-1-051")
			defer cleanupFunc()

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(true),
					},
					ArgoCDAgent: &argov1beta1api.ArgoCDAgentSpec{
						Principal: &argov1beta1api.PrincipalSpec{
							Enabled: ptr.To(true),
							AllowedNamespaces: []string{
								"*",
							},
							JWTAllowGenerate: true,
							Auth:             "mtls:CN=([^,]+)",
							LogLevel:         "trace",
							Image:            "quay.io/user/argocd-agent:v1", // custom image
						},
					},
					SourceNamespaces: []string{
						"agent-managed",
						"agent-autonomous",
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying the Argo CD becomes available")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			principalDeployment := verifyExpectedResourcesExist(ns)

			By("verifying principal has the custom image we specified in ArgoCD CR")
			container := deploymentFixture.GetTemplateSpecContainerByName("argocd-agent-principal", *principalDeployment)
			Expect(container).ToNot(BeNil())
			Expect(container.Image).To(Equal("quay.io/user/argocd-agent:v1"))

			By("deleting ArgoCD instance")
			Expect(k8sClient.Delete(ctx, argoCD)).To(Succeed())
			Eventually(principalDeployment).ShouldNot(k8sFixture.ExistByName())

			By("creating an ArgoCD CR with principal argo cd agent enabled, but without a custom image for agent")
			argoCD = &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: ns.Name},
				Spec: argov1beta1api.ArgoCDSpec{
					Controller: argov1beta1api.ArgoCDApplicationControllerSpec{
						Enabled: ptr.To(true),
					},
					ArgoCDAgent: &argov1beta1api.ArgoCDAgentSpec{
						Principal: &argov1beta1api.PrincipalSpec{
							Enabled: ptr.To(true),
							AllowedNamespaces: []string{
								"*",
							},
							JWTAllowGenerate: true,
							Auth:             "mtls:CN=([^,]+)",
							LogLevel:         "trace",
						},
					},
					SourceNamespaces: []string{
						"agent-managed",
						"agent-autonomous",
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())
			Eventually(argoCD).Should(argocdFixture.BeAvailable())

			By("verifying Deployment is recreated, and that it has the default agent image")
			Eventually(principalDeployment).Should(k8sFixture.ExistByName())

			container = deploymentFixture.GetTemplateSpecContainerByName("argocd-agent-principal", *principalDeployment)
			Expect(container).ToNot(BeNil())
			Expect(container.Image).To(Equal("quay.io/argoprojlabs/argocd-agent:v0.3.2"))

		})

	})
})
