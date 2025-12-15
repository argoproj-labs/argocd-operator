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

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

// Test constants
const (
	testCompName   = "principal"
	testNamespace  = "argocd"
	testArgoCDName = "argocd"
)

// Test helper functions
type argoCDOpt func(*argoproj.ClusterArgoCD)

func makeTestClusterArgoCD(opts ...argoCDOpt) *argoproj.ClusterArgoCD {
	a := &argoproj.ClusterArgoCD{
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

func withPrincipalEnabled(enabled bool) argoCDOpt {
	return func(a *argoproj.ClusterArgoCD) {
		if a.Spec.ArgoCDAgent == nil {
			a.Spec.ArgoCDAgent = &argoproj.ArgoCDAgentSpec{}
		}
		if a.Spec.ArgoCDAgent.Principal == nil {
			a.Spec.ArgoCDAgent.Principal = &argoproj.PrincipalSpec{}
		}
		a.Spec.ArgoCDAgent.Principal.Enabled = &enabled
	}
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

// TestReconcilePrincipalServiceAccount tests

func TestReconcilePrincipalServiceAccount_ServiceAccountDoesNotExist_PrincipalDisabled(t *testing.T) {
	// Test case: ServiceAccount doesn't exist and principal is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestClusterArgoCD(withPrincipalEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePrincipalServiceAccount(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was not created
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalServiceAccount_ServiceAccountDoesNotExist_PrincipalEnabled(t *testing.T) {
	// Test case: ServiceAccount doesn't exist and principal is enabled
	// Expected behavior: Should create the ServiceAccount

	cr := makeTestClusterArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePrincipalServiceAccount(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was created
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.NoError(t, err)

	// Verify ServiceAccount has expected metadata
	assert.Equal(t, generateAgentResourceName(cr.Name, testCompName), retrievedSA.Name)
	assert.Equal(t, cr.Namespace, retrievedSA.Namespace)
	assert.Equal(t, buildLabelsForAgentPrincipal(cr.Name, testCompName), retrievedSA.Labels)

	// Verify owner reference is set
	assert.Len(t, retrievedSA.OwnerReferences, 1)
	assert.Equal(t, cr.Name, retrievedSA.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", retrievedSA.OwnerReferences[0].Kind)
}

func TestReconcilePrincipalServiceAccount_ServiceAccountExists_PrincipalDisabled(t *testing.T) {
	// Test case: ServiceAccount exists and principal is disabled
	// Expected behavior: Should delete the ServiceAccount

	cr := makeTestClusterArgoCD(withPrincipalEnabled(false))

	// Create existing ServiceAccount
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePrincipalServiceAccount(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was deleted
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalServiceAccount_ServiceAccountExists_PrincipalEnabled(t *testing.T) {
	// Test case: ServiceAccount exists and principal is enabled
	// Expected behavior: Should return the existing ServiceAccount without modification

	cr := makeTestClusterArgoCD(withPrincipalEnabled(true))

	// Create existing ServiceAccount
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePrincipalServiceAccount(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount still exists
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.NoError(t, err)
	assert.Equal(t, generateAgentResourceName(cr.Name, testCompName), retrievedSA.Name)
	assert.Equal(t, cr.Namespace, retrievedSA.Namespace)
	assert.Equal(t, buildLabelsForAgentPrincipal(cr.Name, testCompName), retrievedSA.Labels)
}

func TestReconcilePrincipalServiceAccount_ServiceAccountExists_PrincipalNotSet(t *testing.T) {
	// Test case: ServiceAccount exists but principal is not set (nil)
	// Expected behavior: Should delete the ServiceAccount

	cr := makeTestClusterArgoCD() // No principal configuration

	// Create existing ServiceAccount
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePrincipalServiceAccount(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was deleted (since principal is not enabled by default)
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalServiceAccount_ServiceAccountDoesNotExist_PrincipalNotSet(t *testing.T) {
	// Test case: ServiceAccount doesn't exist and principal is not set (nil)
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestClusterArgoCD() // No principal configuration

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePrincipalServiceAccount(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was not created
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalServiceAccount_ServiceAccountDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: ServiceAccount doesn't exist and agent is not set (nil)
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestClusterArgoCD() // No agent configuration

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePrincipalServiceAccount(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was not created
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalServiceAccount_ServiceAccountExists_AgentNotSet(t *testing.T) {
	// Test case: ServiceAccount exists but agent is not set (nil)
	// Expected behavior: Should delete the ServiceAccount

	cr := makeTestClusterArgoCD() // No agent configuration

	// Create existing ServiceAccount
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePrincipalServiceAccount(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was deleted (since agent is not set)
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}
