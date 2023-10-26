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
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	testPEM := generateEncodedPEM(t, "example.com")

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
		"configManagementPlugins":            "",
		"dex.config":                         "",
		"ga.anonymizeusers":                  "false",
		"ga.trackingid":                      "",
		"help.chatText":                      "",
		"help.chatUrl":                       "",
		"kustomize.buildOptions":             "",
		"oidc.config":                        "",
		"repositories":                       "",
		"repository.credentials":             "",
		"resource.inclusions":                "",
		"resource.exclusions":                "",
		"statusbadge.enabled":                "false",
		"url":                                "https://argocd-server",
		"users.anonymous.enabled":            "false",
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

func TestReconcileArgoCDCM_withRepoCredentials(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	a.Spec.RepositoryCredentials = `
- url: https://github.com/test/gitops.git
  passwordSecret:
    name: test
    key: password
  usernameSecret:
    name: test
    key: username`

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: testNamespace,
		},
		Data: map[string]string{
			"application.instanceLabelKey": "mycompany.com/appname",
			"admin.enabled":                "true",
		},
	}

	resObjs := []client.Object{a, cm}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileArgoConfigMap(a)
	assert.NoError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NoError(t, err)

	if got := cm.Data[common.ArgoCDKeyRepositoryCredentials]; got != a.Spec.RepositoryCredentials {
		t.Fatalf("reconcileArgoConfigMap failed: got %s, want %s", got, a.Spec.RepositoryCredentials)
	}
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
	desiredCM["resource.customizations.actions.actionsFoo_actionsFoo"] = "actionsFoo"
	desiredCM["resource.customizations.actions.actionsBar_actionsBar"] = "actionsBar"
	desiredCM["resource.customizations.ignoreDifferences.all"] = desiredIgnoreDifferenceCustomization
	desiredCM["resource.customizations.ignoreDifferences.ignoreDiffBar_ignoreDiffBar"] = desiredIgnoreDifferenceCustomization

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
}
