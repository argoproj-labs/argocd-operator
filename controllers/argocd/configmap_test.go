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
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v2"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

var _ reconcile.Reconciler = &ReconcileArgoCD{}

func TestReconcileArgoCD_reconcileTLSCerts(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(initialCerts(t, "root-ca.example.com"))
	r := makeTestReconciler(t, a)

	assert.NilError(t, r.reconcileTLSCerts(a))

	configMap := &corev1.ConfigMap{}
	assert.NilError(t, r.Client.Get(
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

func TestReconcileArgoCD_reconcileTLSCerts_withUpdate(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	assert.NilError(t, r.reconcileTLSCerts(a))

	a = makeTestArgoCD(initialCerts(t, "testing.example.com"))
	assert.NilError(t, r.reconcileTLSCerts(a))

	configMap := &corev1.ConfigMap{}
	assert.NilError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      common.ArgoCDTLSCertsConfigMapName,
			Namespace: a.Namespace,
		},
		configMap))

	want := []string{"testing.example.com"}
	if k := stringMapKeys(configMap.Data); !reflect.DeepEqual(want, k) {
		t.Fatalf("got %#v, want %#v\n", k, want)
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoConfigMap(a)
	assert.NilError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NilError(t, err)

	want := map[string]string{
		"application.instanceLabelKey": "mycompany.com/appname",
		"admin.enabled":                "true",
		"configManagementPlugins":      "",
		"dex.config":                   "",
		"ga.anonymizeusers":            "false",
		"ga.trackingid":                "",
		"help.chatText":                "Chat now!",
		"help.chatUrl":                 "https://mycorp.slack.com/argo-cd",
		"kustomize.buildOptions":       "",
		"oidc.config":                  "",
		"repositories":                 "",
		"repository.credentials":       "",
		"resource.inclusions":          "",
		"resource.exclusions":          "",
		"statusbadge.enabled":          "false",
		"url":                          "https://argocd-server",
		"users.anonymous.enabled":      "false",
	}

	if diff := cmp.Diff(want, cm.Data); diff != "" {
		t.Fatalf("reconcileArgoConfigMap failed:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileEmptyArgoConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	// An empty Argo CD Configmap
	emptyArgoConfigmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDConfigMapName,
			Namespace: a.Namespace,
		},
	}

	err := r.Client.Create(context.TODO(), emptyArgoConfigmap)
	assert.NilError(t, err)

	err = r.reconcileArgoConfigMap(a)
	assert.NilError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NilError(t, err)
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
	r := makeTestReconciler(t, a, cm)

	err := r.reconcileArgoConfigMap(a)
	assert.NilError(t, err)

	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NilError(t, err)

	if got := cm.Data[common.ArgoCDKeyRepositoryCredentials]; got != a.Spec.RepositoryCredentials {
		t.Fatalf("reconcileArgoConfigMap failed: got %s, want %s", got, a.Spec.RepositoryCredentials)
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withDisableAdmin(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.DisableAdmin = true
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoConfigMap(a)
	assert.NilError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NilError(t, err)

	if c := cm.Data["admin.enabled"]; c != "false" {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, "false")
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withDexConnector(t *testing.T) {
	restoreEnv(t)
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.Dex.OpenShiftOAuth = true
	})
	sa := &corev1.ServiceAccount{
		TypeMeta:   metav1.TypeMeta{Kind: "ServiceAccount", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "argocd-argocd-dex-server", Namespace: "argocd"},
		Secrets: []corev1.ObjectReference{{
			Name: "token",
		}},
	}

	secret := argoutil.NewSecretWithName(metav1.ObjectMeta{Name: "token", Namespace: "argocd"}, "token")
	r := makeTestReconciler(t, a, sa, secret)
	err := r.reconcileArgoConfigMap(a)
	assert.NilError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NilError(t, err)

	dex, ok := cm.Data["dex.config"]
	if !ok {
		t.Fatal("reconcileArgoConfigMap with dex failed")
	}

	m := make(map[string]interface{})
	err = yaml.Unmarshal([]byte(dex), &m)
	assert.NilError(t, err, fmt.Sprintf("failed to unmarshal %s", dex))

	connectors, ok := m["connectors"]
	if !ok {
		t.Fatal("no connectors found in dex.config")
	}
	dexConnector := connectors.([]interface{})[0].(map[interface{}]interface{})
	config := dexConnector["config"]
	assert.Equal(t, config.(map[interface{}]interface{})["clientID"], "system:serviceaccount:argocd:argocd-argocd-dex-server")
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withDexDisabled(t *testing.T) {
	restoreEnv(t)
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	os.Setenv("DISABLE_DEX", "true")
	err := r.reconcileArgoConfigMap(a)
	assert.NilError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NilError(t, err)

	if c, ok := cm.Data["dex.config"]; ok {
		t.Fatalf("reconcileArgoConfigMap failed, dex.config = %q", c)
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withKustomizeVersions(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		kv := argoprojv1alpha1.KustomizeVersionSpec{
			Version: "v4.1.0",
			Path:    "/path/to/kustomize-4.1",
		}
		var kvs []argoprojv1alpha1.KustomizeVersionSpec
		kvs = append(kvs, kv)
		a.Spec.KustomizeVersions = kvs
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoConfigMap(a)
	assert.NilError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NilError(t, err)

	if diff := cmp.Diff(cm.Data["kustomize.version.v4.1.0"], "/path/to/kustomize-4.1"); diff != "" {
		t.Fatalf("failed to reconcile configmap:\n%s", diff)
	}
}

func TestReconcileArgoCD_reconcileGPGKeysConfigMap(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.DisableAdmin = true
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileGPGKeysConfigMap(a)
	assert.NilError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDGPGKeysConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NilError(t, err)
	// Currently the gpg keys configmap is empty
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withResourceInclusions(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	customizations := "testing: testing"
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.ResourceInclusions = customizations
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoConfigMap(a)
	assert.NilError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NilError(t, err)

	if c := cm.Data["resource.inclusions"]; c != customizations {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, customizations)
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withResourceCustomizations(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	customizations := "testing: testing"
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.ResourceCustomizations = customizations
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoConfigMap(a)
	assert.NilError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assert.NilError(t, err)

	if c := cm.Data["resource.customizations"]; c != customizations {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, customizations)
	}
}
