// Copyright 2025 ArgoCD Operator Developers
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

package argocdagent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

const (
	testNamespace  = "argocd"
	testArgoCDName = "argocd"
	testCompName   = "principal"
)

func init() {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))
}

func makeTestArgoCD(opts ...func(*argoproj.ArgoCD)) *argoproj.ArgoCD {
	a := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func makeTestReconcilerScheme() *runtime.Scheme {
	s := scheme.Scheme
	_ = argoproj.AddToScheme(s)
	return s
}

func makeTestReconcilerClient(sch *runtime.Scheme, resObjs []client.Object) client.Client {
	client := fake.NewClientBuilder().WithScheme(sch)
	if len(resObjs) > 0 {
		client = client.WithObjects(resObjs...)
	}
	return client.Build()
}

func withPrincipalEnabled(enabled bool) func(*argoproj.ArgoCD) {
	return func(a *argoproj.ArgoCD) {
		if a.Spec.ArgoCDAgent == nil {
			a.Spec.ArgoCDAgent = &argoproj.ArgoCDAgentSpec{}
		}
		if a.Spec.ArgoCDAgent.Principal == nil {
			a.Spec.ArgoCDAgent.Principal = &argoproj.PrincipalSpec{}
		}
		a.Spec.ArgoCDAgent.Principal.Enabled = &enabled
	}
}

func TestReconcilePrincipalConfigMap_ConfigMapDoesNotExist_PrincipalDisabled(t *testing.T) {
	// Test case: ConfigMap doesn't exist and principal is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalConfigMap(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify ConfigMap was not created
	cm := &corev1.ConfigMap{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      cr.Name + "-agent-params",
		Namespace: cr.Namespace,
	}, cm)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalConfigMap_ConfigMapDoesNotExist_PrincipalEnabled(t *testing.T) {
	// Test case: ConfigMap doesn't exist and principal is enabled
	// Expected behavior: Should create the ConfigMap with expected data

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalConfigMap(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify ConfigMap was created
	cm := &corev1.ConfigMap{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      cr.Name + "-agent-params",
		Namespace: cr.Namespace,
	}, cm)
	assert.NoError(t, err)

	// Verify ConfigMap has expected metadata
	assert.Equal(t, cr.Name+"-agent-params", cm.Name)
	assert.Equal(t, cr.Namespace, cm.Namespace)
	assert.Equal(t, buildLabelsForAgentPrincipal(cr.Name), cm.Labels)

	// Verify ConfigMap has expected data keys (sample check)
	expectedData := buildData(cl, cr)
	assert.Equal(t, expectedData, cm.Data)

	// Verify owner reference is set
	assert.Len(t, cm.OwnerReferences, 1)
	assert.Equal(t, cr.Name, cm.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", cm.OwnerReferences[0].Kind)
}

func TestReconcilePrincipalConfigMap_ConfigMapExists_PrincipalDisabled(t *testing.T) {
	// Test case: ConfigMap exists and principal is disabled
	// Expected behavior: Should delete the ConfigMap

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, []client.Object{cr})

	// Create existing ConfigMap
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-agent-params",
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
		Data: buildData(cl, cr),
	}

	// Recreate client with all objects
	resObjs := []client.Object{cr, existingCM}
	cl = makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalConfigMap(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify ConfigMap was deleted
	cm := &corev1.ConfigMap{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      cr.Name + "-agent-params",
		Namespace: cr.Namespace,
	}, cm)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalConfigMap_ConfigMapExists_PrincipalEnabled_SameData(t *testing.T) {
	// Test case: ConfigMap exists, principal is enabled, and data is the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, []client.Object{cr})

	expectedData := buildData(cl, cr)
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-agent-params",
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
		Data: expectedData,
	}

	// Recreate client with all objects
	resObjs := []client.Object{cr, existingCM}
	cl = makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalConfigMap(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify ConfigMap still exists with same data
	cm := &corev1.ConfigMap{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      cr.Name + "-agent-params",
		Namespace: cr.Namespace,
	}, cm)
	assert.NoError(t, err)
	assert.Equal(t, expectedData, cm.Data)
}

func TestReconcilePrincipalConfigMap_ConfigMapExists_PrincipalEnabled_DifferentData(t *testing.T) {
	// Test case: ConfigMap exists, principal is enabled, but data is different
	// Expected behavior: Should update the ConfigMap with new data

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	// Create existing ConfigMap with different data
	oldData := map[string]string{
		"old-key": "old-value",
	}
	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-agent-params",
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
		Data: oldData,
	}

	resObjs := []client.Object{cr, existingCM}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalConfigMap(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify ConfigMap was updated with new data
	cm := &corev1.ConfigMap{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      cr.Name + "-agent-params",
		Namespace: cr.Namespace,
	}, cm)
	assert.NoError(t, err)

	expectedData := buildData(cl, cr)
	assert.Equal(t, expectedData, cm.Data)
	assert.NotEqual(t, oldData, cm.Data)
}

func TestReconcilePrincipalConfigMap_ConfigMapExists_PrincipalNotSet(t *testing.T) {
	// Test case: ConfigMap exists but Principal is not set (nil)
	// Expected behavior: Should delete the ConfigMap since principal is effectively disabled

	cr := makeTestArgoCD() // No principal configuration

	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, []client.Object{cr})

	existingCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-agent-params",
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name),
		},
		Data: buildData(cl, cr),
	}

	// Recreate client with all objects
	resObjs := []client.Object{cr, existingCM}
	cl = makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalConfigMap(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify ConfigMap was deleted
	cm := &corev1.ConfigMap{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      cr.Name + "-agent-params",
		Namespace: cr.Namespace,
	}, cm)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalConfigMap_ConfigMapDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: ConfigMap doesn't exist and ArgoCDAgent is not set (nil)
	// Expected behavior: Should do nothing since principal is effectively disabled

	cr := makeTestArgoCD() // No agent configuration

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcilePrincipalConfigMap(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify ConfigMap was not created
	cm := &corev1.ConfigMap{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      cr.Name + "-agent-params",
		Namespace: cr.Namespace,
	}, cm)
	assert.True(t, errors.IsNotFound(err))
}
