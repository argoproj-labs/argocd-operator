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
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

// TestReconcileAgentRole tests

func TestReconcileAgentRole_RoleDoesNotExist_AgentDisabled(t *testing.T) {
	// Test case: Role doesn't exist and agent is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withAgentEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcileAgentRole(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role was not created
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentRole_RoleDoesNotExist_AgentEnabled(t *testing.T) {
	// Test case: Role doesn't exist and agent is enabled
	// Expected behavior: Should create the Role with expected rules

	cr := makeTestArgoCD(withAgentEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcileAgentRole(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role was created
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.NoError(t, err)

	// Verify Role has expected metadata
	assert.Equal(t, generateAgentResourceName(cr.Name, testAgentCompName), retrievedRole.Name)
	assert.Equal(t, cr.Namespace, retrievedRole.Namespace)
	assert.Equal(t, buildLabelsForAgent(cr.Name, testAgentCompName), retrievedRole.Labels)

	// Verify Role has expected rules
	expectedRules := buildPolicyRuleForRole()
	assert.Equal(t, expectedRules, retrievedRole.Rules)

	// Verify owner reference is set
	assert.Len(t, retrievedRole.OwnerReferences, 1)
	assert.Equal(t, cr.Name, retrievedRole.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", retrievedRole.OwnerReferences[0].Kind)
}

func TestReconcileAgentRole_RoleExists_AgentDisabled(t *testing.T) {
	// Test case: Role exists and agent is disabled
	// Expected behavior: Should delete the Role

	cr := makeTestArgoCD(withAgentEnabled(false))

	// Create existing Role
	existingRole := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Rules: buildPolicyRuleForRole(),
	}

	resObjs := []client.Object{cr, existingRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcileAgentRole(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role was deleted
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentRole_RoleExists_AgentEnabled_SameRules(t *testing.T) {
	// Test case: Role exists, agent is enabled, and rules are the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withAgentEnabled(true))

	expectedRules := buildPolicyRuleForRole()
	existingRole := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Rules: expectedRules,
	}

	resObjs := []client.Object{cr, existingRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcileAgentRole(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role still exists with same rules
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.NoError(t, err)
	assert.Equal(t, expectedRules, retrievedRole.Rules)
}

func TestReconcileAgentRole_RoleExists_AgentEnabled_DifferentRules(t *testing.T) {
	// Test case: Role exists, agent is enabled, but rules are different
	// Expected behavior: Should update the Role with new rules

	cr := makeTestArgoCD(withAgentEnabled(true))

	// Create existing Role with different rules
	existingRole := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
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

	role, err := ReconcileAgentRole(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role was updated with expected rules
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.NoError(t, err)

	expectedRules := buildPolicyRuleForRole()
	assert.Equal(t, expectedRules, retrievedRole.Rules)
}

func TestReconcileAgentRole_RoleExists_AgentNotSet(t *testing.T) {
	// Test case: Role exists but agent is not set (nil)
	// Expected behavior: Should delete the Role

	cr := makeTestArgoCD() // No agent configuration

	// Create existing Role
	existingRole := &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, testAgentCompName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Rules: buildPolicyRuleForRole(),
	}

	resObjs := []client.Object{cr, existingRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	role, err := ReconcileAgentRole(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, role)

	// Verify Role was deleted (since agent is not enabled by default)
	retrievedRole := &v1.Role{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      generateAgentResourceName(cr.Name, testAgentCompName),
		Namespace: cr.Namespace,
	}, retrievedRole)
	assert.True(t, errors.IsNotFound(err))
}

// TestReconcileAgentClusterRoles tests

func TestReconcileAgentClusterRoles_ClusterRoleDoesNotExist_AgentDisabled(t *testing.T) {
	// Test case: ClusterRole doesn't exist and agent is disabled
	// Expected behavior: Should do nothing (no creation, no error)

	cr := makeTestArgoCD(withAgentEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcileAgentClusterRoles(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole was not created
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, retrievedClusterRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentClusterRoles_ClusterRoleDoesNotExist_AgentEnabled(t *testing.T) {
	// Test case: ClusterRole doesn't exist and agent is enabled
	// Expected behavior: Should create the ClusterRole with expected rules

	cr := makeTestArgoCD(withAgentEnabled(true))
	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", cr.Namespace)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcileAgentClusterRoles(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole was created
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, retrievedClusterRole)
	assert.NoError(t, err)

	// Verify ClusterRole has expected metadata
	assert.Equal(t, generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName), retrievedClusterRole.Name)
	assert.Equal(t, buildLabelsForAgent(cr.Name, testAgentCompName), retrievedClusterRole.Labels)

	// Verify ClusterRole has expected rules
	expectedRules := buildPolicyRuleForClusterRole(cr)
	assert.Equal(t, expectedRules, retrievedClusterRole.Rules)

	// Verify no owner reference is set for ClusterRole (as expected from the code)
	assert.Len(t, retrievedClusterRole.OwnerReferences, 0)
}

func TestReconcileAgentClusterRoles_ClusterRoleExists_AgentDisabled(t *testing.T) {
	// Test case: ClusterRole exists and agent is disabled
	// Expected behavior: Should delete the ClusterRole

	cr := makeTestArgoCD(withAgentEnabled(false))

	// Create existing ClusterRole
	existingClusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
			Labels: buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Rules: buildPolicyRuleForClusterRole(cr),
	}

	resObjs := []client.Object{cr, existingClusterRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcileAgentClusterRoles(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole was deleted
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, retrievedClusterRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcileAgentClusterRoles_ClusterRoleExists_AgentEnabled_SameRules(t *testing.T) {
	// Test case: ClusterRole exists, agent is enabled, and rules are the same
	// Expected behavior: Should do nothing (no update)

	cr := makeTestArgoCD(withAgentEnabled(true))
	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", cr.Namespace)

	expectedRules := buildPolicyRuleForClusterRole(cr)
	existingClusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
			Labels: buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Rules: expectedRules,
	}

	resObjs := []client.Object{cr, existingClusterRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcileAgentClusterRoles(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole still exists with same rules
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, retrievedClusterRole)
	assert.NoError(t, err)
	assert.Equal(t, expectedRules, retrievedClusterRole.Rules)
}

func TestReconcileAgentClusterRoles_ClusterRoleExists_AgentEnabled_DifferentRules(t *testing.T) {
	// Test case: ClusterRole exists, agent is enabled, but rules are different
	// Expected behavior: Should update the ClusterRole with new rules

	cr := makeTestArgoCD(withAgentEnabled(true))
	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", cr.Namespace)

	// Create existing ClusterRole with different rules
	existingClusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
			Labels: buildLabelsForAgent(cr.Name, testAgentCompName),
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

	clusterRole, err := ReconcileAgentClusterRoles(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole was updated with expected rules
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, retrievedClusterRole)
	assert.NoError(t, err)

	expectedRules := buildPolicyRuleForClusterRole(cr)
	assert.Equal(t, expectedRules, retrievedClusterRole.Rules)
}

func TestReconcileAgentClusterRoles_ClusterRoleExists_AgentNotSet(t *testing.T) {
	// Test case: ClusterRole exists but agent is not set (nil)
	// Expected behavior: Should delete the ClusterRole

	cr := makeTestArgoCD() // No agent configuration

	// Create existing ClusterRole
	existingClusterRole := &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
			Labels: buildLabelsForAgent(cr.Name, testAgentCompName),
		},
		Rules: buildPolicyRuleForClusterRole(cr),
	}

	resObjs := []client.Object{cr, existingClusterRole}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcileAgentClusterRoles(cl, testAgentCompName, cr, sch)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	// Verify ClusterRole was deleted (since agent is not enabled by default)
	retrievedClusterRole := &v1.ClusterRole{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name: generateAgentResourceName(cr.Name+"-"+cr.Namespace, testAgentCompName),
	}, retrievedClusterRole)
	assert.True(t, errors.IsNotFound(err))
}

// withAgentDestinationMapping configures DestinationBasedMapping on the Agent spec
func withAgentDestinationMapping(enabled, createNS bool) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		if a.Spec.ArgoCDAgent == nil {
			a.Spec.ArgoCDAgent = &argoproj.ArgoCDAgentSpec{}
		}
		if a.Spec.ArgoCDAgent.Agent == nil {
			a.Spec.ArgoCDAgent.Agent = &argoproj.AgentSpec{}
		}
		a.Spec.ArgoCDAgent.Agent.DestinationBasedMapping = &argoproj.DestinationBasedMappingSpec{
			Enabled:         ptr.To(enabled),
			CreateNamespace: ptr.To(createNS),
		}
	}
}

func TestBuildPolicyRuleForClusterRole_Table(t *testing.T) {
	tests := []struct {
		name                    string
		dmEnabled               bool
		createNamespace         bool
		clusterConfigNamespaces string // value for ARGOCD_CLUSTER_CONFIG_NAMESPACES
		wantExpandedRules       bool   // true if we expect cluster-wide app permissions
	}{
		{
			name:              "namespace based mapping",
			dmEnabled:         false,
			createNamespace:   false,
			wantExpandedRules: false,
		},
		{
			name:                    "destination based mapping in non-cluster-scoped namespace",
			dmEnabled:               true,
			createNamespace:         false,
			clusterConfigNamespaces: "other-namespace",
			wantExpandedRules:       false,
		},
		{
			name:                    "destination based mapping without cluster config namespaces set",
			dmEnabled:               true,
			createNamespace:         false,
			clusterConfigNamespaces: "",
			wantExpandedRules:       false,
		},
		{
			name:                    "destination based mapping in cluster-scoped namespace without creating namespace",
			dmEnabled:               true,
			createNamespace:         false,
			clusterConfigNamespaces: testNamespace,
			wantExpandedRules:       true,
		},
		{
			name:                    "destination based mapping in cluster-scoped namespace with creating namespace",
			dmEnabled:               true,
			createNamespace:         true,
			clusterConfigNamespaces: testNamespace,
			wantExpandedRules:       true,
		},
		{
			name:                    "destination based mapping with wildcard cluster config",
			dmEnabled:               true,
			createNamespace:         false,
			clusterConfigNamespaces: "*",
			wantExpandedRules:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", tt.clusterConfigNamespaces)

			// Build CR for this case
			cr := makeTestArgoCD(withAgentEnabled(true))
			if tt.dmEnabled {
				cr = makeTestArgoCD(withAgentEnabled(true), withAgentDestinationMapping(true, tt.createNamespace))
			}

			rules := buildPolicyRuleForClusterRole(cr)

			defaultRules := []v1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"namespaces"},
					Verbs:     []string{"list", "watch"},
				},
			}

			if !tt.wantExpandedRules {
				assert.Len(t, rules, 1)
				assert.ElementsMatch(t, defaultRules, rules)
				return
			}

			appRules := v1.PolicyRule{
				APIGroups: []string{"argoproj.io"},
				Resources: []string{"applications"},
				Verbs: []string{
					"create",
					"get",
					"list",
					"watch",
					"update",
					"delete",
					"patch",
				},
			}

			assert.Len(t, rules, 2)
			assert.Equal(t, appRules, rules[1])

			if tt.createNamespace {
				defaultRules[0].Verbs = append(defaultRules[0].Verbs, "create", "get")
			}
			assert.Equal(t, defaultRules[0], rules[0])
		})
	}
}
