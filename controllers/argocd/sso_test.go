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
	"errors"
	"testing"

	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	testclient "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

func TestReconcile_illegalSSOConfiguration(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name                     string
		argoCD                   *argoproj.ArgoCD
		wantErr                  bool
		Err                      error
		wantSSOConfigLegalStatus string
	}{
		{
			name:                     "no conflicts - no sso configured",
			argoCD:                   makeTestArgoCD(func(ac *argoproj.ArgoCD) {}),
			wantErr:                  false,
			wantSSOConfigLegalStatus: "",
		},
		{
			name: "no conflict - case insensitive sso provider value",
			argoCD: makeTestArgoCD(func(ac *argoproj.ArgoCD) {
				ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: "DEX",
					Dex: &argoproj.ArgoCDDexSpec{
						Config: "test-config",
					},
				}
			}),
			wantErr:                  false,
			wantSSOConfigLegalStatus: "",
		},
		{
			name: "no conflict - valid dex sso configurations",
			argoCD: makeTestArgoCD(func(ac *argoproj.ArgoCD) {
				ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: "dex",
					Dex: &argoproj.ArgoCDDexSpec{
						Config:         "test-config",
						OpenShiftOAuth: false,
					},
				}
			}),
			wantErr:                  false,
			wantSSOConfigLegalStatus: "",
		},
		{
			name: "keycloak sso configurations are no longer supported",
			argoCD: makeTestArgoCD(func(ac *argoproj.ArgoCD) {
				ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: "keycloak",
				}
			}),
			wantErr:                  true,
			Err:                      errors.New("keycloak is set as SSO provider, but keycloak support has been deprecated and is no longer available"),
			wantSSOConfigLegalStatus: "Failed",
		},
		{
			name: "sso provider dex but no .spec.sso.dex provided",
			argoCD: makeTestArgoCD(func(ac *argoproj.ArgoCD) {
				ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
				}
			}),
			wantErr:                  true,
			Err:                      errors.New("illegal SSO configuration: must supply valid dex configuration when requested SSO provider is dex"),
			wantSSOConfigLegalStatus: "Failed",
		},
		{
			name: "sso provider missing but sso.dex/keycloak supplied",
			argoCD: makeTestArgoCD(func(ac *argoproj.ArgoCD) {
				ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Dex: &argoproj.ArgoCDDexSpec{
						Config:         "test-config",
						OpenShiftOAuth: true,
					},
					Keycloak: &argoproj.ArgoCDKeycloakSpec{
						Image: "test-image",
					},
				}
			}),
			wantErr:                  true,
			Err:                      errors.New("illegal SSO configuration: Cannot specify SSO provider spec without specifying SSO provider type"),
			wantSSOConfigLegalStatus: "Failed",
		},
		{
			name: "unsupported sso provider but sso.dex supplied",
			argoCD: makeTestArgoCD(func(ac *argoproj.ArgoCD) {
				ac.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: "Unsupported",
					Dex: &argoproj.ArgoCDDexSpec{
						Config:         "test-config",
						OpenShiftOAuth: true,
					},
				}
			}),
			wantErr:                  true,
			Err:                      errors.New("illegal SSO configuration: Unsupported SSO provider type. Supported provider is dex"),
			wantSSOConfigLegalStatus: "Failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme, templatev1.Install, oappsv1.Install, routev1.Install)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
			reqState := &RequestState{}

			assert.NoError(t, createNamespace(r, reqState, test.argoCD.Namespace, ""))

			argoCDStatus := argoproj.ArgoCDStatus{}

			err := r.reconcileSSO(test.argoCD, &argoCDStatus, reqState)
			assert.Equal(t, test.wantSSOConfigLegalStatus, argoCDStatus.SSO)
			if err != nil {
				if !test.wantErr {
					// ignore unexpected errors for legal sso configurations.
					// keycloak reconciliation code expects a live cluster &
					// therefore throws unexpected errors during unit testing
					if argoCDStatus.SSO != ssoLegalSuccess {
						t.Errorf("Got unexpected error")
					}
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
