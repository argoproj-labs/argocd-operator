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
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestReconcilePrincipalRole tests

func TestReconcilePrincipalRole_RoleDoesNotExist_PrincipalDisabled(t *testing.T) {
	// Test case: Role doesn't exist and principal is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcilePrincipalRole(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role was not created
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalRole_RoleDoesNotExist_PrincipalEnabled(t *testing.T) {
	// Test case: Role doesn't exist and principal is enabled
	// Expected behavior: Should create the Role with expected rules

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcilePrincipalRole(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role was created
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.NoError(t, err)

	// Verify Role has expected metadata
	assert.Equal(t, generateAgentResourceName(cr.Name, testCompName), retrievedRole.Name)
	assert.Equal(t, cr.Namespace, retrievedRole.Namespace)
	assert.Equal(t, buildLabelsForAgentPrincipal(cr.Name, testCompName), retrievedRole.Labels)

	// Verify Role has expected rules
	expectedRules := buildPolicyRuleForRole()
	assert.Equal(t, expectedRules, retrievedRole.Rules)

	// Verify owner reference is set
	assert.Len(t, retrievedRole.OwnerReferences, 1)
	assert.Equal(t, cr.Name, retrievedRole.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", retrievedRole.OwnerReferences[0].Kind)
}

func TestReconcilePrincipalRole_RoleExists_PrincipalDisabled(t *testing.T) {
	// Test case: Role exists and principal is disabled
	// Expected behavior: Should delete the Role

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	// Create existing Role
	existingRole := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Rules: buildPolicyRuleForRole(),
	}

	resObjs := []client.Object{cr, existingRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcilePrincipalRole(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role was deleted
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalRole_RoleExists_PrincipalEnabled_SameRules(t *testing.T) {
	// Test case: Role exists, principal is enabled, and rules are the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	expectedRules := buildPolicyRuleForRole()
	existingRole := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Rules: expectedRules,
	}

	resObjs := []client.Object{cr, existingRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcilePrincipalRole(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role still exists with same rules
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.NoError(t, err)
	assert.Equal(t, expectedRules, retrievedRole.Rules)
}

func TestReconcilePrincipalRole_RoleExists_PrincipalEnabled_DifferentRules(t *testing.T) {
	// Test case: Role exists, principal is enabled, but rules are different
	// Expected behavior: Should update the Role with new rules

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	// Create existing Role with different rules
	existingRole := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Rules: []v1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list"},
			},
		},
	}

	resObjs := []client.Object{cr, existingRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcilePrincipalRole(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role was updated with expected rules
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.NoError(t, err)

	expectedRules := buildPolicyRuleForRole()
	assert.Equal(t, expectedRules, retrievedRole.Rules)
}

func TestReconcilePrincipalRole_RoleExists_PrincipalNotSet(t *testing.T) {
	// Test case: Role exists but principal is not set (nil)
	// Expected behavior: Should delete the Role

	cr := makeTestArgoCD() // No principal configuration

	// Create existing Role
	existingRole := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Rules: buildPolicyRuleForRole(),
	}

	resObjs := []client.Object{cr, existingRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcilePrincipalRole(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role was deleted (since principal is not enabled by default)
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.True(t, errors.IsNotFound(err))
}

// TestReconcilePrincipalClusterRoles tests

func TestReconcilePrincipalClusterRoles_ClusterRoleDoesNotExist_PrincipalDisabled(t *testing.T) {
	// Test case: ClusterRole doesn't exist and principal is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePrincipalClusterRoles(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole was not created
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName),
	}, retrievedClusterRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalClusterRoles_ClusterRoleDoesNotExist_PrincipalEnabled(t *testing.T) {
	// Test case: ClusterRole doesn't exist and principal is enabled
	// Expected behavior: Should create the ClusterRole with expected rules

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePrincipalClusterRoles(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole was created
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName),
	}, retrievedClusterRole)
	assert.NoError(t, err)

	// Verify ClusterRole has expected metadata
	assert.Equal(t, generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName), retrievedClusterRole.Name)
	assert.Equal(t, buildLabelsForAgentPrincipal(cr.Name, testCompName), retrievedClusterRole.Labels)

	// Verify ClusterRole has expected rules
	expectedRules := buildPolicyRuleForClusterRole()
	assert.Equal(t, expectedRules, retrievedClusterRole.Rules)

	// Verify no owner reference is set for ClusterRole (as expected from the code)
	assert.Len(t, retrievedClusterRole.OwnerReferences, 0)
}

func TestReconcilePrincipalClusterRoles_ClusterRoleExists_PrincipalDisabled(t *testing.T) {
	// Test case: ClusterRole exists and principal is disabled
	// Expected behavior: Should delete the ClusterRole

	cr := makeTestArgoCD(withPrincipalEnabled(false))

	// Create existing ClusterRole
	existingClusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName),
			Labels: buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Rules: buildPolicyRuleForClusterRole(),
	}

	resObjs := []client.Object{cr, existingClusterRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePrincipalClusterRoles(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole was deleted
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName),
	}, retrievedClusterRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePrincipalClusterRoles_ClusterRoleExists_PrincipalEnabled_SameRules(t *testing.T) {
	// Test case: ClusterRole exists, principal is enabled, and rules are the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	expectedRules := buildPolicyRuleForClusterRole()
	existingClusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName),
			Labels: buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Rules: expectedRules,
	}

	resObjs := []client.Object{cr, existingClusterRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePrincipalClusterRoles(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole still exists with same rules
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName),
	}, retrievedClusterRole)
	assert.NoError(t, err)
	assert.Equal(t, expectedRules, retrievedClusterRole.Rules)
}

func TestReconcilePrincipalClusterRoles_ClusterRoleExists_PrincipalEnabled_DifferentRules(t *testing.T) {
	// Test case: ClusterRole exists, principal is enabled, but rules are different
	// Expected behavior: Should update the ClusterRole with new rules

	cr := makeTestArgoCD(withPrincipalEnabled(true))

	// Create existing ClusterRole with different rules
	existingClusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName),
			Labels: buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Rules: []v1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list"},
			},
		},
	}

	resObjs := []client.Object{cr, existingClusterRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePrincipalClusterRoles(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole was updated with expected rules
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName),
	}, retrievedClusterRole)
	assert.NoError(t, err)

	expectedRules := buildPolicyRuleForClusterRole()
	assert.Equal(t, expectedRules, retrievedClusterRole.Rules)
}

func TestReconcilePrincipalClusterRoles_ClusterRoleExists_PrincipalNotSet(t *testing.T) {
	// Test case: ClusterRole exists but principal is not set (nil)
	// Expected behavior: Should delete the ClusterRole

	cr := makeTestArgoCD() // No principal configuration

	// Create existing ClusterRole
	existingClusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName),
			Labels: buildLabelsForAgentPrincipal(cr.Name, testCompName),
		},
		Rules: buildPolicyRuleForClusterRole(),
	}

	resObjs := []client.Object{cr, existingClusterRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePrincipalClusterRoles(cl, testCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole was deleted (since principal is not enabled by default)
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testCompName),
	}, retrievedClusterRole)
	assert.True(t, errors.IsNotFound(err))
}
