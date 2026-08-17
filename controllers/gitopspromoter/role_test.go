// Copyright 2026 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gitopspromoter

import (
	"context"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

const (
	testClusterRoleName = "gitops-promoter-clusterrole"
	testRoleName        = "gitops-promoter-role"
)

func makeTestPolicyRule() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{"promoter.argoproj.io"},
			Resources: []string{"gitrepositories"},
			Verbs:     []string{"create", "get", "patch", "delete"},
		},
	}
}

func makeExistingClusterRole(cr *argoproj.ArgoCD, policyRule []rbacv1.PolicyRule) rbacv1.ClusterRole {
	return rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   testClusterRoleName,
			Labels: buildLabelsForPromoterResources(testCompName, cr),
		},
		Rules: policyRule,
	}
}

func TestReconcilePromoterClusterRole_DoesNotExist_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and ClusterRole does not exist
	// Expected: ClusterRole should not be created

	cr := makeTestArgoCD(withPromoterEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePromoterClusterRole(client, testCompName, testClusterRoleName, makeTestPolicyRule(), cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	retrievedClusterRole := &rbacv1.ClusterRole{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testClusterRoleName}, retrievedClusterRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterClusterRole_DoesNotExist_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and ClusterRole does not exist
	// Expected: ClusterRole should get created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePromoterClusterRole(client, testCompName, testClusterRoleName, makeTestPolicyRule(), cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	retrievedClusterRole := &rbacv1.ClusterRole{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: testClusterRoleName,
	}, retrievedClusterRole)
	assert.NoError(t, err)

	assert.Equal(t, testClusterRoleName, retrievedClusterRole.Name)
	assert.Equal(t, makeTestPolicyRule(), retrievedClusterRole.Rules)
}

func TestReconcilePromoterClusterRole_Exists_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and ClusterRole exists
	// Expected: ClusterRole should be deleted

	cr := makeTestArgoCD(withPromoterEnabled(false))

	existingClusterRole := makeExistingClusterRole(cr, makeTestPolicyRule())

	resObjs := []client.Object{cr, &existingClusterRole}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePromoterClusterRole(client, testCompName, testClusterRoleName, makeTestPolicyRule(), cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	retrievedClusterRole := &rbacv1.ClusterRole{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: testClusterRoleName,
	}, retrievedClusterRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterClusterRole_DoesNotExists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and ClusterRole does not exist
	// Expected: ClusterRole should not be created

	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePromoterClusterRole(client, testCompName, testClusterRoleName, makeTestPolicyRule(), cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	retrievedClusterRole := &rbacv1.ClusterRole{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: testClusterRoleName,
	}, retrievedClusterRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterClusterRole_Exists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and ClusterRole exists
	// Expected: ClusterRole should be deleted

	cr := makeTestArgoCD()

	existingClusterRole := makeExistingClusterRole(cr, makeTestPolicyRule())

	resObjs := []client.Object{cr, &existingClusterRole}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePromoterClusterRole(client, testCompName, testClusterRoleName, makeTestPolicyRule(), cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	retrievedClusterRole := &rbacv1.ClusterRole{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: testClusterRoleName,
	}, retrievedClusterRole)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterClusterRole_Exists_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and ClusterRole exists
	// Expected: ClusterRole should be the same

	cr := makeTestArgoCD(withPromoterEnabled(true))

	existingClusterRole := makeExistingClusterRole(cr, makeTestPolicyRule())

	resObjs := []client.Object{cr, &existingClusterRole}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRole, err := ReconcilePromoterClusterRole(client, testCompName, testClusterRoleName, makeTestPolicyRule(), cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRole)

	retrievedClusterRole := &rbacv1.ClusterRole{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: testClusterRoleName,
	}, retrievedClusterRole)
	assert.NoError(t, err)

	assert.Equal(t, testClusterRoleName, retrievedClusterRole.Name)
	assert.Equal(t, makeTestPolicyRule(), retrievedClusterRole.Rules)
}

func TestReconcilePromoterControllerClusterRoles_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and ClusterRoles do not exist
	// Expected behavior: No cluster roles should be created

	cr := makeTestArgoCD(withPromoterEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoles, err := ReconcilePromoterControllerClusterRoles(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoles)

	for _, clusterRole := range buildPolicyRulesForControllerClusterRoles(testCompName, cr) {
		retrievedClusterRole := &rbacv1.ClusterRole{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name: clusterRole.name,
		}, retrievedClusterRole)
		assert.True(t, errors.IsNotFound(err))
	}
}

func TestReconcilePromoterControllerClusterRoles_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and ClusterRoles do not exist
	// Expected: The wanted cluster roles should be reconciled

	cr := makeTestArgoCD(withPromoterEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoles, err := ReconcilePromoterControllerClusterRoles(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoles)

	for _, clusterRole := range buildPolicyRulesForControllerClusterRoles(testCompName, cr) {
		retrievedClusterRole := &rbacv1.ClusterRole{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name: clusterRole.name,
		}, retrievedClusterRole)
		assert.NoError(t, err)

		assert.Equal(t, clusterRole.name, retrievedClusterRole.Name)
		assert.Equal(t, clusterRole.policyRule, retrievedClusterRole.Rules)
	}
}

func TestReconcilePromoterAPIServerClusterRoles_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled so API Server resources should not exist and ClusterRoles do not exist
	// Expected behavior: No cluster roles should be created for the API Server

	cr := makeTestArgoCD(withPromoterEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoles, err := ReconcilePromoterAPIServerClusterRoles(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoles)

	for _, clusterRole := range buildPolicyRulesForAPIServerClusterRoles(testCompName, cr) {
		retrievedClusterRole := &rbacv1.ClusterRole{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name: clusterRole.name,
		}, retrievedClusterRole)
		assert.True(t, errors.IsNotFound(err))
	}
}

func TestReconcilePromoterAPIServerClusterRoles_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled so API Server resources should be created
	// Expected behavior: Cluster roles for API Server should be created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoles, err := ReconcilePromoterAPIServerClusterRoles(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoles)

	for _, clusterRole := range buildPolicyRulesForAPIServerClusterRoles(testCompName, cr) {
		retrievedClusterRole := &rbacv1.ClusterRole{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name: clusterRole.name,
		}, retrievedClusterRole)
		assert.NoError(t, err)

		assert.Equal(t, clusterRole.name, retrievedClusterRole.Name)
		assert.Equal(t, clusterRole.policyRule, retrievedClusterRole.Rules)
	}
}

func TestReconcilePromoterAPIServerClusterRoles_PromoterEnabled_APIServerDisabled(t *testing.T) {
	// Test Case: Promoter is enabled and API Server is disabled so API Server resources do not exist
	// Expected behavior: No cluster roles should be created for the API Server

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoles, err := ReconcilePromoterAPIServerClusterRoles(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoles)

	for _, clusterRole := range buildPolicyRulesForAPIServerClusterRoles(testCompName, cr) {
		retrievedClusterRole := &rbacv1.ClusterRole{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name: clusterRole.name,
		}, retrievedClusterRole)
		assert.True(t, errors.IsNotFound(err))
	}
}
