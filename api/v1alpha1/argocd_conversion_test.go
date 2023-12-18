package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

type argoCDAlphaOpt func(*ArgoCD)

func makeTestArgoCDAlpha(opts ...argoCDAlphaOpt) *ArgoCD {
	a := &ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-argocd",
			Namespace: "default",
			Labels: map[string]string{
				"example": "conversion",
			},
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

type argoCDBetaOpt func(*argoproj.ArgoCD)

func makeTestArgoCDBeta(opts ...argoCDBetaOpt) *argoproj.ArgoCD {
	a := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-argocd",
			Namespace: "default",
			Labels: map[string]string{
				"example": "conversion",
			},
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

// in case of conflict, deprecated fields will have more priority during conversion to beta
func TestAlphaToBetaConversion(t *testing.T) {
	tests := []struct {
		name           string
		input          *ArgoCD
		expectedOutput *argoproj.ArgoCD
	}{
		// dex conversion
		{
			name: ".dex -> .sso.dex",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.Dex = &ArgoCDDexSpec{
					OpenShiftOAuth: true,
					Image:          "test",
					Version:        "latest",
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: "dex",
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
						Image:          "test",
						Version:        "latest",
					},
				}
			}),
		},
		{
			name: "Conflict: .dex & .sso.dex -> .sso.dex (values from v1alpha1.spec.dex)",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.Dex = &ArgoCDDexSpec{
					OpenShiftOAuth: true,
					Resources: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resourcev1.MustParse("2048Mi"),
							corev1.ResourceCPU:    resourcev1.MustParse("2000m"),
						},
					},
				}
				cr.Spec.SSO = &ArgoCDSSOSpec{
					Provider: SSOProviderTypeDex,
					Dex: &ArgoCDDexSpec{
						Config: "test-config",
						Image:  "test-image",
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
						Resources: &corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resourcev1.MustParse("2048Mi"),
								corev1.ResourceCPU:    resourcev1.MustParse("2000m"),
							},
						},
					},
				}
			}),
		},
		{
			name: "Missing dex provider: .dex & .sso.dex -> .spec.sso(values from v1alpha1.spec.dex with dex provider set)",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.Dex = &ArgoCDDexSpec{
					Config: "test-config",
				}
				cr.Spec.SSO = &ArgoCDSSOSpec{
					Dex: &ArgoCDDexSpec{
						OpenShiftOAuth: false,
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						Config: "test-config",
					},
				}
			}),
		},
		{
			name: "Missing dex provider without deprecated dex: .sso.dex -> .sso(values from v1alpha1.spec.sso)",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.SSO = &ArgoCDSSOSpec{
					Dex: &ArgoCDDexSpec{
						OpenShiftOAuth: false,
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: false,
					},
				}
			}),
		},

		// dex + keycloak - .spec.dex has more priority
		{
			name: "Conflict: .dex & .sso.keycloak provider -> .sso.dex + .sso.keycloak with dex provider",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.Dex = &ArgoCDDexSpec{
					OpenShiftOAuth: true,
				}
				cr.Spec.SSO = &ArgoCDSSOSpec{
					Provider: SSOProviderTypeKeycloak,
					Keycloak: &ArgoCDKeycloakSpec{
						Image: "keycloak",
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
					Keycloak: &argoproj.ArgoCDKeycloakSpec{
						Image: "keycloak",
					},
				}
			}),
		},

		// keycloak conversion
		{
			name: ".sso.VerifyTLS -> .sso.Keycloak.VerifyTLS",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				tls := new(bool)
				*tls = false
				cr.Spec.SSO = &ArgoCDSSOSpec{
					Provider: SSOProviderTypeKeycloak,
					Keycloak: &ArgoCDKeycloakSpec{
						RootCA: "__CA__",
					},
					VerifyTLS: tls,
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				tls := new(bool)
				*tls = false
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
					Keycloak: &argoproj.ArgoCDKeycloakSpec{
						RootCA:    "__CA__",
						VerifyTLS: tls,
					},
				}
			}),
		},
		{
			name: ".sso.Image without provider -> .sso.Keycloak.Image without provider",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.SSO = &ArgoCDSSOSpec{
					Image: "test-image",
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Keycloak: &argoproj.ArgoCDKeycloakSpec{
						Image: "test-image",
					},
				}
			}),
		},

		// other fields
		{
			name:           "ArgoCD Example - Empty",
			input:          makeTestArgoCDAlpha(func(cr *ArgoCD) {}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {}),
		},
		{
			name: "ArgoCD Example - Dex + RBAC",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.Dex = &ArgoCDDexSpec{
					OpenShiftOAuth: true,
				}

				defaultPolicy := "role:readonly"
				policy := "g, system:cluster-admins, role:admin"
				scope := "[groups]"
				cr.Spec.RBAC = ArgoCDRBACSpec{
					DefaultPolicy: &defaultPolicy,
					Policy:        &policy,
					Scopes:        &scope,
				}

				cr.Spec.Server = ArgoCDServerSpec{
					Route: ArgoCDRouteSpec{
						Enabled: true,
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}

				defaultPolicy := "role:readonly"
				policy := "g, system:cluster-admins, role:admin"
				scope := "[groups]"
				cr.Spec.RBAC = argoproj.ArgoCDRBACSpec{
					DefaultPolicy: &defaultPolicy,
					Policy:        &policy,
					Scopes:        &scope,
				}

				cr.Spec.Server = argoproj.ArgoCDServerSpec{
					Route: argoproj.ArgoCDRouteSpec{
						Enabled: true,
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - ResourceCustomizations",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.ResourceIgnoreDifferences = &ResourceIgnoreDifference{
					All: &IgnoreDifferenceCustomization{
						JsonPointers: []string{
							"/spec/replicas",
						},
						ManagedFieldsManagers: []string{
							"kube-controller-manager",
						},
					},
					ResourceIdentifiers: []ResourceIdentifiers{
						{
							Group: "admissionregistration.k8s.io",
							Kind:  "MutatingWebhookConfiguration",
							Customization: IgnoreDifferenceCustomization{
								JqPathExpressions: []string{
									"'.webhooks[]?.clientConfig.caBundle'",
								},
							},
						},
						{
							Group: "apps",
							Kind:  "Deployment",
							Customization: IgnoreDifferenceCustomization{
								ManagedFieldsManagers: []string{
									"kube-controller-manager",
								},
								JsonPointers: []string{
									"/spec/replicas",
								},
							},
						},
					},
				}
				cr.Spec.ResourceHealthChecks = []ResourceHealthCheck{
					{
						Group: "certmanager.k8s.io",
						Kind:  "Certificate",
					},
				}
				cr.Spec.ResourceActions = []ResourceAction{
					{
						Group: "apps",
						Kind:  "Deployment",
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.ResourceIgnoreDifferences = &argoproj.ResourceIgnoreDifference{
					All: &argoproj.IgnoreDifferenceCustomization{
						JsonPointers: []string{
							"/spec/replicas",
						},
						ManagedFieldsManagers: []string{
							"kube-controller-manager",
						},
					},
					ResourceIdentifiers: []argoproj.ResourceIdentifiers{
						{
							Group: "admissionregistration.k8s.io",
							Kind:  "MutatingWebhookConfiguration",
							Customization: argoproj.IgnoreDifferenceCustomization{
								JqPathExpressions: []string{
									"'.webhooks[]?.clientConfig.caBundle'",
								},
							},
						},
						{
							Group: "apps",
							Kind:  "Deployment",
							Customization: argoproj.IgnoreDifferenceCustomization{
								ManagedFieldsManagers: []string{
									"kube-controller-manager",
								},
								JsonPointers: []string{
									"/spec/replicas",
								},
							},
						},
					},
				}
				cr.Spec.ResourceHealthChecks = []argoproj.ResourceHealthCheck{
					{
						Group: "certmanager.k8s.io",
						Kind:  "Certificate",
					},
				}
				cr.Spec.ResourceActions = []argoproj.ResourceAction{
					{
						Group: "apps",
						Kind:  "Deployment",
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Image + ExtraConfig",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.Image = "test-image"
				cr.Spec.ExtraConfig = map[string]string{
					"ping": "pong",
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.Image = "test-image"
				cr.Spec.ExtraConfig = map[string]string{
					"ping": "pong",
				}
			}),
		},
		{
			name: "ArgoCD Example - Sever + Import",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.Server.Autoscale = ArgoCDServerAutoscaleSpec{
					Enabled: true,
				}
				cr.Spec.Import = &ArgoCDImportSpec{
					Name: "test-name",
				}
				cr.Spec.Server = ArgoCDServerSpec{
					Host: "test-host.argocd.org",
					GRPC: ArgoCDServerGRPCSpec{
						Ingress: ArgoCDIngressSpec{
							Enabled: false,
						},
					},
					Ingress: ArgoCDIngressSpec{
						Enabled: true,
						TLS: []v1.IngressTLS{
							{Hosts: []string{
								"test-tls",
							}},
						},
					},
					Insecure: true,
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.Server.Autoscale = argoproj.ArgoCDServerAutoscaleSpec{
					Enabled: true,
				}
				cr.Spec.Import = &argoproj.ArgoCDImportSpec{
					Name: "test-name",
				}
				cr.Spec.Server = argoproj.ArgoCDServerSpec{
					Host: "test-host.argocd.org",
					GRPC: argoproj.ArgoCDServerGRPCSpec{
						Ingress: argoproj.ArgoCDIngressSpec{
							Enabled: false,
						},
					},
					Ingress: argoproj.ArgoCDIngressSpec{
						Enabled: true,
						TLS: []v1.IngressTLS{
							{Hosts: []string{
								"test-tls",
							}},
						},
					},
					Insecure: true,
				}
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// Set v1beta1 object in Hub, converted values will be set in this object.
			var hub conversion.Hub = &argoproj.ArgoCD{}

			// Call ConvertTo function to convert v1alpha1 version to v1beta1
			test.input.ConvertTo(hub)

			// Fetch the converted object
			result := hub.(*argoproj.ArgoCD)

			// Compare converted object with expected.
			assert.Equal(t, test.expectedOutput, result)
		})
	}
}

// During beta to alpha conversion, converting sso fields back to deprecated fields is ignored as
// there is no data loss since the new fields in v1beta1 are also present in v1alpha1
func TestBetaToAlphaConversion(t *testing.T) {
	tests := []struct {
		name           string
		input          *argoproj.ArgoCD
		expectedOutput *ArgoCD
	}{
		{
			name:           "ArgoCD Example - Empty",
			input:          makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {}),
			expectedOutput: makeTestArgoCDAlpha(func(cr *ArgoCD) {}),
		},
		{
			name: "ArgoCD Example - Image + ExtraConfig",
			input: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.Image = "test-image"
				cr.Spec.ExtraConfig = map[string]string{
					"ping": "pong",
				}
			}),
			expectedOutput: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.Image = "test-image"
				cr.Spec.ExtraConfig = map[string]string{
					"ping": "pong",
				}
			}),
		},
		{
			name: "ArgoCD Example - Dex + RBAC",
			input: makeTestArgoCDBeta(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}

				defaultPolicy := "role:readonly"
				policy := "g, system:cluster-admins, role:admin"
				scope := "[groups]"
				cr.Spec.RBAC = argoproj.ArgoCDRBACSpec{
					DefaultPolicy: &defaultPolicy,
					Policy:        &policy,
					Scopes:        &scope,
				}

				cr.Spec.Server = argoproj.ArgoCDServerSpec{
					Route: argoproj.ArgoCDRouteSpec{
						Enabled: true,
					},
				}
			}),
			expectedOutput: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.SSO = &ArgoCDSSOSpec{
					Provider: SSOProviderTypeDex,
					Dex: &ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}

				defaultPolicy := "role:readonly"
				policy := "g, system:cluster-admins, role:admin"
				scope := "[groups]"
				cr.Spec.RBAC = ArgoCDRBACSpec{
					DefaultPolicy: &defaultPolicy,
					Policy:        &policy,
					Scopes:        &scope,
				}

				cr.Spec.Server = ArgoCDServerSpec{
					Route: ArgoCDRouteSpec{
						Enabled: true,
					},
				}
			}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// Add input v1beta1 object in Hub
			var hub conversion.Hub = test.input

			result := &ArgoCD{}
			// Call ConvertFrom function to convert v1beta1 version to v1alpha
			result.ConvertFrom(hub)

			// Compare converted object with expected.
			assert.Equal(t, test.expectedOutput, result)
		})
	}
}
