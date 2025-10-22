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
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Test helper functions

func makeTestServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName + "-agent-principal",
			Namespace: testNamespace,
		},
	}
}

// Tests for ReconcileAgentRoleBinding

func TestReconcileAgentRoleBinding_RoleBindingDoesNotExist_AgentDisabled(t *testing.T) {
	// Test case: RoleBinding doesn't exist and agent is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withAgentEnabled(false))
	sa := makeTestServiceAccount()

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify RoleBinding was not created
	roleBinding := &v1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, roleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentRoleBinding_RoleBindingDoesNotExist_AgentEnabled(t *testing.T) {
	// Test case: RoleBinding doesn't exist and agent is enabled
	// Expected behavior: Should create the RoleBinding with expected subjects and roleRef

	cr := makeTestArgoCD(withAgentEnabled(true))
	sa := makeTestServiceAccount()

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify RoleBinding was created
	roleBinding := &v1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, roleBinding)
	assert.NoError(t, err)

	// Verify RoleBinding has expected metadata
	assert.Equal(t, generateAgentResourceName(cr.Name, testAgentCompName), roleBinding.Name)
	assert.Equal(t, cr.Namespace, roleBinding.Namespace)
	assert.Equal(t, buildLabelsForAgent(cr.Name, testAgentCompName), roleBinding.Labels)

	// Verify RoleBinding has expected subjects
	expectedSubjects := buildSubjects(sa, cr)
	assert.Equal(t, expectedSubjects, roleBinding.Subjects)

	// Verify RoleBinding has expected roleRef
	expectedRoleRef := buildRoleRef(generateAgentResourceName(cr.Name, testAgentCompName), "Role")
	assert.Equal(t, expectedRoleRef, roleBinding.RoleRef)

	// Verify owner reference is set
	assert.Len(t, roleBinding.OwnerReferences, 1)
	assert.Equal(t, cr.Name, roleBinding.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", roleBinding.OwnerReferences[0].Kind)
}

func TestReconcileAgentRoleBinding_RoleBindingExists_AgentDisabled(t *testing.T) {
	// Test case: RoleBinding exists and agent is disabled
	// Expected behavior: Should delete the RoleBinding

	cr := makeTestArgoCD(withAgentEnabled(false))
	sa := makeTestServiceAccount()

	// Create existing RoleBinding
	existingRoleBinding := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Subjects: buildSubjects(sa, cr),
		RoleRef:  buildRoleRef(generateAgentResourceName(cr.Name, testAgentCompName), "Role"),
	}

	resObjs := []client.Object{cr, sa, existingRoleBinding}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify RoleBinding was deleted
	roleBinding := &v1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, roleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentRoleBinding_RoleBindingExists_AgentEnabled_SameConfiguration(t *testing.T) {
	// Test case: RoleBinding exists, agent is enabled, and configuration is the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withAgentEnabled(true))
	sa := makeTestServiceAccount()

	expectedSubjects := buildSubjects(sa, cr)
	expectedRoleRef := buildRoleRef(generateAgentResourceName(cr.Name, testAgentCompName), "Role")

	// Create existing RoleBinding with correct configuration
	existingRoleBinding := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Subjects: expectedSubjects,
		RoleRef:  expectedRoleRef,
	}

	resObjs := []client.Object{cr, sa, existingRoleBinding}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify RoleBinding still exists with same configuration
	roleBinding := &v1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, roleBinding)
	assert.NoError(t, err)
	assert.Equal(t, expectedSubjects, roleBinding.Subjects)
	assert.Equal(t, expectedRoleRef, roleBinding.RoleRef)
}

func TestReconcileAgentRoleBinding_RoleBindingExists_AgentEnabled_DifferentSubjects(t *testing.T) {
	// Test case: RoleBinding exists, agent is enabled, but subjects are different
	// Expected behavior: Should update the RoleBinding with new subjects

	cr := makeTestArgoCD(withAgentEnabled(true))
	sa := makeTestServiceAccount()

	// Create existing RoleBinding with different subjects
	oldSubjects := []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      "different-sa",
			Namespace: cr.Namespace,
		},
	}
	expectedRoleRef := buildRoleRef(generateAgentResourceName(cr.Name, testAgentCompName), "Role")

	existingRoleBinding := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Subjects: oldSubjects,
		RoleRef:  expectedRoleRef,
	}

	resObjs := []client.Object{cr, sa, existingRoleBinding}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify RoleBinding was updated with expected subjects
	roleBinding := &v1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, roleBinding)
	assert.NoError(t, err)

	expectedSubjects := buildSubjects(sa, cr)
	assert.Equal(t, expectedSubjects, roleBinding.Subjects)
	assert.NotEqual(t, oldSubjects, roleBinding.Subjects)
}

func TestReconcileAgentRoleBinding_RoleBindingExists_AgentEnabled_DifferentRoleRef(t *testing.T) {
	// Test case: RoleBinding exists, agent is enabled, but roleRef is different
	// Expected behavior: Should update the RoleBinding with new roleRef

	cr := makeTestArgoCD(withAgentEnabled(true))
	sa := makeTestServiceAccount()

	expectedSubjects := buildSubjects(sa, cr)

	// Create existing RoleBinding with different roleRef
	oldRoleRef := v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "Role",
		Name:     "different-role",
	}

	existingRoleBinding := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Subjects: expectedSubjects,
		RoleRef:  oldRoleRef,
	}

	resObjs := []client.Object{cr, sa, existingRoleBinding}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify RoleBinding was updated with expected roleRef
	roleBinding := &v1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, roleBinding)
	assert.NoError(t, err)

	expectedRoleRef := buildRoleRef(generateAgentResourceName(cr.Name, testAgentCompName), "Role")
	assert.Equal(t, expectedRoleRef, roleBinding.RoleRef)
	assert.NotEqual(t, oldRoleRef, roleBinding.RoleRef)
}

func TestReconcileAgentRoleBinding_RoleBindingExists_AgentNotSet(t *testing.T) {
	// Test case: RoleBinding exists but agent is not set (nil)
	// Expected behavior: Should delete the RoleBinding

	cr := makeTestArgoCD() // No agent configuration
	sa := makeTestServiceAccount()

	// Create existing RoleBinding
	existingRoleBinding := &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Subjects: buildSubjects(sa, cr),
		RoleRef:  buildRoleRef(generateAgentResourceName(cr.Name, testAgentCompName), "Role"),
	}

	resObjs := []client.Object{cr, sa, existingRoleBinding}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify RoleBinding was deleted (since agent is not enabled by default)
	roleBinding := &v1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, roleBinding)
	assert.True(t, errors.IsNotFound(err))
}

// Tests for ReconcileAgentClusterRoleBinding

func TestReconcileAgentClusterRoleBinding_ClusterRoleBindingDoesNotExist_AgentDisabled(t *testing.T) {
	// Test case: ClusterRoleBinding doesn't exist and agent is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withAgentEnabled(false))
	sa := makeTestServiceAccount()

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentClusterRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify ClusterRoleBinding was not created
	clusterRoleBinding := &v1.ClusterRoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, clusterRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentClusterRoleBinding_ClusterRoleBindingDoesNotExist_AgentEnabled(t *testing.T) {
	// Test case: ClusterRoleBinding doesn't exist and agent is enabled
	// Expected behavior: Should create the ClusterRoleBinding with expected subjects and roleRef

	cr := makeTestArgoCD(withAgentEnabled(true))
	sa := makeTestServiceAccount()

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentClusterRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify ClusterRoleBinding was created
	clusterRoleBinding := &v1.ClusterRoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, clusterRoleBinding)
	assert.NoError(t, err)

	// Verify ClusterRoleBinding has expected metadata
	assert.Equal(t, generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName), clusterRoleBinding.Name)
	assert.Equal(t, buildLabelsForAgent(cr.Name, testAgentCompName), clusterRoleBinding.Labels)

	// Verify ClusterRoleBinding has expected subjects
	expectedSubjects := buildSubjects(sa, cr)
	assert.Equal(t, expectedSubjects, clusterRoleBinding.Subjects)

	// Verify ClusterRoleBinding has expected roleRef
	expectedRoleRef := buildRoleRef(generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName), "ClusterRole")
	assert.Equal(t, expectedRoleRef, clusterRoleBinding.RoleRef)

	// Verify no owner reference is set for ClusterRoleBinding (as expected from the code)
	assert.Len(t, clusterRoleBinding.OwnerReferences, 0)
}

func TestReconcileAgentClusterRoleBinding_ClusterRoleBindingExists_AgentDisabled(t *testing.T) {
	// Test case: ClusterRoleBinding exists and agent is disabled
	// Expected behavior: Should delete the ClusterRoleBinding

	cr := makeTestArgoCD(withAgentEnabled(false))
	sa := makeTestServiceAccount()

	// Create existing ClusterRoleBinding
	existingClusterRoleBinding := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
			Labels: buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Subjects: buildSubjects(sa, cr),
		RoleRef:  buildRoleRef(generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName), "ClusterRole"),
	}

	resObjs := []client.Object{cr, sa, existingClusterRoleBinding}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentClusterRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify ClusterRoleBinding was deleted
	clusterRoleBinding := &v1.ClusterRoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, clusterRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentClusterRoleBinding_ClusterRoleBindingExists_AgentEnabled_SameConfiguration(t *testing.T) {
	// Test case: ClusterRoleBinding exists, agent is enabled, and configuration is the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withAgentEnabled(true))
	sa := makeTestServiceAccount()

	expectedSubjects := buildSubjects(sa, cr)
	expectedRoleRef := buildRoleRef(generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName), "ClusterRole")

	// Create existing ClusterRoleBinding with correct configuration
	existingClusterRoleBinding := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
			Labels: buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Subjects: expectedSubjects,
		RoleRef:  expectedRoleRef,
	}

	resObjs := []client.Object{cr, sa, existingClusterRoleBinding}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentClusterRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify ClusterRoleBinding still exists with same configuration
	clusterRoleBinding := &v1.ClusterRoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, clusterRoleBinding)
	assert.NoError(t, err)
	assert.Equal(t, expectedSubjects, clusterRoleBinding.Subjects)
	assert.Equal(t, expectedRoleRef, clusterRoleBinding.RoleRef)
}

func TestReconcileAgentClusterRoleBinding_ClusterRoleBindingExists_AgentEnabled_DifferentSubjects(t *testing.T) {
	// Test case: ClusterRoleBinding exists, agent is enabled, but subjects are different
	// Expected behavior: Should update the ClusterRoleBinding with new subjects

	cr := makeTestArgoCD(withAgentEnabled(true))
	sa := makeTestServiceAccount()

	// Create existing ClusterRoleBinding with different subjects
	oldSubjects := []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      "different-sa",
			Namespace: cr.Namespace,
		},
	}
	expectedRoleRef := buildRoleRef(generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName), "ClusterRole")

	existingClusterRoleBinding := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
			Labels: buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Subjects: oldSubjects,
		RoleRef:  expectedRoleRef,
	}

	resObjs := []client.Object{cr, sa, existingClusterRoleBinding}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentClusterRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify ClusterRoleBinding was updated with expected subjects
	clusterRoleBinding := &v1.ClusterRoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, clusterRoleBinding)
	assert.NoError(t, err)

	expectedSubjects := buildSubjects(sa, cr)
	assert.Equal(t, expectedSubjects, clusterRoleBinding.Subjects)
	assert.NotEqual(t, oldSubjects, clusterRoleBinding.Subjects)
}

func TestReconcileAgentClusterRoleBinding_ClusterRoleBindingExists_AgentEnabled_DifferentRoleRef(t *testing.T) {
	// Test case: ClusterRoleBinding exists, agent is enabled, but roleRef is different
	// Expected behavior: Should update the ClusterRoleBinding with new roleRef

	cr := makeTestArgoCD(withAgentEnabled(true))
	sa := makeTestServiceAccount()

	expectedSubjects := buildSubjects(sa, cr)

	// Create existing ClusterRoleBinding with different roleRef
	oldRoleRef := v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "ClusterRole",
		Name:     "different-cluster-role",
	}

	existingClusterRoleBinding := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
			Labels: buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Subjects: expectedSubjects,
		RoleRef:  oldRoleRef,
	}

	resObjs := []client.Object{cr, sa, existingClusterRoleBinding}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentClusterRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify ClusterRoleBinding was updated with expected roleRef
	clusterRoleBinding := &v1.ClusterRoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, clusterRoleBinding)
	assert.NoError(t, err)

	expectedRoleRef := buildRoleRef(generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName), "ClusterRole")
	assert.Equal(t, expectedRoleRef, clusterRoleBinding.RoleRef)
	assert.NotEqual(t, oldRoleRef, clusterRoleBinding.RoleRef)
}

func TestReconcileAgentClusterRoleBinding_ClusterRoleBindingExists_AgentNotSet(t *testing.T) {
	// Test case: ClusterRoleBinding exists but agent is not set (nil)
	// Expected behavior: Should delete the ClusterRoleBinding

	cr := makeTestArgoCD() // No agent configuration
	sa := makeTestServiceAccount()

	// Create existing ClusterRoleBinding
	existingClusterRoleBinding := &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
			Labels: buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Subjects: buildSubjects(sa, cr),
		RoleRef:  buildRoleRef(generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName), "ClusterRole"),
	}

	resObjs := []client.Object{cr, sa, existingClusterRoleBinding}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentClusterRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify ClusterRoleBinding was deleted (since agent is not enabled by default)
	clusterRoleBinding := &v1.ClusterRoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, clusterRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentRoleBinding_RoleBindingDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: RoleBinding doesn't exist and ArgoCDAgent is not set (nil)
	// Expected behavior: Should do nothing since agent is effectively disabled

	cr := makeTestArgoCD() // No agent configuration
	sa := makeTestServiceAccount()

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify RoleBinding was not created
	roleBinding := &v1.RoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, roleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentClusterRoleBinding_ClusterRoleBindingDoesNotExist_AgentNotSet(t *testing.T) {
	// Test case: ClusterRoleBinding doesn't exist and ArgoCDAgent is not set (nil)
	// Expected behavior: Should do nothing since agent is effectively disabled

	cr := makeTestArgoCD() // No agent configuration
	sa := makeTestServiceAccount()

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileAgentClusterRoleBinding(cl, testAgentCompName, sa, cr, sch)
	assert.NoError(t, err)

	// Verify ClusterRoleBinding was not created
	clusterRoleBinding := &v1.ClusterRoleBinding{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, clusterRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}
