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
	"reflect"
	"testing"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var _ reconcile.Reconciler = &ReconcileArgoCD{}

func TestReconcileArgoCD_reconcileTLSCerts(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD(initialCerts(t, "root-ca.example.com"))
	r := makeTestReconciler(t, a)

	assertNoError(t, r.reconcileTLSCerts(a))

	configMap := &corev1.ConfigMap{}
	assertNoError(t, r.client.Get(
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
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	assertNoError(t, r.reconcileTLSCerts(a))

	a = makeTestArgoCD(initialCerts(t, "testing.example.com"))
	assertNoError(t, r.reconcileTLSCerts(a))

	configMap := &corev1.ConfigMap{}
	assertNoError(t, r.client.Get(
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
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoConfigMap(a)
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assertNoError(t, err)

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

func TestReconcileArgoCD_reconcileArgoConfigMap_withDisableAdmin(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.DisableAdmin = true
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoConfigMap(a)
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assertNoError(t, err)

	if c := cm.Data["admin.enabled"]; c != "false" {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, "false")
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withResourceInclusions(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	customizations := "testing: testing"
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.ResourceInclusions = customizations
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoConfigMap(a)
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assertNoError(t, err)

	if c := cm.Data["resource.inclusions"]; c != customizations {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, customizations)
	}
}

func TestReconcileArgoCD_reconcileArgoConfigMap_withResourceCustomizations(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	customizations := "testing: testing"
	a := makeTestArgoCD(func(a *argoprojv1alpha1.ArgoCD) {
		a.Spec.ResourceCustomizations = customizations
	})
	r := makeTestReconciler(t, a)

	err := r.reconcileArgoConfigMap(a)
	assertNoError(t, err)

	cm := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      common.ArgoCDConfigMapName,
		Namespace: testNamespace,
	}, cm)
	assertNoError(t, err)

	if c := cm.Data["resource.customizations"]; c != customizations {
		t.Fatalf("reconcileArgoConfigMap failed got %q, want %q", c, customizations)
	}
}
