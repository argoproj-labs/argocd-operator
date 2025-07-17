// Copyright 2020 ArgoCD Operator Developers
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
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

var _ reconcile.Reconciler = &ReconcileArgoCD{}

func TestReconcileArgoCD_reconcileTLSCerts(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(initialCerts(t, "root-ca.example.com"))

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, r.reconcileTLSCerts(a))

	configMap := &corev1.ConfigMap{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      common.ArgoCDTLSCertsConfigMapName,
			Namespace: a.Namespace,
		},
		configMap))

	want := []string{"root-ca.example.com"}
	if k := stringMapKeys(configMap.Data); !reflect.DeepEqual(want, k) {
		t.Fatalf("got %#v, want %#v\n", k, want)
	}
}

func TestReconcileArgoCD_reconcileTLSCerts_configMapUpdate(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(initialCerts(t, "root-ca.example.com"))

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, r.reconcileTLSCerts(a))

	configMap := &corev1.ConfigMap{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      common.ArgoCDTLSCertsConfigMapName,
			Namespace: a.Namespace,
		},
		configMap))

	want := []string{"root-ca.example.com"}
	if k := stringMapKeys(configMap.Data); !reflect.DeepEqual(want, k) {
		t.Fatalf("got %#v, want %#v\n", k, want)
	}

	// update a new cert in argocd-tls-certs-cm
	testPEM := generateEncodedPEM(t)

	configMap.Data["example.com"] = string(testPEM)
	assert.NoError(t, r.Client.Update(context.TODO(), configMap))

	// verify that a new reconciliation does not remove example.com from
	// argocd-tls-certs-cm
	assert.NoError(t, r.reconcileTLSCerts(a))

	want = []string{"example.com", "root-ca.example.com"}
	if k := stringMapKeys(configMap.Data); !reflect.DeepEqual(want, k) {
		t.Fatalf("got %#v, want %#v\n", k, want)
	}
}

// TestReconcileArgoCD_reconcileRedisHAHealthConfigMap tests the reconcileRedisHAHealthConfigMap function.
func TestReconcileArgoCD_reconcileRedisHAHealthConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Create a test ArgoCD resource with HA enabled
	cr := makeTestArgoCD()
	cr.Spec.HA.Enabled = true

	// Initialize test objects
	resObjs := []client.Object{cr}
	subresObjs := []client.Object{}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// Perform initial reconciliation
	assert.NoError(t, r.reconcileRedisHAHealthConfigMap(cr, false))

	// Modify ConfigMap data to simulate external changes
	existingCM := &corev1.ConfigMap{}
	assert.True(t, argoutil.IsObjectFound(cl, cr.Namespace, common.ArgoCDRedisHAHealthConfigMapName, existingCM))
	existingCM.Data["redis_liveness.sh"] = "modified_script_content"
	assert.NoError(t, cl.Update(context.TODO(), existingCM))

	// Reconcile again and verify changes are reverted
	assert.NoError(t, r.reconcileRedisHAHealthConfigMap(cr, false))
	existingCMAfter := &corev1.ConfigMap{}
	assert.True(t, argoutil.IsObjectFound(cl, cr.Namespace, common.ArgoCDRedisHAHealthConfigMapName, existingCMAfter))
	assert.Equal(t, getRedisLivenessScript(false), existingCMAfter.Data["redis_liveness.sh"])

	// Disable HA and ensure ConfigMap is deleted
	cr.Spec.HA.Enabled = false
	assert.NoError(t, r.reconcileRedisHAHealthConfigMap(cr, false))
	assert.False(t, argoutil.IsObjectFound(cl, cr.Namespace, common.ArgoCDRedisHAHealthConfigMapName, existingCM))
}

// TestReconcileArgoCD_reconcileRedisHAConfigMap tests the reconcileRedisHAConfigMap function.
func TestReconcileArgoCD_reconcileRedisHAConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Create a test ArgoCD resource with HA enabled
	cr := makeTestArgoCD()
	cr.Spec.HA.Enabled = true

	// Initialize test objects
	resObjs := []client.Object{cr}
	subresObjs := []client.Object{}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// Perform initial reconciliation
	assert.NoError(t, r.reconcileRedisHAConfigMap(cr, false))

	// Modify ConfigMap data to simulate external changes
	existingCM := &corev1.ConfigMap{}
	assert.True(t, argoutil.IsObjectFound(cl, cr.Namespace, common.ArgoCDRedisHAConfigMapName, existingCM))
	existingCM.Data["haproxy.cfg"] = "modified_config_content"
	assert.NoError(t, cl.Update(context.TODO(), existingCM))

	// Reconcile again and verify changes are reverted
	assert.NoError(t, r.reconcileRedisHAConfigMap(cr, false))
	existingCMAfter := &corev1.ConfigMap{}
	assert.True(t, argoutil.IsObjectFound(cl, cr.Namespace, common.ArgoCDRedisHAConfigMapName, existingCMAfter))
	assert.Equal(t, getRedisHAProxyConfig(cr, false), existingCMAfter.Data["haproxy.cfg"])

	// Disable HA and ensure ConfigMap is deleted
	cr.Spec.HA.Enabled = false
	assert.NoError(t, r.reconcileRedisHAConfigMap(cr, false))
	assert.False(t, argoutil.IsObjectFound(cl, cr.Namespace, common.ArgoCDRedisHAConfigMapName, existingCM))
}

func TestReconcileArgoCD_reconcileTLSCerts_withInitialCertsUpdate(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, r.reconcileTLSCerts(a))

	a = makeTestArgoCD(initialCerts(t, "testing.example.com"))
	assert.NoError(t, r.reconcileTLSCerts(a))

	configMap := &corev1.ConfigMap{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      common.ArgoCDTLSCertsConfigMapName,
			Namespace: a.Namespace,
		},
		configMap))

	// Any certs added to .spec.tls.intialCerts of Argo CD CR after the cluster creation
	// should not affect the argocd-tls-certs-cm configmap.
	want := []string{}
	if k := stringMapKeys(configMap.Data); !reflect.DeepEqual(want, k) {
		t.Fatalf("got %#v, want %#v\n", k, want)
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	defaultConfigMapData := map[string]string{
		"application.instanceLabelKey":       common.ArgoCDDefaultApplicationInstanceLabelKey,
		"application.resourceTrackingMethod": argoproj.ResourceTrackingMethodLabel.String(),
		"admin.enabled":                      "true",
		"dex.config":                         "",
		"ga.anonymizeusers":                  "false",
		"ga.trackingid":                      "",
		"help.chatText":                      "",
		"help.chatUrl":                       "",
		"kustomize.buildOptions":             "",
		"oidc.config":                        "",
		"resource.inclusions":                "",
		"resource.exclusions":                "",
		"server.rbac.disableApplicationFineGrainedRBACInheritance": "false",
		"statusbadge.enabled":     "false",
		"url":                     "https://argocd-server",
		"users.anonymous.enabled": "false",
	}

	cmdTests := []struct {
		name     string
		opts     []argoCDOpt
		dataDiff map[string]string
	}{
		{
			"defaults",
			[]argoCDOpt{},
			map[string]string{},
		},
		{
			"with-banner",
			[]argoCDOpt{func(a *argoproj.ArgoCD) {
				a.Spec.Banner = &argoproj.Banner{
					Content: "Custom Styles - Banners",
				}
			}},
			map[string]string{
				"users.anonymous.enabled": "false",
				"ui.bannercontent":        "Custom Styles - Banners",
			},
		},
		{
			"with-banner-and-url",
			[]argoCDOpt{func(a *argoproj.ArgoCD) {
				a.Spec.Banner = &argoproj.Banner{
					Content: "Custom Styles - Banners",
					URL:     "https://argo-cd.readthedocs.io/en/stable/operator-manual/custom-styles/#banners",
				}
			}},
			map[string]string{
				"ui.bannercontent": "Custom Styles - Banners",
				"ui.bannerurl":     "https://argo-cd.readthedocs.io/en/stable/operator-manual/custom-styles/#banners",
			},
		},
	}

	for _, tt := range cmdTests {
		a := makeTestArgoCD(tt.opts...)
		a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
			Provider: argoproj.SSOProviderTypeDex,
		}

		resObjs := []client.Object{a}
		subresObjs := []client.Object{a}
		runtimeObjs := []runtime.Object{}
		sch := makeTestReconcilerScheme(argoproj.AddToScheme)
		cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
		r := makeTestReconciler(cl, sch)

		err := r.reconcileArgoConfigMap(a)
		assert.NoError(t, err)

		cm := &corev1.ConfigMap{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      common.ArgoCDConfigMapName,
			Namespace: testNamespace,
		}, cm)
		assert.NoError(t, err)

		want := merge(defaultConfigMapData, tt.dataDiff)

		if diff := cmp.Diff(want, cm.Data); diff != "" {
			t.Fatalf("reconcileArgoConfigMap (%s) failed:\n%s", tt.name, diff)
		}
	}
}

func TestReconcileArgoCD_reconcileEmptyArgoConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// An empty Argo CD Configmap
	emptyArgoConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: a.Namespace,
		},
	}

	err := r.Client.Create(context.TODO(), emptyArgoConfigmap)
	assert.NoError(t, err)

	err = r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withDisableAdmin(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.DisableAdmin = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	if c := cm.Data["admin.enabled"]; c != "false" {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, "false")
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withDexConnector(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	getSampleDexConfig := func(t *testing.T) []byte {
		t.Helper()

		type expiry struct {
			IdTokens    string `yaml:"idTokens"`
			SigningKeys string `yaml:"signingKeys"`
		}

		dexCfg := map[string]interface{}{
			"expiry": expiry{
				IdTokens:    "1hr",
				SigningKeys: "12hr",
			},
		}

		dexCfgBytes, err := yaml.Marshal(dexCfg)
		assert.NoError(t, err)
		return dexCfgBytes
	}

	tests := []struct {
		name             string
		updateCrSpecFunc func(cr *argoproj.ArgoCD)
	}{
		{
			name: "dex config using .spec.sso.provider=dex + .spec.sso.dex",
			updateCrSpecFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			},
		},
		{
			name: "update .dex.config and verify that the dex connector is not overwritten",
			updateCrSpecFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
						Config:         string(getSampleDexConfig(t)),
					},
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				TypeMeta:   metav1.TypeMeta{Kind: "ServiceAccount", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-dex-server", Namespace: "argocd"},
				Secrets: []corev1.ObjectReference{{
					Name: "token",
				}},
			}

			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: false,
					},
				}
			})

			secret := argoutil.NewSecretWithName(a, "token")

			resObjs := []client.Object{a, sa, secret}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)

			if test.updateCrSpecFunc != nil {
				test.updateCrSpecFunc(a)
			}
			err := r.reconcileArgoConfigMap(a)
			assert.NoError(t, err)

			cm := &corev1.ConfigMap{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      common.ArgoCDConfigMapName,
				Namespace: testNamespace,
			}, cm)
			assert.NoError(t, err)

			dex, ok := cm.Data["dex.config"]
			if !ok {
				t.Fatal("reconcileArgoConfigMap with dex failed")
			}

			m := make(map[string]interface{})
			err = yaml.Unmarshal([]byte(dex), &m)
			assert.NoError(t, err, fmt.Sprintf("failed to unmarshal %s", dex))

			connectors, ok := m["connectors"]
			if !ok {
				t.Fatal("no connectors found in dex.config")
			}
			dexConnector := connectors.([]interface{})[0].(map[interface{}]interface{})
			config := dexConnector["config"]
			assert.Equal(t, config.(map[interface{}]interface{})["clientID"], "system:serviceaccount:argocd:argocd-argocd-dex-server")

			// verify that the dex config in the CR matches the config from the argocd-cm
			if a.Spec.SSO.Dex.Config != "" {
				expectedCfg := make(map[string]interface{})
				expectedCfgStr, err := r.getOpenShiftDexConfig(a)
				assert.NoError(t, err)

				err = yaml.Unmarshal([]byte(expectedCfgStr), expectedCfg)
				assert.NoError(t, err, fmt.Sprintf("failed to unmarshal %s", dex))
				assert.Equal(t, expectedCfg, m)
			}
		})
	}

}

func TestReconcileArgoCD_reconcileArgoConfigMap_withDexDisabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name   string
		argoCD *argoproj.ArgoCD
	}{
		{
			name: "dex disabled by removing .spec.sso",
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = nil
			}),
		},
		{
			name: "dex disabled by switching provider",
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
				}
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)

			err := r.reconcileArgoConfigMap(test.argoCD)
			assert.NoError(t, err)

			cm := &corev1.ConfigMap{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      common.ArgoCDConfigMapName,
				Namespace: testNamespace,
			}, cm)
			assert.NoError(t, err)

			if c, ok := cm.Data["dex.config"]; ok {
				t.Fatalf("reconcileArgoConfigMap failed, dex.config = %q", c)
			}
		})
	}
}

// When dex is enabled, dexConfig should be present in argocd-cm, when disabled, it should be removed
func TestReconcileArgoCD_reconcileArgoConfigMap_dexConfigDeletedwhenDexDisabled(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name              string
		updateCrFunc      func(cr *argoproj.ArgoCD)
		argoCD            *argoproj.ArgoCD
		wantConfigRemoved bool
	}{
		{
			name: "dex disabled by removing .spec.sso.provider",
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = nil
			},
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						Config: "test-dex-config",
					},
				}
			}),
			wantConfigRemoved: true,
		},
		{
			name: "dex disabled by switching provider",
			updateCrFunc: func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeKeycloak,
				}
			},
			argoCD: makeTestArgoCD(func(cr *argoproj.ArgoCD) {
				cr.Spec.SSO = &argoproj.ArgoCDSSOSpec{
					Provider: argoproj.SSOProviderTypeDex,
					Dex: &argoproj.ArgoCDDexSpec{
						OpenShiftOAuth: true,
					},
				}
			}),
			wantConfigRemoved: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				TypeMeta:   metav1.TypeMeta{Kind: "ServiceAccount", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-dex-server", Namespace: "argocd"},
				Secrets: []corev1.ObjectReference{{
					Name: "token",
				}},
			}
			secret := argoutil.NewSecretWithName(test.argoCD, "token")

			resObjs := []client.Object{test.argoCD, sa, secret}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)

			err := r.reconcileArgoConfigMap(test.argoCD)
			assert.NoError(t, err)

			cm := &corev1.ConfigMap{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      common.ArgoCDConfigMapName,
				Namespace: testNamespace,
			}, cm)
			assert.NoError(t, err)

			if _, ok := cm.Data["dex.config"]; !ok {
				t.Fatalf("reconcileArgoConfigMap failed,could not find dexConfig")
			}

			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			err = r.reconcileDexConfiguration(cm, test.argoCD)
			assert.NoError(t, err)

			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      common.ArgoCDConfigMapName,
				Namespace: testNamespace,
			}, cm)
			assert.NoError(t, err)

			if c, ok := cm.Data["dex.config"]; ok && c != "" {
				if test.wantConfigRemoved {
					t.Fatalf("reconcileArgoConfigMap failed, dex.config = %q", c)
				}
			}
		})
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withKustomizeVersions(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		kv := argoproj.KustomizeVersionSpec{
			Version: "v4.1.0",
			Path:    "/path/to/kustomize-4.1",
		}
		var kvs []argoproj.KustomizeVersionSpec
		kvs = append(kvs, kv)
		a.Spec.KustomizeVersions = kvs
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	if diff := cmp.Diff(cm.Data["kustomize.version.v4.1.0"], "/path/to/kustomize-4.1"); diff != "" {
		t.Fatalf("failed to reconcile configmap:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileGPGKeysConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.DisableAdmin = true
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileGPGKeysConfigMap(a)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDGPGKeysConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)
	// Currently the gpg keys configmap is empty
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withResourceTrackingMethod(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	t.Run("Check default tracking method", func(t *testing.T) {
		err = r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      common.ArgoCDConfigMapName,
			Namespace: testNamespace,
		}, cm)
		assert.NoError(t, err)

		rtm, ok := cm.Data[common.ArgoCDKeyResourceTrackingMethod]
		assert.Equal(t, argoproj.ResourceTrackingMethodLabel.String(), rtm)
		assert.True(t, ok)
	})

	t.Run("Tracking method label", func(t *testing.T) {
		err = r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      common.ArgoCDConfigMapName,
			Namespace: testNamespace,
		}, cm)
		assert.NoError(t, err)

		rtm, ok := cm.Data[common.ArgoCDKeyResourceTrackingMethod]
		assert.Equal(t, argoproj.ResourceTrackingMethodLabel.String(), rtm)
		assert.True(t, ok)
	})

	t.Run("Set tracking method to annotation+label", func(t *testing.T) {
		a.Spec.ResourceTrackingMethod = argoproj.ResourceTrackingMethodAnnotationAndLabel.String()
		err = r.reconcileArgoConfigMap(a)
		assert.NoError(t, err)

		err = r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      common.ArgoCDConfigMapName,
			Namespace: testNamespace,
		}, cm)
		assert.NoError(t, err)

		rtm, ok := cm.Data[common.ArgoCDKeyResourceTrackingMethod]
		assert.True(t, ok)
		assert.Equal(t, argoproj.ResourceTrackingMethodAnnotationAndLabel.String(), rtm)
	})

	t.Run("Set tracking method to annotation", func(t *testing.T) {
		a.Spec.ResourceTrackingMethod = argoproj.ResourceTrackingMethodAnnotation.String()
		err = r.reconcileArgoConfigMap(a)
		assert.NoError(t, err)

		err = r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      common.ArgoCDConfigMapName,
			Namespace: testNamespace,
		}, cm)
		assert.NoError(t, err)

		rtm, ok := cm.Data[common.ArgoCDKeyResourceTrackingMethod]
		assert.True(t, ok)
		assert.Equal(t, argoproj.ResourceTrackingMethodAnnotation.String(), rtm)
	})

	// Invalid value sets the default "label"
	t.Run("Set tracking method to invalid value", func(t *testing.T) {
		a.Spec.ResourceTrackingMethod = "anotaions"
		err = r.reconcileArgoConfigMap(a)
		assert.NoError(t, err)

		err = r.Client.Get(context.TODO(), types.NamespacedName{
			Name:      common.ArgoCDConfigMapName,
			Namespace: testNamespace,
		}, cm)
		assert.NoError(t, err)

		rtm, ok := cm.Data[common.ArgoCDKeyResourceTrackingMethod]
		assert.True(t, ok)
		assert.Equal(t, argoproj.ResourceTrackingMethodLabel.String(), rtm)
	})

}

func TestReconcileArgoCD_reconcileArgoConfigMap_withResourceInclusions(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	customizations := "testing: testing"
	updatedCustomizations := "updated-testing: updated-testing"

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ResourceInclusions = customizations
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	if c := cm.Data["resource.inclusions"]; c != customizations {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, customizations)
	}

	a.Spec.ResourceInclusions = updatedCustomizations
	err = r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	if c := cm.Data["resource.inclusions"]; c != updatedCustomizations {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, updatedCustomizations)
	}

}

func TestReconcileArgoCD_reconcileArgoConfigMap_withNewResourceCustomizations(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	desiredIgnoreDifferenceCustomization :=
		`jqpathexpressions:
- a
- b
jsonpointers:
- a
- b
managedfieldsmanagers:
- a
- b
`

	health := []argoproj.ResourceHealthCheck{
		{
			Group: "healthFoo",
			Kind:  "healthFoo",
			Check: "healthFoo",
		},
		{
			Group: "healthBar",
			Kind:  "healthBar",
			Check: "healthBar",
		},
		{
			Group: "",
			Kind:  "healthFooBar",
			Check: "healthFooBar",
		},
	}
	actions := []argoproj.ResourceAction{
		{
			Group:  "actionsFoo",
			Kind:   "actionsFoo",
			Action: "actionsFoo",
		},
		{
			Group:  "actionsBar",
			Kind:   "actionsBar",
			Action: "actionsBar",
		},
		{
			Group:  "",
			Kind:   "actionsFooBar",
			Action: "actionsFooBar",
		},
	}
	ignoreDifferences := argoproj.ResourceIgnoreDifference{
		All: &argoproj.IgnoreDifferenceCustomization{
			JqPathExpressions:     []string{"a", "b"},
			JsonPointers:          []string{"a", "b"},
			ManagedFieldsManagers: []string{"a", "b"},
		},
		ResourceIdentifiers: []argoproj.ResourceIdentifiers{
			{
				Group: "ignoreDiffBar",
				Kind:  "ignoreDiffBar",
				Customization: argoproj.IgnoreDifferenceCustomization{
					JqPathExpressions:     []string{"a", "b"},
					JsonPointers:          []string{"a", "b"},
					ManagedFieldsManagers: []string{"a", "b"},
				},
			},
			{
				Group: "",
				Kind:  "ignoreDiffFoo",
				Customization: argoproj.IgnoreDifferenceCustomization{
					JqPathExpressions:     []string{"a", "b"},
					JsonPointers:          []string{"a", "b"},
					ManagedFieldsManagers: []string{"a", "b"},
				},
			},
		},
	}

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.ResourceHealthChecks = health
		a.Spec.ResourceActions = actions
		a.Spec.ResourceIgnoreDifferences = &ignoreDifferences
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	desiredCM := make(map[string]string)
	desiredCM["resource.customizations.health.healthFoo_healthFoo"] = "healthFoo"
	desiredCM["resource.customizations.health.healthBar_healthBar"] = "healthBar"
	desiredCM["resource.customizations.health.healthFooBar"] = "healthFooBar"
	desiredCM["resource.customizations.actions.actionsFoo_actionsFoo"] = "actionsFoo"
	desiredCM["resource.customizations.actions.actionsBar_actionsBar"] = "actionsBar"
	desiredCM["resource.customizations.actions.actionsFooBar"] = "actionsFooBar"
	desiredCM["resource.customizations.ignoreDifferences.all"] = desiredIgnoreDifferenceCustomization
	desiredCM["resource.customizations.ignoreDifferences.ignoreDiffBar_ignoreDiffBar"] = desiredIgnoreDifferenceCustomization
	desiredCM["resource.customizations.ignoreDifferences.ignoreDiffFoo"] = desiredIgnoreDifferenceCustomization

	for k, v := range desiredCM {
		if value, ok := cm.Data[k]; !ok || value != v {
			t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", value, desiredCM[k])
		}
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withExtraConfig(t *testing.T) {
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	// Verify Argo CD configmap is created.
	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	// Verify that updates to the configmap are rejected(reconciled back to default) by the operator.
	cm.Data["ping"] = "pong"
	err = r.Client.Update(context.TODO(), cm)
	assert.NoError(t, err)

	err = r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	assert.Equal(t, cm.Data["ping"], "")

	// Verify that operator updates argocd-cm according to ExtraConfig.
	a.Spec.ExtraConfig = map[string]string{
		"foo": "bar",
	}

	err = r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	assert.Equal(t, cm.Data["foo"], "bar")

	// Verify that ExtraConfig overrides FirstClass entries
	a.Spec.DisableAdmin = true
	a.Spec.ExtraConfig["admin.enabled"] = "true"

	err = r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)

	assert.NoError(t, err)
	assert.Equal(t, cm.Data["admin.enabled"], "true")

	// Verify that deletion of a field from ExtraConfig does not delete any existing configuration
	// created by FirstClass citizens.
	a.Spec.ExtraConfig = make(map[string]string, 0)

	err = r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)

	assert.NoError(t, err)
	assert.Equal(t, cm.Data["admin.enabled"], "false")

}

func TestReconcileArgoCD_reconcileArgoConfigMap_withRespectRBAC(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Controller.RespectRBAC = "normal"
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: common.ArgoCDConfigMapName, Namespace: testNamespace}, cm))

	if c := cm.Data["resource.respectRBAC"]; c != "normal" {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, "false")
	}

	// update config
	a.Spec.Controller.RespectRBAC = "strict"

	err = r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: common.ArgoCDConfigMapName, Namespace: testNamespace}, cm))
	if c := cm.Data["resource.respectRBAC"]; c != "strict" {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, "false")
	}

	// update config
	a.Spec.Controller.RespectRBAC = ""

	err = r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: common.ArgoCDConfigMapName, Namespace: testNamespace}, cm))
	if c := cm.Data["resource.respectRBAC"]; c != "" {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, "false")
	}
}

func Test_reconcileRBAC(t *testing.T) {
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileRBAC(a)
	assert.NoError(t, err)

	// Verify ArgoCD CR can be used to configure the RBAC policy matcher mode.\
	matcherMode := "regex"
	a.Spec.RBAC.PolicyMatcherMode = &matcherMode

	err = r.reconcileRBAC(a)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDRBACConfigMapName,
		Namespace: testNamespace,
	}, cm)

	assert.NoError(t, err)
	assert.Equal(t, cm.Data["policy.matchMode"], matcherMode)

	// Verify when SSO different from keycloak it sync
	rbacScopes := "[groups,email]"
	cmRbacScopes := ""
	a.Spec.RBAC.Scopes = &rbacScopes
	cm.Data["scopes"] = cmRbacScopes
	a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
		Provider: argoproj.SSOProviderTypeDex,
	}
	err = r.reconcileRBAC(a)
	assert.NoError(t, err)

	cm = &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDRBACConfigMapName,
		Namespace: testNamespace,
	}, cm)

	assert.NoError(t, err)
	assert.Equal(t, rbacScopes, cm.Data["scopes"])
	assert.Equal(t, rbacScopes, *a.Spec.RBAC.Scopes)

	rbacScopes = "[groups]"
	cmRbacScopes = cm.Data["scopes"]
	a.Spec.RBAC.Scopes = &rbacScopes
	a.Spec.SSO = &argoproj.ArgoCDSSOSpec{
		Provider: argoproj.SSOProviderTypeKeycloak,
	}
	err = r.reconcileRBAC(a)
	assert.NoError(t, err)

	cm = &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDRBACConfigMapName,
		Namespace: testNamespace,
	}, cm)

	assert.NoError(t, err)
	assert.Equal(t, cmRbacScopes, cm.Data["scopes"])
	assert.Equal(t, rbacScopes, *a.Spec.RBAC.Scopes)

}

func Test_validateOwnerReferences(t *testing.T) {
	a := makeTestArgoCD()
	uid := uuid.NewUUID()
	a.UID = uid
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)
	cm := newConfigMapWithName(common.ArgoCDConfigMapName, a)

	// verify when OwnerReferences is not set
	_, err := modifyOwnerReferenceIfNeeded(a, cm, r.Scheme)
	assert.NoError(t, err)

	assert.Equal(t, cm.OwnerReferences[0].APIVersion, "argoproj.io/v1beta1")
	assert.Equal(t, cm.OwnerReferences[0].Kind, "ArgoCD")
	assert.Equal(t, cm.OwnerReferences[0].Name, "argocd")
	assert.Equal(t, cm.OwnerReferences[0].UID, uid)

	// verify when APIVersion is changed
	cm.OwnerReferences[0].APIVersion = "test"

	changed, err := modifyOwnerReferenceIfNeeded(a, cm, r.Scheme)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, cm.OwnerReferences[0].APIVersion, "argoproj.io/v1beta1")
	assert.Equal(t, cm.OwnerReferences[0].Kind, "ArgoCD")
	assert.Equal(t, cm.OwnerReferences[0].Name, "argocd")
	assert.Equal(t, cm.OwnerReferences[0].UID, uid)

	// verify when Kind is changed
	cm.OwnerReferences[0].Kind = "test"

	changed, err = modifyOwnerReferenceIfNeeded(a, cm, r.Scheme)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, cm.OwnerReferences[0].APIVersion, "argoproj.io/v1beta1")
	assert.Equal(t, cm.OwnerReferences[0].Kind, "ArgoCD")
	assert.Equal(t, cm.OwnerReferences[0].Name, "argocd")
	assert.Equal(t, cm.OwnerReferences[0].UID, uid)

	// verify when Kind is changed
	cm.OwnerReferences[0].Name = "test"

	changed, err = modifyOwnerReferenceIfNeeded(a, cm, r.Scheme)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, cm.OwnerReferences[0].APIVersion, "argoproj.io/v1beta1")
	assert.Equal(t, cm.OwnerReferences[0].Kind, "ArgoCD")
	assert.Equal(t, cm.OwnerReferences[0].Name, "argocd")
	assert.Equal(t, cm.OwnerReferences[0].UID, uid)

	// verify when UID is changed
	cm.OwnerReferences[0].UID = "test"

	changed, err = modifyOwnerReferenceIfNeeded(a, cm, r.Scheme)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, cm.OwnerReferences[0].APIVersion, "argoproj.io/v1beta1")
	assert.Equal(t, cm.OwnerReferences[0].Kind, "ArgoCD")
	assert.Equal(t, cm.OwnerReferences[0].Name, "argocd")
	assert.Equal(t, cm.OwnerReferences[0].UID, uid)
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withInstallationID(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.InstallationID = "test-id"
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// Test initial installationID
	err := r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	// Verify installationID is set as a top-level key
	assert.Equal(t, "test-id", cm.Data[common.ArgoCDKeyInstallationID])

	//Test updating installationID
	a.Spec.InstallationID = "test-id-2"
	err = r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	cm = &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	assert.Equal(t, "test-id-2", cm.Data[common.ArgoCDKeyInstallationID])

	// Test removing installationID
	a.Spec.InstallationID = ""
	err = r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	// Verify installationID was removed
	assert.NotContains(t, cm.Data, common.ArgoCDKeyInstallationID)
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withMultipleInstances(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	// Create first ArgoCD instance
	argocd1 := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Name = "argocd-1"
		a.Namespace = testNamespace
		a.Spec.InstallationID = "instance-1"
	})

	// Create second ArgoCD instance
	argocd2 := makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Name = "argocd-2"
		a.Namespace = testNamespace
		a.Spec.InstallationID = "instance-2"
	})

	resObjs := []client.Object{argocd1, argocd2}
	subresObjs := []client.Object{argocd1, argocd2}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// Test first instance
	err := r.reconcileArgoConfigMap(argocd1)
	assert.NoError(t, err)

	cm1 := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm1)
	assert.NoError(t, err)

	// Verify first instance's installationID
	assert.Equal(t, "instance-1", cm1.Data[common.ArgoCDKeyInstallationID])

	// Test second instance
	err = r.reconcileArgoConfigMap(argocd2)
	assert.NoError(t, err)

	cm2 := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm2)
	assert.NoError(t, err)

	// Verify second instance's installationID
	assert.Equal(t, "instance-2", cm2.Data[common.ArgoCDKeyInstallationID])
}

func TestReconcileArgoCD_RBACPolicyWithLogsPermissions(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	argocdCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: a.Namespace,
		},
		Data: map[string]string{},
	}

	resObjs := []client.Object{a, argocdCM}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	ctx := context.TODO()

	// Test 1: Verify no default policy is injected when no policy is specified
	err := r.reconcileRBAC(a)
	assert.NoError(t, err)

	createdCM := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      common.ArgoCDRBACConfigMapName,
		Namespace: a.Namespace,
	}, createdCM)
	assert.NoError(t, err)

	// Verify no default policy is injected
	policy := createdCM.Data[common.ArgoCDKeyRBACPolicyCSV]
	assert.Empty(t, policy)

	// Test 2: Verify default readonly role with logs permissions
	defaultReadonlyPolicy := `p, role:readonly, applications, get, */*, allow
p, role:readonly, logs, get, */*, allow`
	a.Spec.RBAC.Policy = &defaultReadonlyPolicy
	defaultPolicy := "role:readonly"
	a.Spec.RBAC.DefaultPolicy = &defaultPolicy

	err = r.reconcileRBAC(a)
	assert.NoError(t, err)

	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      common.ArgoCDRBACConfigMapName,
		Namespace: a.Namespace,
	}, createdCM)
	assert.NoError(t, err)

	// Verify readonly role has logs permissions
	policy = createdCM.Data[common.ArgoCDKeyRBACPolicyCSV]
	assert.Equal(t, defaultReadonlyPolicy, policy)
	assert.Equal(t, defaultPolicy, createdCM.Data[common.ArgoCDKeyRBACPolicyDefault])

	// Test 3: Verify default admin role with logs permissions
	defaultAdminPolicy := `p, role:admin, applications, *, */*, allow
p, role:admin, logs, get, */*, allow`
	a.Spec.RBAC.Policy = &defaultAdminPolicy
	defaultPolicy = "role:admin"
	a.Spec.RBAC.DefaultPolicy = &defaultPolicy

	err = r.reconcileRBAC(a)
	assert.NoError(t, err)

	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      common.ArgoCDRBACConfigMapName,
		Namespace: a.Namespace,
	}, createdCM)
	assert.NoError(t, err)

	// Verify admin role has logs permissions
	policy = createdCM.Data[common.ArgoCDKeyRBACPolicyCSV]
	assert.Equal(t, defaultAdminPolicy, policy)
	assert.Equal(t, defaultPolicy, createdCM.Data[common.ArgoCDKeyRBACPolicyDefault])

	// Test 4: Verify custom role without logs permissions
	customPolicy := `p, role:custom-app-viewer, applications, get, */*, allow`
	a.Spec.RBAC.Policy = &customPolicy
	defaultPolicy = "role:custom-app-viewer"
	a.Spec.RBAC.DefaultPolicy = &defaultPolicy

	err = r.reconcileRBAC(a)
	assert.NoError(t, err)

	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      common.ArgoCDRBACConfigMapName,
		Namespace: a.Namespace,
	}, createdCM)
	assert.NoError(t, err)

	// Verify custom role does not have logs permissions
	policy = createdCM.Data[common.ArgoCDKeyRBACPolicyCSV]
	assert.Equal(t, customPolicy, policy)
	assert.NotContains(t, policy, "p, role:custom-app-viewer, logs, get, */*, allow")

	// Test 5: Verify custom role with explicit logs permissions
	customPolicyWithLogs := `p, role:custom-app-viewer, applications, get, */*, allow
p, role:custom-app-viewer, logs, get, */*, allow`
	a.Spec.RBAC.Policy = &customPolicyWithLogs

	err = r.reconcileRBAC(a)
	assert.NoError(t, err)

	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      common.ArgoCDRBACConfigMapName,
		Namespace: a.Namespace,
	}, createdCM)
	assert.NoError(t, err)

	// Verify custom role has explicit logs permissions
	policy = createdCM.Data[common.ArgoCDKeyRBACPolicyCSV]
	assert.Equal(t, customPolicyWithLogs, policy)

	// Test 6: Verify global log viewer role
	globalLogViewerPolicy := `p, role:global-log-viewer, logs, get, */*, allow`
	a.Spec.RBAC.Policy = &globalLogViewerPolicy
	defaultPolicy = "role:global-log-viewer"
	a.Spec.RBAC.DefaultPolicy = &defaultPolicy

	err = r.reconcileRBAC(a)
	assert.NoError(t, err)

	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      common.ArgoCDRBACConfigMapName,
		Namespace: a.Namespace,
	}, createdCM)
	assert.NoError(t, err)

	// Verify global log viewer role and default policy
	policy = createdCM.Data[common.ArgoCDKeyRBACPolicyCSV]
	assert.Equal(t, globalLogViewerPolicy, policy)
	assert.Equal(t, defaultPolicy, createdCM.Data[common.ArgoCDKeyRBACPolicyDefault])

	// Test 7: Verify no default policy is restored when custom policy is removed
	// Clean up ConfigMap to simulate a reset state
	err = r.Client.Delete(ctx, createdCM)
	assert.NoError(t, err)

	a.Spec.RBAC.Policy = nil
	a.Spec.RBAC.DefaultPolicy = nil

	err = r.reconcileRBAC(a)
	assert.NoError(t, err)

	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      common.ArgoCDRBACConfigMapName,
		Namespace: a.Namespace,
	}, createdCM)
	assert.NoError(t, err)

	// Verify no default policy is restored
	policy = createdCM.Data[common.ArgoCDKeyRBACPolicyCSV]
	assert.Empty(t, policy)

	// Test 8: Verify server.rbac.log.enforce.enable is not set in argocd-cm
	cm := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: a.Namespace,
	}, cm)
	assert.NoError(t, err)

	_, exists := cm.Data["server.rbac.log.enforce.enable"]
	assert.False(t, exists, "server.rbac.log.enforce.enable should not exist in Argo CD v3.0+")
}

func TestReconcileArgoCD_RemovesLegacyLogEnforceFlag(t *testing.T) {
	// Setup fake ArgoCD instance and client
	cr := makeTestArgoCD() // helper to create ArgoCD CR
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: cr.Namespace,
		},
		Data: map[string]string{
			"server.rbac.log.enforce.enable": "true",
		},
	}

	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = argoproj.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cr, cm).Build()
	r := &ReconcileArgoCD{Client: client, Scheme: scheme}

	// Call reconcile
	err := r.reconcileArgoConfigMap(cr)
	assert.NoError(t, err)

	// Fetch updated ConfigMap
	updated := &corev1.ConfigMap{}
	err = client.Get(context.TODO(), types.NamespacedName{
		Name:      cm.Name,
		Namespace: cm.Namespace,
	}, updated)
	assert.NoError(t, err)

	_, exists := updated.Data["server.rbac.log.enforce.enable"]
	assert.False(t, exists, "expected deprecated key to be removed")
}

func TestReconcileArgoCD_reconcileArgoCmdParamsConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))

	tests := []struct {
		name           string
		cmdParams      map[string]string
		expectedValue  string
		expectKeyExist bool
	}{
		{
			name:           "No user-specified CmdParams",
			cmdParams:      nil,
			expectedValue:  "true",
			expectKeyExist: true,
		},
		{
			name: "User-specified CmdParams without health.persist",
			cmdParams: map[string]string{
				"some.other.param": "value",
			},
			expectedValue:  "true",
			expectKeyExist: true,
		},
		{
			name: "User overrides health.persist to false",
			cmdParams: map[string]string{
				"controller.resource.health.persist": "false",
			},
			expectedValue:  "false",
			expectKeyExist: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := makeTestArgoCD(func(a *argoproj.ArgoCD) {
				a.Spec.CmdParams = test.cmdParams
			})

			resObjs := []client.Object{a}
			subresObjs := []client.Object{a}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoproj.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)

			err := r.reconcileArgoCmdParamsConfigMap(a)
			assert.NoError(t, err)

			cm := &corev1.ConfigMap{}
			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      common.ArgoCDCmdParamsConfigMapName,
				Namespace: testNamespace,
			}, cm)
			assert.NoError(t, err)

			val, exists := cm.Data["controller.resource.health.persist"]
			assert.Equal(t, test.expectKeyExist, exists, "Expected key existence mismatesth")
			assert.Equal(t, test.expectedValue, val, "Expected value mismatch")
		})
	}
}
