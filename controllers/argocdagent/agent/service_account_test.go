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

package agent

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
	testAgentCompName = "agent"
	testNamespace     = "argocd"
	testArgoCDName    = "argocd"
)

// Test helper functions
type argoCDOpt func(*argoproj.ArgoCD)

func makeTestArgoCD(opts ...argoCDOpt) *argoproj.ArgoCD {
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

func withAgentEnabled(enabled bool) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		if a.Spec.ArgoCDAgent == nil {
			a.Spec.ArgoCDAgent = &argoproj.ArgoCDAgentSpec{}
		}
		if a.Spec.ArgoCDAgent.Agent == nil {
			a.Spec.ArgoCDAgent.Agent = &argoproj.AgentSpec{}
		}
		a.Spec.ArgoCDAgent.Agent.Enabled = &enabled
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

// TestReconcileAgentServiceAccount tests

func TestReconcileAgentServiceAccount_ServiceAccountDoesNotExist_AgentDisabled(t *testing.T) {
	// Test case: ServiceAccount doesn't exist and agent is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withAgentEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcileAgentServiceAccount(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was not created
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentServiceAccount_ServiceAccountDoesNotExist_AgentEnabled(t *testing.T) {
	// Test case: ServiceAccount doesn't exist and agent is enabled
	// Expected behavior: Should create the ServiceAccount

	cr := makeTestArgoCD(withAgentEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcileAgentServiceAccount(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was created
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.NoError(t, err)

	// Verify ServiceAccount has expected metadata
	assert.Equal(t, generateAgentResourceName(cr.Name, testAgentCompName), retrievedSA.Name)
	assert.Equal(t, cr.Namespace, retrievedSA.Namespace)
	assert.Equal(t, buildLabelsForAgent(cr.Name, testAgentCompName), retrievedSA.Labels)

	// Verify owner reference is set
	assert.Len(t, retrievedSA.OwnerReferences, 1)
	assert.Equal(t, cr.Name, retrievedSA.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", retrievedSA.OwnerReferences[0].Kind)
}

func TestReconcileAgentServiceAccount_ServiceAccountExists_AgentDisabled(t *testing.T) {
	// Test case: ServiceAccount exists and agent is disabled
	// Expected behavior: Should delete the ServiceAccount

	cr := makeTestArgoCD(withAgentEnabled(false))

	// Create existing ServiceAccount
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcileAgentServiceAccount(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was deleted
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentServiceAccount_ServiceAccountExists_AgentEnabled(t *testing.T) {
	// Test case: ServiceAccount exists and agent is enabled
	// Expected behavior: Should return the existing ServiceAccount without modification

	cr := makeTestArgoCD(withAgentEnabled(true))

	// Create existing ServiceAccount
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcileAgentServiceAccount(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount still exists
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.NoError(t, err)
	assert.Equal(t, generateAgentResourceName(cr.Name, testAgentCompName), retrievedSA.Name)
	assert.Equal(t, cr.Namespace, retrievedSA.Namespace)
	assert.Equal(t, buildLabelsForAgent(cr.Name, testAgentCompName), retrievedSA.Labels)
}

func TestReconcileAgentServiceAccount_ServiceAccountExists_AgentNotSet(t *testing.T) {
	// Test case: ServiceAccount exists but agent is not set (nil)
	// Expected behavior: Should delete the ServiceAccount

	cr := makeTestArgoCD() // No agent configuration

	// Create existing ServiceAccount
	existingSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
	}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcileAgentServiceAccount(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was deleted (since agent is not enabled by default)
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentServiceAccount_ServiceAccountDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: ServiceAccount doesn't exist and agent is not set (nil)
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD() // No agent configuration

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcileAgentServiceAccount(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	// Verify ServiceAccount was not created
	retrievedSA := &corev1.ServiceAccount{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}
