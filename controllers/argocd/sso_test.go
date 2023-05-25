// Copyright 2021 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argocd

import (
	"context"
	"errors"
	"sort"
	"testing"

	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/stretchr/testify/assert"
	k8sappsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argov1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
)

func makeFakeReconciler(t *testing.T, acd *argov1alpha1.ArgoCD, objs ...runtime.Object) *ReconcileArgoCD {
	t.Helper()
	s := scheme.Scheme
	// Register template scheme
	s.AddKnownTypes(templatev1.SchemeGroupVersion, objs...)
	s.AddKnownTypes(oappsv1.SchemeGroupVersion, objs...)
	assert.NoError(t, argov1alpha1.AddToScheme(s))
	templatev1.Install(s)
	oappsv1.Install(s)
	routev1.Install(s)

	cl := fake.NewFakeClientWithScheme(s, objs...)
	return &ReconcileArgoCD{
		Client: cl,
		Scheme: s,
	}
}

func TestReconcile_testKeycloakTemplateInstance(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloak()

	templateAPIFound = true
	r := makeFakeReconciler(t, a)
	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	assert.NoError(t, r.reconcileSSO(a))

	templateInstance := &templatev1.TemplateInstance{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "rhsso",
			Namespace: a.Namespace,
		},
		templateInstance))
}

func TestReconcile_noTemplateInstance(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloak()
	r := makeFakeReconciler(t, a)
	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	assert.NoError(t, r.reconcileSSO(a))
}

func TestReconcile_illegalSSOConfiguration(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name          string
		argoCD        *argov1alpha1.ArgoCD
		envVar        string
		setEnvVarFunc func(*testing.T, string)
		wantErr       bool
		Err           error
	}{
		{
			name:          "no conflicts - no sso configured",
			argoCD:        makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {}),
			setEnvVarFunc: nil,
			envVar:        "",
			wantErr:       false,
		},
		{
			name: "sso provider dex + DISABLE_DEX",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: v1alpha1.SSOProviderTypeDex,
					Dex: &v1alpha1.ArgoCDDexSpec{
						Config: "test-config",
					},
				}
			}),
			setEnvVarFunc: func(t *testing.T, envVar string) {
				t.Setenv("DISABLE_DEX", envVar)
			},
			envVar:  "true",
			wantErr: true,
			Err:     errors.New("illegal SSO configuration: cannot set DISABLE_DEX to true when dex is configured through .spec.sso"),
		},
		{
			name: "sso provider dex + non empty, conflicting `.spec.dex` fields",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: v1alpha1.SSOProviderTypeDex,
					Dex: &v1alpha1.ArgoCDDexSpec{
						Config:         "",
						OpenShiftOAuth: true,
					},
				}
				ac.Spec.Dex = &v1alpha1.ArgoCDDexSpec{
					Config:         "non-empty-config",
					OpenShiftOAuth: true,
				}
			}),
			setEnvVarFunc: nil,
			envVar:        "",
			wantErr:       true,
			Err:           errors.New("illegal SSO configuration: cannot specify .spec.Dex fields when dex is configured through .spec.sso.dex"),
		},
		{
			name: "sso provider dex but no .spec.sso.dex provided",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: v1alpha1.SSOProviderTypeDex,
				}
			}),
			setEnvVarFunc: nil,
			envVar:        "",
			wantErr:       true,
			Err:           errors.New("illegal SSO configuration: must suppy valid dex configuration when requested SSO provider is dex"),
		},
		{
			name: "sso provider dex + `.spec.sso` fields provided",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: v1alpha1.SSOProviderTypeDex,
					Dex: &v1alpha1.ArgoCDDexSpec{
						Config: "test",
					},
					Image:   "test-image",
					Version: "test-image-version",
				}
			}),
			setEnvVarFunc: nil,
			envVar:        "",
			wantErr:       true,
			Err:           errors.New("illegal SSO configuration: cannot supply keycloak configuration in spec.sso when requested SSO provider is dex"),
		},
		{
			name: "sso provider dex + `.spec.sso.keycloak`",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Keycloak: &v1alpha1.ArgoCDKeycloakSpec{
						Image:   "test-image",
						Version: "test-image-version",
					},
					Provider: argov1alpha1.SSOProviderTypeDex,
					Dex: &v1alpha1.ArgoCDDexSpec{
						Config: "test",
					},
				}
			}),
			setEnvVarFunc: nil,
			envVar:        "",
			wantErr:       true,
			Err:           errors.New("illegal SSO configuration: cannot supply keycloak configuration in .spec.sso.keycloak when requested SSO provider is dex"),
		},
		{
			name: "DISABLE_DEX + `.spec.sso.keycloak`",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Keycloak: &v1alpha1.ArgoCDKeycloakSpec{
						Image:   "test-image",
						Version: "test-image-version",
					},
				}
			}),
			setEnvVarFunc: func(t *testing.T, envVar string) {
				t.Setenv("DISABLE_DEX", envVar)
			},
			envVar:  "false",
			wantErr: true,
			Err:     errors.New("illegal SSO configuration: Cannot specify SSO provider spec without specifying SSO provider type"),
		},
		{
			name: "no conflicts - `DISABLE_DEX` + `.spec.sso` fields",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Image:   "test-image",
					Version: "test-image-version",
				}
			}),
			setEnvVarFunc: func(t *testing.T, envVar string) {
				t.Setenv("DISABLE_DEX", envVar)
			},
			envVar:  "true",
			wantErr: false,
		},
		{
			name: "sso provider keycloak + `.spec.dex.OpenShiftOAuth`",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argov1alpha1.SSOProviderTypeKeycloak,
				}
				ac.Spec.Dex = &v1alpha1.ArgoCDDexSpec{
					OpenShiftOAuth: true,
				}
			}),
			setEnvVarFunc: nil,
			envVar:        "",
			wantErr:       true,
			Err:           errors.New("multiple SSO configuration: multiple SSO providers configured simultaneously"),
		},
		{
			name: "sso provider keycloak + `.spec.sso` + `.spec.sso.keycloak",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argov1alpha1.SSOProviderTypeKeycloak,
					Image:    "test-image",
					Keycloak: &v1alpha1.ArgoCDKeycloakSpec{
						Version: "test-image-version-2",
					},
				}
			}),
			setEnvVarFunc: nil,
			envVar:        "",
			wantErr:       true,
			Err:           errors.New("illegal SSO configuration: cannot specify keycloak fields in .spec.sso when keycloak is configured through .spec.sso.keycloak"),
		},
		{
			name: "sso provider keycloak + `.spec.sso.dex`",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argov1alpha1.SSOProviderTypeKeycloak,
					Dex: &v1alpha1.ArgoCDDexSpec{
						Config:         "test-config",
						OpenShiftOAuth: true,
					},
				}
			}),
			setEnvVarFunc: nil,
			envVar:        "",
			wantErr:       true,
			Err:           errors.New("illegal SSO configuration: cannot supply dex configuration when requested SSO provider is keycloak"),
		},
		{
			name: "sso provider missing but sso.dex/keycloak supplied",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Dex: &v1alpha1.ArgoCDDexSpec{
						Config:         "test-config",
						OpenShiftOAuth: true,
					},
					Keycloak: &v1alpha1.ArgoCDKeycloakSpec{
						Image: "test-image",
					},
				}
			}),
			setEnvVarFunc: nil,
			envVar:        "",
			wantErr:       true,
			Err:           errors.New("illegal SSO configuration: Cannot specify SSO provider spec without specifying SSO provider type"),
		},
		{
			name: "no conflict - no provider but .spec.sso fields supplied",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Image:   "test-image",
					Version: "test-image-version",
				}
			}),
			setEnvVarFunc: nil,
			envVar:        "",
			wantErr:       false,
		},
		{
			name: "no conflict (preserve existing behavior) sso provider keycloak + DISABLE_DEX",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argov1alpha1.SSOProviderTypeKeycloak,
				}
				ac.Spec.Dex = &v1alpha1.ArgoCDDexSpec{
					OpenShiftOAuth: false,
				}
			}),
			setEnvVarFunc: func(t *testing.T, envVar string) {
				t.Setenv("DISABLE_DEX", envVar)
			},
			envVar:  "true",
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := makeTestReconciler(t, test.argoCD)
			assert.NoError(t, createNamespace(r, test.argoCD.Namespace, ""))

			if test.setEnvVarFunc != nil {
				test.setEnvVarFunc(t, test.envVar)
			}

			err := r.reconcileSSO(test.argoCD)
			if err != nil {
				if !test.wantErr {
					t.Errorf("Got unexpected error")
				} else {
					assert.Equal(t, test.Err, err)
				}
			} else {
				if test.wantErr {
					t.Errorf("expected error but didn't get one")
				}
			}
		})
	}

}

func TestReconcile_emitEventOnDetectingDeprecatedFields(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	DeprecationEventEmissionTracker = make(map[string]DeprecationEventEmissionStatus)

	disableDexEvent := &corev1.Event{
		Reason:  "DeprecationNotice",
		Message: "`DISABLE_DEX` is deprecated, and support will be removed in Argo CD Operator v0.8.0/OpenShift GitOps v1.10.0. Dex can be enabled/disabled through `.spec.sso`",
		Action:  "Deprecated",
	}

	specDexEvent := &corev1.Event{
		Reason:  "DeprecationNotice",
		Message: "`.spec.dex` is deprecated, and support will be removed in Argo CD Operator v0.8.0/OpenShift GitOps v1.10.0. Dex configuration can be managed through `.spec.sso.dex`",
		Action:  "Deprecated",
	}

	specSSOEvent := &corev1.Event{
		Reason:  "DeprecationNotice",
		Message: "`.spec.SSO.Image`, `.spec.SSO.Version`, `.spec.SSO.Resources` and `.spec.SSO.VerifyTLS` are deprecated, and support will be removed in Argo CD Operator v0.8.0/OpenShift GitOps v1.10.0. Keycloak configuration can be managed through `.spec.sso.keycloak`",
		Action:  "Deprecated",
	}

	tests := []struct {
		name          string
		argoCD        *argov1alpha1.ArgoCD
		envVar        string
		setEnvVarFunc func(*testing.T, string)
		wantEvents    []*corev1.Event
	}{
		{
			name:   "DISABLE_DEX env var in use",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {}),
			envVar: "true",
			setEnvVarFunc: func(t *testing.T, envVar string) {
				t.Setenv("DISABLE_DEX", envVar)
			},
			wantEvents: []*corev1.Event{disableDexEvent},
		},
		{
			name: ".spec.dex in use",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.Dex = &argov1alpha1.ArgoCDDexSpec{
					Config:  "",
					Groups:  []string{},
					Image:   "",
					Version: "",
				}
			}),
			envVar:        "",
			setEnvVarFunc: nil,
			wantEvents:    []*corev1.Event{specDexEvent},
		},
		{
			name: ".spec.sso in use",
			argoCD: makeTestArgoCD(func(ac *argov1alpha1.ArgoCD) {
				ac.Spec.SSO = &argov1alpha1.ArgoCDSSOSpec{
					Image:   "test-image",
					Version: "test-image-version",
				}
			}),
			envVar:        "",
			setEnvVarFunc: nil,
			wantEvents:    []*corev1.Event{specSSOEvent},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r := makeFakeReconciler(t, test.argoCD)

			if test.setEnvVarFunc != nil {
				test.setEnvVarFunc(t, test.envVar)
			}
			err := r.reconcileSSO(test.argoCD)
			assert.NoError(t, err)

			gotEventList := &corev1.EventList{}

			err = r.Client.List(context.TODO(), gotEventList)
			assert.NoError(t, err)
			assert.Equal(t, len(test.wantEvents), len(gotEventList.Items))

			sort.Slice(gotEventList.Items, func(i, j int) bool {
				return gotEventList.Items[i].Message < gotEventList.Items[j].Message
			})

			sort.Slice(test.wantEvents, func(i, j int) bool {
				return test.wantEvents[i].Message < test.wantEvents[j].Message
			})

			for i := range gotEventList.Items {
				assert.Equal(t, test.wantEvents[i].Message, gotEventList.Items[i].Message)
			}

		})
	}
}

func TestReconcile_testKeycloakK8sInstance(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloak()

	// Cluster does not have a template instance
	templateAPIFound = false
	r := makeReconciler(t, a)
	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	assert.NoError(t, r.reconcileSSO(a))
}

func TestReconcile_testKeycloakInstanceResources(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloak()

	// Cluster does not have a template instance
	templateAPIFound = false
	r := makeReconciler(t, a)
	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	assert.NoError(t, r.reconcileSSO(a))

	// Keycloak Deployment
	deployment := &k8sappsv1.Deployment{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: defaultKeycloakIdentifier, Namespace: a.Namespace}, deployment)
	assert.NoError(t, err)

	assert.Equal(t, deployment.Name, defaultKeycloakIdentifier)
	assert.Equal(t, deployment.Namespace, a.Namespace)

	testLabels := map[string]string{
		"app": defaultKeycloakIdentifier,
	}
	assert.Equal(t, deployment.Labels, testLabels)

	testSelector := &v1.LabelSelector{
		MatchLabels: map[string]string{
			"app": defaultKeycloakIdentifier,
		},
	}
	assert.Equal(t, deployment.Spec.Selector, testSelector)

	assert.Equal(t, deployment.Spec.Template.ObjectMeta.Labels, testLabels)
	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Name,
		defaultKeycloakIdentifier)
	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Image,
		getKeycloakContainerImage(a))

	testEnv := []corev1.EnvVar{
		{Name: "KEYCLOAK_USER", Value: defaultKeycloakAdminUser},
		{Name: "KEYCLOAK_PASSWORD", Value: defaultKeycloakAdminPassword},
		{Name: "PROXY_ADDRESS_FORWARDING", Value: "true"},
	}
	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].Env,
		testEnv)

	// Keycloak Service
	svc := &corev1.Service{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: defaultKeycloakIdentifier, Namespace: a.Namespace}, svc)
	assert.NoError(t, err)

	assert.Equal(t, svc.Name, defaultKeycloakIdentifier)
	assert.Equal(t, svc.Namespace, a.Namespace)
	assert.Equal(t, svc.Labels, testLabels)

	assert.Equal(t, svc.Spec.Selector, testLabels)
	assert.Equal(t, svc.Spec.Type, corev1.ServiceType("LoadBalancer"))

	// Keycloak Ingress
	ing := &networkingv1.Ingress{}
	testPathType := networkingv1.PathTypeImplementationSpecific
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: defaultKeycloakIdentifier, Namespace: a.Namespace}, ing)
	assert.NoError(t, err)

	assert.Equal(t, ing.Name, defaultKeycloakIdentifier)
	assert.Equal(t, ing.Namespace, a.Namespace)

	testTLS := []networkingv1.IngressTLS{
		{
			Hosts: []string{keycloakIngressHost},
		},
	}
	assert.Equal(t, ing.Spec.TLS, testTLS)

	testRules := []networkingv1.IngressRule{
		{
			Host: keycloakIngressHost,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path: "/",
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: defaultKeycloakIdentifier,
									Port: networkingv1.ServiceBackendPort{
										Name: "http",
									},
								},
							},
							PathType: &testPathType,
						},
					},
				},
			},
		},
	}

	assert.Equal(t, ing.Spec.Rules, testRules)
}
