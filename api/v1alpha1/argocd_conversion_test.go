package v1alpha1

import (
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	v1beta1 "github.com/argoproj-labs/argocd-operator/api/v1beta1"
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

type argoCDBetaOpt func(*v1beta1.ArgoCD)

func makeTestArgoCDBeta(opts ...argoCDBetaOpt) *v1beta1.ArgoCD {
	a := &v1beta1.ArgoCD{
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
		expectedOutput *v1beta1.ArgoCD
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
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.SSO = &v1beta1.ArgoCDSSOSpec{
					Provider: "dex",
					Dex: &v1beta1.ArgoCDDexSpec{
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
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.SSO = &v1beta1.ArgoCDSSOSpec{
					Provider: v1beta1.SSOProviderTypeDex,
					Dex: &v1beta1.ArgoCDDexSpec{
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
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.SSO = &v1beta1.ArgoCDSSOSpec{
					Provider: v1beta1.SSOProviderTypeDex,
					Dex: &v1beta1.ArgoCDDexSpec{
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
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.SSO = &v1beta1.ArgoCDSSOSpec{
					Dex: &v1beta1.ArgoCDDexSpec{
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
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.SSO = &v1beta1.ArgoCDSSOSpec{
					Provider: v1beta1.SSOProviderTypeDex,
					Dex: &v1beta1.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
					Keycloak: &v1beta1.ArgoCDKeycloakSpec{
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
						Host:   "test-keycloak-host",
					},
					VerifyTLS: tls,
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				tls := new(bool)
				*tls = false
				cr.Spec.SSO = &v1beta1.ArgoCDSSOSpec{
					Provider: v1beta1.SSOProviderTypeKeycloak,
					Keycloak: &v1beta1.ArgoCDKeycloakSpec{
						RootCA:    "__CA__",
						VerifyTLS: tls,
						Host:      "test-keycloak-host",
					},
				}
			}),
		},
		{
			name: ".sso.Image without provider -> .sso.Keycloak.Image without provider",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.SSO = &ArgoCDSSOSpec{
					Image: "test-image",
					Keycloak: &ArgoCDKeycloakSpec{
						Host: "test-host",
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.SSO = &v1beta1.ArgoCDSSOSpec{
					Keycloak: &v1beta1.ArgoCDKeycloakSpec{
						Image: "test-image",
						Host:  "test-host",
					},
				}
			}),
		},

		// other fields
		{
			name:           "ArgoCD Example - Empty",
			input:          makeTestArgoCDAlpha(func(cr *ArgoCD) {}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {}),
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
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.SSO = &v1beta1.ArgoCDSSOSpec{
					Provider: v1beta1.SSOProviderTypeDex,
					Dex: &v1beta1.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}

				defaultPolicy := "role:readonly"
				policy := "g, system:cluster-admins, role:admin"
				scope := "[groups]"
				cr.Spec.RBAC = v1beta1.ArgoCDRBACSpec{
					DefaultPolicy: &defaultPolicy,
					Policy:        &policy,
					Scopes:        &scope,
				}

				cr.Spec.Server = v1beta1.ArgoCDServerSpec{
					Route: v1beta1.ArgoCDRouteSpec{
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
				cr.Spec.NamespaceManagement = []ManagedNamespaces{
					{
						Name:           "test-nm-namespace",
						AllowManagedBy: true,
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.ResourceIgnoreDifferences = &v1beta1.ResourceIgnoreDifference{
					All: &v1beta1.IgnoreDifferenceCustomization{
						JsonPointers: []string{
							"/spec/replicas",
						},
						ManagedFieldsManagers: []string{
							"kube-controller-manager",
						},
					},
					ResourceIdentifiers: []v1beta1.ResourceIdentifiers{
						{
							Group: "admissionregistration.k8s.io",
							Kind:  "MutatingWebhookConfiguration",
							Customization: v1beta1.IgnoreDifferenceCustomization{
								JqPathExpressions: []string{
									"'.webhooks[]?.clientConfig.caBundle'",
								},
							},
						},
						{
							Group: "apps",
							Kind:  "Deployment",
							Customization: v1beta1.IgnoreDifferenceCustomization{
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
				cr.Spec.ResourceHealthChecks = []v1beta1.ResourceHealthCheck{
					{
						Group: "certmanager.k8s.io",
						Kind:  "Certificate",
					},
				}
				cr.Spec.ResourceActions = []v1beta1.ResourceAction{
					{
						Group: "apps",
						Kind:  "Deployment",
					},
				}
				cr.Spec.NamespaceManagement = []v1beta1.ManagedNamespaces{
					{
						Name:           "test-nm-namespace",
						AllowManagedBy: true,
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
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
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
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.Server.Autoscale = v1beta1.ArgoCDServerAutoscaleSpec{
					Enabled: true,
				}
				cr.Spec.Import = &v1beta1.ArgoCDImportSpec{
					Name: "test-name",
				}
				cr.Spec.Server = v1beta1.ArgoCDServerSpec{
					Host: "test-host.argocd.org",
					GRPC: v1beta1.ArgoCDServerGRPCSpec{
						Ingress: v1beta1.ArgoCDIngressSpec{
							Enabled: false,
						},
					},
					Ingress: v1beta1.ArgoCDIngressSpec{
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
		{
			name: "ArgoCD Example - Route TLS",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				cr.Spec.Server.Route = ArgoCDRouteSpec{
					Enabled: true,
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationEdge,
					},
				}
				cr.Spec.Prometheus.Route = ArgoCDRouteSpec{
					Enabled: true,
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationEdge,
					},
				}
				cr.Spec.Grafana.Route = ArgoCDRouteSpec{
					Enabled: true,
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationEdge,
					},
				}
				cr.Spec.ApplicationSet = &ArgoCDApplicationSet{
					WebhookServer: WebhookServerSpec{
						Route: ArgoCDRouteSpec{
							Enabled: true,
							TLS: &routev1.TLSConfig{
								Termination: routev1.TLSTerminationEdge,
							},
						},
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.Server.Route = v1beta1.ArgoCDRouteSpec{
					Enabled: true,
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationEdge,
					},
				}
				cr.Spec.Prometheus.Route = v1beta1.ArgoCDRouteSpec{
					Enabled: true,
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationEdge,
					},
				}
				//lint:ignore SA1019 known to be deprecated
				cr.Spec.Grafana.Route = v1beta1.ArgoCDRouteSpec{ //nolint:staticcheck // SA1019: We must test deprecated fields.
					Enabled: true,
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationEdge,
					},
				}
				cr.Spec.ApplicationSet = &v1beta1.ArgoCDApplicationSet{
					WebhookServer: v1beta1.WebhookServerSpec{
						Route: v1beta1.ArgoCDRouteSpec{
							Enabled: true,
							TLS: &routev1.TLSConfig{
								Termination: routev1.TLSTerminationEdge,
							},
						},
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Principal Basic",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := true
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Principal: &PrincipalSpec{
						Enabled: &enabled,
						Auth:    "mtls:CN=([^,]+)",
						Server: &PrincipalServerSpec{
							Service: ArgoCDAgentPrincipalServiceSpec{
								Type: corev1.ServiceTypeClusterIP,
							},
							Route: ArgoCDAgentPrincipalRouteSpec{
								Enabled: ptr.To(true),
							},
						},
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := true
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Principal: &v1beta1.PrincipalSpec{
						Enabled: &enabled,
						Auth:    "mtls:CN=([^,]+)",
						Server: &v1beta1.PrincipalServerSpec{
							Service: v1beta1.ArgoCDAgentPrincipalServiceSpec{
								Type: corev1.ServiceTypeClusterIP,
							},
							Route: v1beta1.ArgoCDAgentPrincipalRouteSpec{
								Enabled: ptr.To(true),
							},
						},
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Principal Full Configuration",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := true
				enableWebSocket := true
				enableNamespaceCreate := true
				insecureGenerate := true
				allowGenerate := true
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Principal: &PrincipalSpec{
						Enabled:   &enabled,
						Auth:      "mtls:CN=([^,]+)",
						LogLevel:  "info",
						LogFormat: "text",
						Image:     "quay.io/user/argocd-agent:v1",
						Server: &PrincipalServerSpec{
							EnableWebSocket:      &enableWebSocket,
							KeepAliveMinInterval: "30s",
							Service: ArgoCDAgentPrincipalServiceSpec{
								Type: corev1.ServiceTypeLoadBalancer,
							},
							Route: ArgoCDAgentPrincipalRouteSpec{
								Enabled: ptr.To(false),
							},
						},
						Redis: &PrincipalRedisSpec{
							ServerAddress:   "redis:6379",
							CompressionType: "gzip",
						},
						Namespace: &PrincipalNamespaceSpec{
							AllowedNamespaces:      []string{"*"},
							EnableNamespaceCreate:  &enableNamespaceCreate,
							NamespaceCreatePattern: "agent-.*",
							NamespaceCreateLabels:  []string{"environment=agent"},
						},
						TLS: &PrincipalTLSSpec{
							SecretName:       "tls-secret",
							RootCASecretName: "ca-secret",
							InsecureGenerate: &allowGenerate,
						},
						ResourceProxy: &PrincipalResourceProxySpec{
							SecretName:   "proxy-secret",
							CASecretName: "proxy-ca-secret",
						},
						JWT: &PrincipalJWTSpec{
							InsecureGenerate: &insecureGenerate,
							SecretName:       "jwt-secret",
						},
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := true
				enableWebSocket := true
				enableNamespaceCreate := true
				allowGenerate := true
				insecureGenerate := true
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Principal: &v1beta1.PrincipalSpec{
						Enabled:   &enabled,
						Auth:      "mtls:CN=([^,]+)",
						LogLevel:  "info",
						LogFormat: "text",
						Image:     "quay.io/user/argocd-agent:v1",
						Server: &v1beta1.PrincipalServerSpec{
							EnableWebSocket:      &enableWebSocket,
							KeepAliveMinInterval: "30s",
							Service: v1beta1.ArgoCDAgentPrincipalServiceSpec{
								Type: corev1.ServiceTypeLoadBalancer,
							},
							Route: v1beta1.ArgoCDAgentPrincipalRouteSpec{
								Enabled: ptr.To(false),
							},
						},
						Redis: &v1beta1.PrincipalRedisSpec{
							ServerAddress:   "redis:6379",
							CompressionType: "gzip",
						},
						Namespace: &v1beta1.PrincipalNamespaceSpec{
							AllowedNamespaces:      []string{"*"},
							EnableNamespaceCreate:  &enableNamespaceCreate,
							NamespaceCreatePattern: "agent-.*",
							NamespaceCreateLabels:  []string{"environment=agent"},
						},
						TLS: &v1beta1.PrincipalTLSSpec{
							SecretName:       "tls-secret",
							RootCASecretName: "ca-secret",
							InsecureGenerate: &allowGenerate,
						},
						ResourceProxy: &v1beta1.PrincipalResourceProxySpec{
							SecretName:   "proxy-secret",
							CASecretName: "proxy-ca-secret",
						},
						JWT: &v1beta1.PrincipalJWTSpec{
							InsecureGenerate: &insecureGenerate,
							SecretName:       "jwt-secret",
						},
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Principal Disabled",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := false
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Principal: &PrincipalSpec{
						Enabled: &enabled,
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := false
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Principal: &v1beta1.PrincipalSpec{
						Enabled: &enabled,
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Agent Basic",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := true
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Agent: &AgentSpec{
						Enabled: &enabled,
						Creds:   "mtls:any",
						Client: &AgentClientSpec{
							PrincipalServerAddress: "argocd-agent-principal.example.com",
							PrincipalServerPort:    "443",
							Mode:                   "managed",
						},
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := true
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Agent: &v1beta1.AgentSpec{
						Enabled: &enabled,
						Creds:   "mtls:any",
						Client: &v1beta1.AgentClientSpec{
							PrincipalServerAddress: "argocd-agent-principal.example.com",
							PrincipalServerPort:    "443",
							Mode:                   "managed",
						},
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Agent Full Configuration",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := true
				enableWebSocket := true
				enableCompression := true
				insecure := true
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Agent: &AgentSpec{
						Enabled:   &enabled,
						Creds:     "mtls:any",
						LogLevel:  "info",
						LogFormat: "text",
						Image:     "quay.io/user/argocd-agent:v1",
						Client: &AgentClientSpec{
							PrincipalServerAddress: "argocd-agent-principal.example.com",
							PrincipalServerPort:    "443",
							Mode:                   "managed",
							EnableWebSocket:        &enableWebSocket,
							EnableCompression:      &enableCompression,
							KeepAliveInterval:      "30s",
						},
						Redis: &AgentRedisSpec{
							ServerAddress: "redis:6379",
						},
						TLS: &AgentTLSSpec{
							SecretName:       "agent-tls-secret",
							RootCASecretName: "agent-ca-secret",
							Insecure:         &insecure,
						},
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := true
				enableWebSocket := true
				enableCompression := true
				insecure := true
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Agent: &v1beta1.AgentSpec{
						Enabled:   &enabled,
						Creds:     "mtls:any",
						LogLevel:  "info",
						LogFormat: "text",
						Image:     "quay.io/user/argocd-agent:v1",
						Client: &v1beta1.AgentClientSpec{
							PrincipalServerAddress: "argocd-agent-principal.example.com",
							PrincipalServerPort:    "443",
							Mode:                   "managed",
							EnableWebSocket:        &enableWebSocket,
							EnableCompression:      &enableCompression,
							KeepAliveInterval:      "30s",
						},
						Redis: &v1beta1.AgentRedisSpec{
							ServerAddress: "redis:6379",
						},
						TLS: &v1beta1.AgentTLSSpec{
							SecretName:       "agent-tls-secret",
							RootCASecretName: "agent-ca-secret",
							Insecure:         &insecure,
						},
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Agent Disabled",
			input: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := false
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Agent: &AgentSpec{
						Enabled: &enabled,
					},
				}
			}),
			expectedOutput: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := false
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Agent: &v1beta1.AgentSpec{
						Enabled: &enabled,
					},
				}
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// Set v1beta1 object in Hub, converted values will be set in this object.
			var hub conversion.Hub = &v1beta1.ArgoCD{}

			// Call ConvertTo function to convert v1alpha1 version to v1beta1
			err := test.input.ConvertTo(hub)
			assert.NoError(t, err)

			// Fetch the converted object
			result := hub.(*v1beta1.ArgoCD)

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
		input          *v1beta1.ArgoCD
		expectedOutput *ArgoCD
	}{
		{
			name:           "ArgoCD Example - Empty",
			input:          makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {}),
			expectedOutput: makeTestArgoCDAlpha(func(cr *ArgoCD) {}),
		},
		{
			name: "ArgoCD Example - Image + ExtraConfig",
			input: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
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
			input: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				cr.Spec.SSO = &v1beta1.ArgoCDSSOSpec{
					Provider: v1beta1.SSOProviderTypeDex,
					Dex: &v1beta1.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}

				defaultPolicy := "role:readonly"
				policy := "g, system:cluster-admins, role:admin"
				scope := "[groups]"
				cr.Spec.RBAC = v1beta1.ArgoCDRBACSpec{
					DefaultPolicy: &defaultPolicy,
					Policy:        &policy,
					Scopes:        &scope,
				}

				cr.Spec.Server = v1beta1.ArgoCDServerSpec{
					Route: v1beta1.ArgoCDRouteSpec{
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
		{
			name: "ArgoCD Example - Agent Principal Basic",
			input: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := true
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Principal: &v1beta1.PrincipalSpec{
						Enabled: &enabled,
						Auth:    "mtls:CN=([^,]+)",
						Server: &v1beta1.PrincipalServerSpec{
							Service: v1beta1.ArgoCDAgentPrincipalServiceSpec{
								Type: corev1.ServiceTypeNodePort,
							},
							Route: v1beta1.ArgoCDAgentPrincipalRouteSpec{
								Enabled: ptr.To(true),
							},
						},
					},
				}
			}),
			expectedOutput: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := true
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Principal: &PrincipalSpec{
						Enabled: &enabled,
						Auth:    "mtls:CN=([^,]+)",
						Server: &PrincipalServerSpec{
							Service: ArgoCDAgentPrincipalServiceSpec{
								Type: corev1.ServiceTypeNodePort,
							},
							Route: ArgoCDAgentPrincipalRouteSpec{
								Enabled: ptr.To(true),
							},
						},
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Principal Full Configuration",
			input: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := true
				enableWebSocket := true
				enableNamespaceCreate := true
				allowGenerate := true
				insecureGenerate := true
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Principal: &v1beta1.PrincipalSpec{
						Enabled:   &enabled,
						Auth:      "mtls:CN=([^,]+)",
						LogLevel:  "info",
						LogFormat: "text",
						Image:     "quay.io/user/argocd-agent:v1",
						Env: []corev1.EnvVar{
							{Name: "TEST_ENV", Value: "test-value"},
						},
						Server: &v1beta1.PrincipalServerSpec{
							EnableWebSocket:      &enableWebSocket,
							KeepAliveMinInterval: "30s",
							Service: v1beta1.ArgoCDAgentPrincipalServiceSpec{
								Type: corev1.ServiceTypeExternalName,
							},
							Route: v1beta1.ArgoCDAgentPrincipalRouteSpec{
								Enabled: ptr.To(false),
							},
						},
						Redis: &v1beta1.PrincipalRedisSpec{
							ServerAddress:   "redis:6379",
							CompressionType: "gzip",
						},
						Namespace: &v1beta1.PrincipalNamespaceSpec{
							AllowedNamespaces:      []string{"*"},
							EnableNamespaceCreate:  &enableNamespaceCreate,
							NamespaceCreatePattern: "agent-.*",
							NamespaceCreateLabels:  []string{"environment=agent"},
						},
						TLS: &v1beta1.PrincipalTLSSpec{
							SecretName:       "tls-secret",
							RootCASecretName: "ca-secret",
							InsecureGenerate: &allowGenerate,
						},
						ResourceProxy: &v1beta1.PrincipalResourceProxySpec{
							SecretName:   "proxy-secret",
							CASecretName: "proxy-ca-secret",
						},
						JWT: &v1beta1.PrincipalJWTSpec{
							InsecureGenerate: &insecureGenerate,
							SecretName:       "jwt-secret",
						},
					},
				}
			}),
			expectedOutput: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := true
				enableWebSocket := true
				enableNamespaceCreate := true
				allowGenerate := true
				insecureGenerate := true
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Principal: &PrincipalSpec{
						Enabled:   &enabled,
						Auth:      "mtls:CN=([^,]+)",
						LogLevel:  "info",
						LogFormat: "text",
						Image:     "quay.io/user/argocd-agent:v1",
						Env: []corev1.EnvVar{
							{Name: "TEST_ENV", Value: "test-value"},
						},
						Server: &PrincipalServerSpec{
							EnableWebSocket:      &enableWebSocket,
							KeepAliveMinInterval: "30s",
							Service: ArgoCDAgentPrincipalServiceSpec{
								Type: corev1.ServiceTypeExternalName,
							},
							Route: ArgoCDAgentPrincipalRouteSpec{
								Enabled: ptr.To(false),
							},
						},
						Redis: &PrincipalRedisSpec{
							ServerAddress:   "redis:6379",
							CompressionType: "gzip",
						},
						Namespace: &PrincipalNamespaceSpec{
							AllowedNamespaces:      []string{"*"},
							EnableNamespaceCreate:  &enableNamespaceCreate,
							NamespaceCreatePattern: "agent-.*",
							NamespaceCreateLabels:  []string{"environment=agent"},
						},
						TLS: &PrincipalTLSSpec{
							SecretName:       "tls-secret",
							RootCASecretName: "ca-secret",
							InsecureGenerate: &allowGenerate,
						},
						ResourceProxy: &PrincipalResourceProxySpec{
							SecretName:   "proxy-secret",
							CASecretName: "proxy-ca-secret",
						},
						JWT: &PrincipalJWTSpec{
							InsecureGenerate: &insecureGenerate,
							SecretName:       "jwt-secret",
						},
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Principal Disabled",
			input: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := false
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Principal: &v1beta1.PrincipalSpec{
						Enabled: &enabled,
					},
				}
			}),
			expectedOutput: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := false
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Principal: &PrincipalSpec{
						Enabled: &enabled,
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Agent Basic",
			input: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := true
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Agent: &v1beta1.AgentSpec{
						Enabled: &enabled,
						Creds:   "mtls:any",
						Client: &v1beta1.AgentClientSpec{
							PrincipalServerAddress: "argocd-agent-principal.example.com",
							PrincipalServerPort:    "443",
							Mode:                   "managed",
						},
					},
				}
			}),
			expectedOutput: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := true
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Agent: &AgentSpec{
						Enabled: &enabled,
						Creds:   "mtls:any",
						Client: &AgentClientSpec{
							PrincipalServerAddress: "argocd-agent-principal.example.com",
							PrincipalServerPort:    "443",
							Mode:                   "managed",
						},
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Agent Full Configuration",
			input: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := true
				enableWebSocket := true
				enableCompression := true
				insecure := true
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Agent: &v1beta1.AgentSpec{
						Enabled:   &enabled,
						Creds:     "mtls:any",
						LogLevel:  "info",
						LogFormat: "text",
						Image:     "quay.io/user/argocd-agent:v1",
						Env: []corev1.EnvVar{
							{Name: "TEST_ENV", Value: "test-value"},
						},
						Client: &v1beta1.AgentClientSpec{
							PrincipalServerAddress: "argocd-agent-principal.example.com",
							PrincipalServerPort:    "443",
							Mode:                   "managed",
							EnableWebSocket:        &enableWebSocket,
							EnableCompression:      &enableCompression,
							KeepAliveInterval:      "30s",
						},
						Redis: &v1beta1.AgentRedisSpec{
							ServerAddress: "redis:6379",
						},
						TLS: &v1beta1.AgentTLSSpec{
							SecretName:       "agent-tls-secret",
							RootCASecretName: "agent-ca-secret",
							Insecure:         &insecure,
						},
					},
				}
			}),
			expectedOutput: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := true
				enableWebSocket := true
				enableCompression := true
				insecure := true
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Agent: &AgentSpec{
						Enabled:   &enabled,
						Creds:     "mtls:any",
						LogLevel:  "info",
						LogFormat: "text",
						Image:     "quay.io/user/argocd-agent:v1",
						Env: []corev1.EnvVar{
							{Name: "TEST_ENV", Value: "test-value"},
						},
						Client: &AgentClientSpec{
							PrincipalServerAddress: "argocd-agent-principal.example.com",
							PrincipalServerPort:    "443",
							Mode:                   "managed",
							EnableWebSocket:        &enableWebSocket,
							EnableCompression:      &enableCompression,
							KeepAliveInterval:      "30s",
						},
						Redis: &AgentRedisSpec{
							ServerAddress: "redis:6379",
						},
						TLS: &AgentTLSSpec{
							SecretName:       "agent-tls-secret",
							RootCASecretName: "agent-ca-secret",
							Insecure:         &insecure,
						},
					},
				}
			}),
		},
		{
			name: "ArgoCD Example - Agent Agent Disabled",
			input: makeTestArgoCDBeta(func(cr *v1beta1.ArgoCD) {
				enabled := false
				cr.Spec.ArgoCDAgent = &v1beta1.ArgoCDAgentSpec{
					Agent: &v1beta1.AgentSpec{
						Enabled: &enabled,
					},
				}
			}),
			expectedOutput: makeTestArgoCDAlpha(func(cr *ArgoCD) {
				enabled := false
				cr.Spec.ArgoCDAgent = &ArgoCDAgentSpec{
					Agent: &AgentSpec{
						Enabled: &enabled,
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
			err := result.ConvertFrom(hub)
			assert.NoError(t, err)

			// Compare converted object with expected.
			assert.Equal(t, test.expectedOutput, result)
		})
	}
}
