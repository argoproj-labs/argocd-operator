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
	routev1 "github.com/openshift/api/route/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	argocdFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/argocd"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	secretFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/secret"
	fixtureUtils "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("GitOps Operator Parallel E2E Tests", func() {

	Context("1-005_validate_route_tls", func() {

		var (
			ctx       context.Context
			k8sClient client.Client
		)

		BeforeEach(func() {
			fixture.EnsureParallelCleanSlate()
			k8sClient, _ = fixtureUtils.GetE2ETestKubeClient()
			ctx = context.Background()
		})

		It("ensures that certificates can be confirmed on server and webhook Routes", func() {

			fixture.EnsureRunningOnOpenShift()

			ns, nsCleanup := fixture.CreateRandomE2ETestNamespaceWithCleanupFunc()
			defer nsCleanup()

			By("creating Argo CD with server and appset controller webhook routes")

			argoCD := &argov1beta1api.ArgoCD{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example",
					Namespace: ns.Name,
				},
				Spec: argov1beta1api.ArgoCDSpec{
					Server: argov1beta1api.ArgoCDServerSpec{
						Route: argov1beta1api.ArgoCDRouteSpec{
							Enabled: true,
						},
					},
					ApplicationSet: &argov1beta1api.ArgoCDApplicationSet{
						Enabled: ptr.To(true),
						WebhookServer: argov1beta1api.WebhookServerSpec{
							Host: "example.com",
							Route: argov1beta1api.ArgoCDRouteSpec{
								Enabled: true,
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, argoCD)).To(Succeed())

			By("verifying expected resources exist")

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD, "2m", "5s").Should(
				And(argocdFixture.HaveApplicationSetControllerStatus("Running"), argocdFixture.HaveServerStatus("Running")))

			serverRoute := &routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "example-server", Namespace: ns.Name}}
			Eventually(serverRoute).Should(k8sFixture.ExistByName())
			Expect(serverRoute.Spec.To.Kind).To(Equal("Service"))
			Expect(serverRoute.Spec.To.Name).To(Equal("example-server"))
			Expect(*serverRoute.Spec.To.Weight).To(Equal(int32(100)))
			Expect(serverRoute.Spec.TLS.InsecureEdgeTerminationPolicy).To(Equal(routev1.InsecureEdgeTerminationPolicyRedirect))
			Expect(serverRoute.Spec.TLS.Termination).To(Equal(routev1.TLSTerminationReencrypt))

			webhookRoute := &routev1.Route{ObjectMeta: metav1.ObjectMeta{Name: "example-appset-webhook", Namespace: ns.Name}}
			Eventually(webhookRoute).Should(k8sFixture.ExistByName())
			Expect(webhookRoute.Spec.To.Kind).To(Equal("Service"))
			Expect(webhookRoute.Spec.To.Name).To(Equal("example-applicationset-controller"))
			Expect(*webhookRoute.Spec.To.Weight).To(Equal(int32(100)))
			Expect(webhookRoute.Spec.TLS.InsecureEdgeTerminationPolicy).To(Equal(routev1.InsecureEdgeTerminationPolicyRedirect))
			Expect(webhookRoute.Spec.TLS.Termination).To(Equal(routev1.TLSTerminationEdge))

			By("setting an embedded certificate and private key in ArgoCD CR")

			cert := `-----BEGIN CERTIFICATE-----
MIIEbTCCAtWgAwIBAgIUA80/UfgNcx8tYz/XXlo6X8DJzXQwDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAgFw0yNDA5MjUwNDM4MjdaGA8yMTIz
MDQyMDA0MzgyN1owRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUx
ITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDCCAaIwDQYJKoZIhvcN
AQEBBQADggGPADCCAYoCggGBAJUuv+nO7S02+BHo5zkVg/IwUNSqQhsgKe3Djzsm
ISctrzNgrtUPqxYU0XDPXIS/v4wrtXrbXjlEaVgpTToqt/DRITH/I9FZzFQRQWKb
Gx0g3aH/LFJHHix4KCMPzEcykXba3zJqZei4NeJ7ym/Z5g/gJjGOE2SDVJN7YA9p
WKEgf/+TB6uPkEcgNc+8rFKbwQ63IhqOnHZq0mFaT/DWQUWYqLNZOHIiXjIELjGe
RjzmxlTQd9hWrC+FP1fOz9Ahpnw8oJ+wEpMUSpsAd3FFYUDZW/bj3jwWLT3WtmTb
d5ehpeE/zM5twy4rZXzT43+fsO/ns2YDxsSiujrtwm/Ar5k86S2XTkWro6f/t/Ml
dcIGzUZm2lSRacX1brIhNryHU2ZyVsEKJbS4/7N/wHTqhctSZlJRXkfjPiIC2KHV
YngPAtJ+fSmdULd7rIWcaxsrpnyozVpzYm5U8XRGm/pj2FFHVKPdSBoo2GrkVMyh
oU3+YiFno57wNbrm9ROzMIHhhwIDAQABo1MwUTAdBgNVHQ4EFgQUTbU3O3JsKBC6
jCLjxTX4zWEAgc8wHwYDVR0jBBgwFoAUTbU3O3JsKBC6jCLjxTX4zWEAgc8wDwYD
VR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAYEAMthyYhEUf5GdrKSMBuWR
+QlsBau/6N2nSxRxM2g4oexQOGUny1r76KrW6o/2V/PYyz/3WgOgSB/4sZxNoeu8
rsjY9sp/bCWJ6jEmhm2kkVeb3Arix0iNt7BviOCjoVchc31R20JLP0a6WK+KtiV2
C8qbuOQEkVWY/NVy+buHKqJjNZXyj8ADX0It8rAmaEGMEGkEFtYTnjEYHdkPWfYx
6P9C12PrZySu9+L3eGmylKeDU7dWvBAONbHfHL8W/8pxG1CwObfkTEpzVTlR0SfI
W1dZ9YXb7S5F/0j6GLeUSgvnQZxH4rbc699wC9Y/kt5EozT1xvmKgZ6G6vaU2Mhb
jZnrbB4swXCVf98HDAy8PWrn7BWky9G8SbM5kS6Mj9pQwZnnfF6VLg+uWBBjMh7g
0Ntf+Lv/IC5v+jC7TDKRPCAUGYzBRLMbT0WvK0BVXhp6swCi4qtME/BTsqXA6zzk
5PfEh1b+yuqxbF3bU8rII1LIsXxr96lssl+H0HxPpQKv
-----END CERTIFICATE-----`

			key := `-----BEGIN PRIVATE KEY-----
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
HkXkOt1cAbJWZruf4j13X4s=
-----END PRIVATE KEY-----`

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				argoCD.Spec.Server.Route.TLS = &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
					Certificate: cert,
					Key:         key,
				}
				argoCD.Spec.ApplicationSet.WebhookServer.Route.TLS = &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
					Certificate: cert,
					Key:         key,
				}
			})

			By("verifying Routes pick up certificate and key data")

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD, "3m", "5s").Should(
				And(argocdFixture.HaveApplicationSetControllerStatus("Running"), argocdFixture.HaveServerStatus("Running")))

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverRoute), serverRoute); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				if serverRoute.Spec.TLS.Certificate != cert {
					return false
				}

				if serverRoute.Spec.TLS.Key != key {
					return false
				}

				return true

			}).Should(BeTrue())

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(webhookRoute), webhookRoute); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				if webhookRoute.Spec.TLS.Certificate != cert {
					return false
				}

				if webhookRoute.Spec.TLS.Key != key {
					return false
				}

				return true

			}).Should(BeTrue())

			By("updating ArgoCD to use an external Secret containing certificate and key")

			tlsCRTForSecret := `-----BEGIN CERTIFICATE-----
MIIFrjCCA5agAwIBAgIUbM9O0W6IdumLQodDCDqyckYDr2IwDQYJKoZIhvcNAQEL
BQAwTTELMAkGA1UEBhMCVVMxDTALBgNVBAgMBFRlc3QxDTALBgNVBAoMBFRlc3Qx
DTALBgNVBAsMBFRlc3QxETAPBgNVBAMMCHRlc3QuY29tMCAXDTIzMTEyNjIyMTg0
N1oYDzIxMjMxMTI3MjIxODQ3WjBNMQswCQYDVQQGEwJVUzENMAsGA1UECAwEVGVz
dDENMAsGA1UECgwEVGVzdDENMAsGA1UECwwEVGVzdDERMA8GA1UEAwwIdGVzdC5j
b20wggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQDbgAmnUjFux9u2Xzhi
mno5zjA/YsoXr3eFtK9XtByQMLLyT0hbXoa9gpTeafOs3IkCotPdN+omxm2tN9UA
ebAq+EamWyIF28EA3UbCWWULghveezrmAKSMcqQqby3knbcbGng+ZZjRdC3xc0uz
/sd4FqaLt0UHBDMlpxRskj/S3CDetfyIrKYQcZ5NQjx75aRN8At5OPC1NiWTmlsv
ppa4LLV0HR6AJzq+C6RAmJTcHQOFAq33wZEHHIpoQoGWHHPpT0ut54KIiVTRJ2o4
MEV4KlBBgL3ux4+v7R0RfVmzgaMEDG1fC9tX8pIofv7wP7WX/5XHTjyAiv8gbpUW
nLiU8FoTDZWxZN+MiCkUvZl8KqotbcUPjhnRdnq4anFwywY1lKILnCIayqzI7mPW
12h39fNwprFz9YFYbLLoQHekir2nLw8ZH83nNyD82YQ3EFm7UnOld6zw/8aURRuQ
C0oOEHyAXsvIyaWAb6lWvplDdCUGQWWr7MVp5YPPhWdtAv7B4QLDUNHGQMU/1Qrq
VBH22lcU7XrCh6GXrRVm+gF7kAuJzkuae0txvk9mHc+8Y0C4/i9C3xU2qHjWcElw
etcHbqOZjDtC8+n8mDD4hDYEMGV54VhXCKwoFLneT2no27S3SVPvNbMfyyNuUa2i
5azKnIf439Cmfww7ImxIpOR5nQIDAQABo4GDMIGAMB0GA1UdDgQWBBQfe95iWKlT
K6BGFov9JFXQTQN0ZjAfBgNVHSMEGDAWgBQfe95iWKlTK6BGFov9JFXQTQN0ZjAP
BgNVHRMBAf8EBTADAQH/MC0GA1UdEQQmMCSCB3dlYmhvb2uCDndlYmhvb2stc2Vy
dmVygglsb2NhbGhvc3QwDQYJKoZIhvcNAQELBQADggIBAH7Vv+Iar1UbF41c9I88
oIu8iWfLVnAfe/64tULy77x4nfEQiukBJDoZ9m19KEVdPqsFzT6lFB7Fu1oc9A28
5b1+PEynHcopNK41zF4n4FnIy9h8zJfaPYYCPPMT0v9LzuT5zyF5sXCz0o4KwQJ6
zrggZme8udl9sWyDxZyFoFPLWtnQFY7vJ9LSM2Gt+XUIuYNwDkvGFs6RfBYJGarX
qq7YHYj0H2x/us3KQCXGX5GzSmM9ewHvaScRpFcCdVwszKwWF0vMvdnh+3P72/Yy
dQvZXyfNiwqaIdznJn/AjzR9K4dHfbY7wMm83WHwWyjzV6CybHbtWpoUIlZtW3TT
gz6MP2z+BhOdMiQA33aO38J2TX/CMkEvkagEiZdS9t3xtpF2LOb5bRIdlENtZU0i
LnhgWEpJmswxBtuJ0d/zcyUlvK7FYoJZB7pT3YX/321HXZVCKyw+xrinwQoI3RnX
7u0TZ3MqtSKEwCyDWYRJDbs6XUX1G0q7jXBf1+3cd+lBdOZ4Kl5B4YSU9hcFxAuO
4a1eFXBdmT8PnwoTizFvag3IgBXkf8PqcKNvSMU6UKcD5LYTwRGK3JVl1L79gkrb
LmWEfOXFHgSlMIZkEs41TiopXy8p/LSera8NR86Q3mTZ7rRdEveOb6ZLJksRqaqr
UVwpFuaKz5vTCD36Gmmy/u8y
-----END CERTIFICATE-----`

			tlsKeyForSecret := `-----BEGIN PRIVATE KEY-----
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
Xq+NinfrqOLJkIZ/u/PJu4KqN3M=
-----END PRIVATE KEY-----`

			tlsDataSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tls-data-secret",
					Namespace: ns.Name,
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": ([]byte)(tlsCRTForSecret),
					"tls.key": ([]byte)(tlsKeyForSecret),
				},
			}
			Expect(k8sClient.Create(ctx, tlsDataSecret)).To(Succeed())

			argocdFixture.Update(argoCD, func(ac *argov1beta1api.ArgoCD) {
				argoCD.Spec.Server.Route.TLS = &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
					ExternalCertificate: &routev1.LocalObjectReference{
						Name: "tls-data-secret",
					},
					Certificate: "",
					Key:         "",
				}
				argoCD.Spec.ApplicationSet.WebhookServer.Route.TLS = &routev1.TLSConfig{
					Termination: routev1.TLSTerminationReencrypt,
					ExternalCertificate: &routev1.LocalObjectReference{
						Name: "tls-data-secret",
					},
					Certificate: "",
					Key:         "",
				}
			})

			By("validating that Routes pickup certificate and key from external Secret")

			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverRoute), serverRoute); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				if serverRoute.Spec.TLS.Certificate != tlsCRTForSecret {
					return false
				}

				if serverRoute.Spec.TLS.Key != tlsKeyForSecret {
					return false
				}

				return true

			}).Should(BeTrue())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(webhookRoute), webhookRoute); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				if webhookRoute.Spec.TLS.Certificate != tlsCRTForSecret {
					return false
				}

				if webhookRoute.Spec.TLS.Key != tlsKeyForSecret {
					return false
				}

				return true

			}).Should(BeTrue())

			By("updating the cert/key data of the external secret")

			newTLSCRTForSecret := `-----BEGIN CERTIFICATE-----
MIIEbTCCAtWgAwIBAgIUA80/UfgNcx8tYz/XXlo6X8DJzXQwDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAgFw0yNDA5MjUwNDM4MjdaGA8yMTIz
MDQyMDA0MzgyN1owRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUx
ITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDCCAaIwDQYJKoZIhvcN
AQEBBQADggGPADCCAYoCggGBAJUuv+nO7S02+BHo5zkVg/IwUNSqQhsgKe3Djzsm
ISctrzNgrtUPqxYU0XDPXIS/v4wrtXrbXjlEaVgpTToqt/DRITH/I9FZzFQRQWKb
Gx0g3aH/LFJHHix4KCMPzEcykXba3zJqZei4NeJ7ym/Z5g/gJjGOE2SDVJN7YA9p
WKEgf/+TB6uPkEcgNc+8rFKbwQ63IhqOnHZq0mFaT/DWQUWYqLNZOHIiXjIELjGe
RjzmxlTQd9hWrC+FP1fOz9Ahpnw8oJ+wEpMUSpsAd3FFYUDZW/bj3jwWLT3WtmTb
d5ehpeE/zM5twy4rZXzT43+fsO/ns2YDxsSiujrtwm/Ar5k86S2XTkWro6f/t/Ml
dcIGzUZm2lSRacX1brIhNryHU2ZyVsEKJbS4/7N/wHTqhctSZlJRXkfjPiIC2KHV
YngPAtJ+fSmdULd7rIWcaxsrpnyozVpzYm5U8XRGm/pj2FFHVKPdSBoo2GrkVMyh
oU3+YiFno57wNbrm9ROzMIHhhwIDAQABo1MwUTAdBgNVHQ4EFgQUTbU3O3JsKBC6
jCLjxTX4zWEAgc8wHwYDVR0jBBgwFoAUTbU3O3JsKBC6jCLjxTX4zWEAgc8wDwYD
VR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAYEAMthyYhEUf5GdrKSMBuWR
+QlsBau/6N2nSxRxM2g4oexQOGUny1r76KrW6o/2V/PYyz/3WgOgSB/4sZxNoeu8
rsjY9sp/bCWJ6jEmhm2kkVeb3Arix0iNt7BviOCjoVchc31R20JLP0a6WK+KtiV2
C8qbuOQEkVWY/NVy+buHKqJjNZXyj8ADX0It8rAmaEGMEGkEFtYTnjEYHdkPWfYx
6P9C12PrZySu9+L3eGmylKeDU7dWvBAONbHfHL8W/8pxG1CwObfkTEpzVTlR0SfI
W1dZ9YXb7S5F/0j6GLeUSgvnQZxH4rbc699wC9Y/kt5EozT1xvmKgZ6G6vaU2Mhb
jZnrbB4swXCVf98HDAy8PWrn7BWky9G8SbM5kS6Mj9pQwZnnfF6VLg+uWBBjMh7g
0Ntf+Lv/IC5v+jC7TDKRPCAUGYzBRLMbT0WvK0BVXhp6swCi4qtME/BTsqXA6zzk
5PfEh1b+yuqxbF3bU8rII1LIsXxr96lssl+H0HxPpQKv
-----END CERTIFICATE-----`

			newTLSKeyForSecret := `-----BEGIN PRIVATE KEY-----
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
HkXkOt1cAbJWZruf4j13X4s=
-----END PRIVATE KEY-----`

			secretFixture.Update(tlsDataSecret, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"tls.crt": ([]byte)(newTLSCRTForSecret),
					"tls.key": ([]byte)(newTLSKeyForSecret),
				}
			})

			By("verifying the Routes pick up the updated certificate/key data from the Secret")
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(serverRoute), serverRoute); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				if serverRoute.Spec.TLS.Certificate != newTLSCRTForSecret {
					return false
				}

				if serverRoute.Spec.TLS.Key != newTLSKeyForSecret {
					return false
				}

				return true

			}).Should(BeTrue())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(webhookRoute), webhookRoute); err != nil {
					GinkgoWriter.Println(err)
					return false
				}

				if webhookRoute.Spec.TLS.Certificate != newTLSCRTForSecret {
					return false
				}

				if webhookRoute.Spec.TLS.Key != newTLSKeyForSecret {
					return false
				}

				return true

			}).Should(BeTrue())

			Eventually(argoCD, "5m", "5s").Should(argocdFixture.BeAvailable())
			Eventually(argoCD).Should(
				And(argocdFixture.HaveApplicationSetControllerStatus("Running"), argocdFixture.HaveServerStatus("Running")))

		})
	})
})
