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
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	configmapFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/configmap"
	deploymentconfigFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/deploymentconfig"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-073_validate_rhsso_test", func() {

		var (
			k8sClient client.Client
			ctx       context.Context
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("verifies that the keycloak installed by operator has all the expected resources and is configured as expected, in Argo CD config and in Keycloak config", func() {

			if fixture.EnvLocalRun() {
				Skip("This test is known not to work when running gitops operator locally, in both kuttl and ginkgo forms")
				return
			}

			fixture.EnsureRunningOnOpenShift()

			ns, cleanupFunc := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer cleanupFunc()

			By("creating simple namespace-scoped Argo CD instance where keycloak is enabled with a fake root cert")
			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{Name: "example-argocd-keycloak", Namespace: ns.Name,
					Labels: map[string]string{"example": "keycloak"}},
				Spec: argov1beta1api.ArgoCDSpec{
					SSO: &argov1beta1api.ArgoCDSSOSpec{
						Provider: argov1beta1api.SSOProviderTypeKeycloak,
						Keycloak: &argov1beta1api.ArgoCDKeycloakSpec{
							RootCA: "---BEGIN---END---",
						},
					},
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("waiting for ArgoCD CR to be reconciled and the instance to be ready")
			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())

			By("verifying DeploymentConfig created by operator has expected values")

			dc := &openshiftappsv1.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "keycloak", Namespace: ns.Name},
			}
			Eventually(dc).Should(k8sFixture.ExistByName())

			Expect(dc.Spec.Selector).To(Equal(map[string]string{
				"deploymentConfig": "keycloak",
			}))

			Expect(dc.Spec.Strategy).To(Equal(openshiftappsv1.DeploymentStrategy{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("250m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
				RecreateParams:        &openshiftappsv1.RecreateDeploymentStrategyParams{TimeoutSeconds: ptr.To(int64(600))},
				Type:                  openshiftappsv1.DeploymentStrategyTypeRecreate,
				ActiveDeadlineSeconds: ptr.To(int64(21600)),
			}))

			Expect(dc.Spec.Template.ObjectMeta).Should(Equal(metav1.ObjectMeta{
				Labels: map[string]string{
					"application":      "keycloak",
					"deploymentConfig": "keycloak",
				},
				Name: "keycloak",
			}))

			Expect(dc.Spec.Template.Spec.Containers[0].Resources).Should(Equal(corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			}))

			Expect(dc.Spec.Template.Spec.Containers[0].VolumeMounts).Should(Equal([]corev1.VolumeMount{
				{Name: "sso-x509-https-volume", MountPath: "/etc/x509/https", ReadOnly: true},
				{Name: "service-ca", MountPath: "/var/run/configmaps/service-ca", ReadOnly: true},
				{Name: "sso-probe-netrc-volume", MountPath: "/mnt/rh-sso"},
			}))

			Expect(dc.Spec.Template.Spec.RestartPolicy).Should(Equal(corev1.RestartPolicyAlways))

			Expect(dc.Spec.Template.Spec.Volumes).Should(Equal([]corev1.Volume{
				{
					Name: "sso-x509-https-volume", VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							DefaultMode: ptr.To(int32(420)),
							SecretName:  "sso-x509-https-secret",
						},
					},
				},
				{
					Name: "service-ca", VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							DefaultMode: ptr.To(int32(420)),
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "keycloak-service-ca",
							},
						},
					},
				},
				{
					Name: "sso-probe-netrc-volume", VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							Medium: corev1.StorageMediumMemory,
						},
					},
				},
			}))

			Expect(dc.Spec.Triggers).To(Equal(openshiftappsv1.DeploymentTriggerPolicies{{Type: "ConfigChange"}}))
			Eventually(dc, "6m", "5s").Should(deploymentconfigFixture.HaveReadyReplicas(1))

			By("verifying Service, Route, and Secret exist and/or contain the expected values")

			keycloakService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "keycloak", Namespace: ns.Name}}
			Eventually(keycloakService).Should(k8sFixture.ExistByName())

			keycloakRoute := &routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "keycloak", Namespace: ns.Name}}
			Eventually(keycloakRoute).Should(k8sFixture.ExistByName())
			Expect(keycloakRoute.Spec.TLS.Termination).Should(Equal(routev1.TLSTerminationReencrypt))
			Expect(keycloakRoute.Spec.To).Should(Equal(routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   "keycloak",
				Weight: ptr.To(int32(100)),
			}))
			Expect(keycloakRoute.Spec.WildcardPolicy).Should(Equal(routev1.WildcardPolicyNone))

			keycloakSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "keycloak-secret", Namespace: ns.Name}}
			Eventually(keycloakSecret).Should(k8sFixture.ExistByName())
			Expect(keycloakSecret.Type).Should(Equal(corev1.SecretTypeOpaque))

			By("wait for argocd-cm ConfigMap to be configured for keycloak")
			argocdCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "argocd-cm", Namespace: ns.Name}}
			Expect(argocdCM).Should(k8sFixture.ExistByName())

			Eventually(argocdCM, "4m", "5s").Should(configmapFixture.HaveNonEmptyDataKey("oidc.config"))

			oidcConfigContents := argocdCM.Data["oidc.config"]

			findLineInOutput := func(key string, output string) string {
				// input format: "clientid: argocd\n\n"
				// output format, for 'clientid': "argocd"

				for _, line := range strings.Split(output, "\n") {
					if strings.Contains(line, key) {
						return strings.TrimSpace(line[strings.Index(line, ":")+1:])
					}
				}
				return ""
			}

			By("verifying that oidc.config value of argocd-cm ConfigMap contains expected configuration values")
			keycloakRouteHost := keycloakRoute.Spec.Host
			Expect(findLineInOutput("issuer", oidcConfigContents)).Should(Equal("https://"+keycloakRouteHost+"/auth/realms/argocd"), "certificate issuer should be /auth/realms/argocd from the route host")

			Expect(findLineInOutput("clientid", oidcConfigContents)).Should(Equal("argocd"))
			Expect(findLineInOutput("name", oidcConfigContents)).Should(Equal("Keycloak"))
			Expect(findLineInOutput("rootca", oidcConfigContents)).Should(Equal("'---BEGIN---END---'"))

			By("extracting the access token for keycloak, to allow us to issue API commands")

			ssoUsername, exists := keycloakSecret.Data["SSO_USERNAME"]
			Expect(exists).To(BeTrue())
			Expect(ssoUsername).ToNot(BeNil())

			ssoPassword, exists := keycloakSecret.Data["SSO_PASSWORD"]
			Expect(exists).To(BeTrue())
			Expect(ssoPassword).ToNot(BeNil())

			GinkgoWriter.Println("ssoUsername has length", len(ssoUsername), "ssoPass has length", len(ssoPassword), "keycloakRouteHost:", keycloakRouteHost)

			output, err := osFixture.ExecCommandWithOutputParam(false, "curl", "-d", "client_id=admin-cli", "-d", "username="+string(ssoUsername), "-d", "password="+string(ssoPassword), "-d", "grant_type=password", "https://"+keycloakRouteHost+"/auth/realms/master/protocol/openid-connect/token", "-k")
			Expect(err).ToNot(HaveOccurred())

			// Extract the JSON part of the output
			accessTokenIndex := strings.Index(output, "{\"access_token\"")
			Expect(accessTokenIndex).To(BeNumerically(">=", 0))
			output = output[accessTokenIndex:]

			var jsonData map[string]any
			Expect(json.Unmarshal([]byte(output), &jsonData)).ToNot(HaveOccurred())

			accessToken, success := jsonData["access_token"].(string)
			Expect(success).To(BeTrue())

			By("executing the CURL command to verify the realm and client creation")

			output, err = osFixture.ExecCommandWithOutputParam(false, "curl", "-H", "Content-Type: application/json", "-H", "Authorization: bearer "+accessToken, "https://"+keycloakRouteHost+"/auth/admin/realms/argocd/clients", "-k")
			Expect(err).ToNot(HaveOccurred())
			Expect(output).To(ContainSubstring("\"clientId\":\"argocd\""))

			By("executing the CURL command to verify OpenShift-v4 IdP creation")

			output, err = osFixture.ExecCommandWithOutputParam(false, "curl", "-H", "Content-Type: application/json", "-H", "Authorization: bearer "+accessToken, "https://"+keycloakRouteHost+"/auth/admin/realms/argocd/identity-provider/instances", "-k")
			Expect(err).ToNot(HaveOccurred())
			Expect(output).To(ContainSubstring("openshift-v4"))
			Expect(output).To(ContainSubstring("\"syncMode\":\"FORCE\""))

		})

	})
})
