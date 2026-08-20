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
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	testClusterRoleBindingName = "promoter-test-clusterrole-binding"
	testRoleBindingName        = "promoter-test-role-binding"
)

func makeExistingClusterBinding(name, roleName string, cr *argoproj.ArgoCD) rbacv1.ClusterRoleBinding {
	sa := makeExistingServiceAccount(cr)

	return rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: makeLabelsForClusterRoleBinding(cr),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Name:     roleName,
			Kind:     "ClusterRole",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
	}
}

func makeExisintgRoleBinding(name, roleName string, cr *argoproj.ArgoCD) rbacv1.RoleBinding {
	sa := makeExistingServiceAccount(cr)

	return rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    buildLabelsForPromoterResources(testCompName, cr),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Name:     roleName,
			Kind:     "Role",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
	}
}

func makeLabelsForClusterRoleBinding(cr *argoproj.ArgoCD) map[string]string {
	labels := buildLabelsForPromoterResources(testCompName, cr)
	labels[common.ArgoCDKeyName] = argoutil.TruncateWithHash(testClusterRoleBindingName, argoutil.GetMaxLabelLength())
	return labels
}

func TestReconcilePromoterClusterRoleBinding_DoesNotExist_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and ClusterRoleBinding does not exist
	// Expected behavior: ClusterRoleBinding should not be created

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoleBinding, err := ReconcilePromoterClusterRoleBinding(client, testCompName, testClusterRoleBindingName, testClusterRoleName, sa, cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoleBinding)

	retrievedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testClusterRoleBindingName}, retrievedClusterRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterClusterRoleBinding_Exists_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and ClusterRoleBinding exists
	// Expected behavior: ClusterRoleBinding should be deleted

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)
	existingClusterRoleBinding := makeExistingClusterBinding(testClusterRoleBindingName, testClusterRoleName, cr)

	resObjs := []client.Object{cr, sa, &existingClusterRoleBinding}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoleBinding, err := ReconcilePromoterClusterRoleBinding(client, testCompName, testClusterRoleBindingName, testClusterRoleName, sa, cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoleBinding)

	retrievedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testClusterRoleBindingName}, retrievedClusterRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterClusterRoleBinding_DoesNotExist_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and ClusterRoleBinding does not exist
	// Expected behavior: ClusterRoleBinding should be created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoleBinding, err := ReconcilePromoterClusterRoleBinding(client, testCompName, testClusterRoleBindingName, testClusterRoleName, sa, cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoleBinding)

	retrievedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testClusterRoleBindingName}, retrievedClusterRoleBinding)
	assert.NoError(t, err)

	assert.Equal(t, testClusterRoleBindingName, retrievedClusterRoleBinding.Name)
	assert.Equal(t, makeLabelsForClusterRoleBinding(cr), retrievedClusterRoleBinding.Labels)

	assert.Equal(t, sa.Name, retrievedClusterRoleBinding.Subjects[0].Name)
	assert.Equal(t, sa.Namespace, retrievedClusterRoleBinding.Subjects[0].Namespace)
	assert.Equal(t, rbacv1.ServiceAccountKind, retrievedClusterRoleBinding.Subjects[0].Kind)

	assert.Equal(t, testClusterRoleName, retrievedClusterRoleBinding.RoleRef.Name)
	assert.Equal(t, "ClusterRole", retrievedClusterRoleBinding.RoleRef.Kind)
	assert.Equal(t, rbacv1.GroupName, retrievedClusterRoleBinding.RoleRef.APIGroup)
}

func TestReconcilePromoterClusterRoleBinding_DoesNotExists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and ClusterRoleBinding does not exists
	// Expected behavior: ClusterRoleBinding should not be created

	cr := makeTestArgoCD()

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoleBinding, err := ReconcilePromoterClusterRoleBinding(client, testCompName, testClusterRoleBindingName, testClusterRoleName, sa, cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoleBinding)

	retrievedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testClusterRoleBindingName}, retrievedClusterRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterClusterRoleBinding_Exists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and ClusterRoleBinding exists
	// Expected behavior: ClusterRoleBinding should be deleted

	cr := makeTestArgoCD()

	sa := makeExistingServiceAccount(cr)
	existingClusterRoleBinding := makeExistingClusterBinding(testClusterRoleBindingName, testClusterRoleName, cr)

	resObjs := []client.Object{cr, sa, &existingClusterRoleBinding}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoleBinding, err := ReconcilePromoterClusterRoleBinding(client, testCompName, testClusterRoleBindingName, testClusterRoleName, sa, cr, true)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoleBinding)

	retrievedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testClusterRoleBindingName}, retrievedClusterRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterRoleBinding_DoesNotExist_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and RoleBinding does not exist
	// Expected behavior: RoleBinding should not be created

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	roleBinding, err := ReconcilePromoterRoleBinding(client, testCompName, testRoleBindingName, testRoleName, sa, cr, true, true)
	assert.NoError(t, err)
	assert.NotNil(t, roleBinding)

	retrievedRoleBinding := &rbacv1.RoleBinding{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testRoleBindingName, Namespace: testNamespace}, retrievedRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterRoleBinding_Exists_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and RoleBinding exists
	// Expected behavior: RoleBinding should be deleted

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)
	existingRoleBinding := makeExisintgRoleBinding(testRoleBindingName, testRoleName, cr)

	resObjs := []client.Object{cr, sa, &existingRoleBinding}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	roleBinding, err := ReconcilePromoterRoleBinding(client, testCompName, testRoleBindingName, testRoleName, sa, cr, true, true)
	assert.NoError(t, err)
	assert.NotNil(t, roleBinding)

	retrievedRoleBinding := &rbacv1.RoleBinding{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testRoleBindingName, Namespace: testNamespace}, retrievedRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterRoleBinding_DoesNotExist_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and RoleBinding does not exist
	// Expected behavior: RoleBinding should be created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	roleBinding, err := ReconcilePromoterRoleBinding(client, testCompName, testRoleBindingName, testRoleName, sa, cr, true, true)
	assert.NoError(t, err)
	assert.NotNil(t, roleBinding)

	retrievedRoleBinding := &rbacv1.RoleBinding{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testRoleBindingName, Namespace: testNamespace}, retrievedRoleBinding)
	assert.NoError(t, err)

	assert.Equal(t, testRoleBindingName, retrievedRoleBinding.Name)
	assert.Equal(t, testNamespace, retrievedRoleBinding.Namespace)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedRoleBinding.Labels)

	assert.Equal(t, sa.Name, retrievedRoleBinding.Subjects[0].Name)
	assert.Equal(t, sa.Namespace, retrievedRoleBinding.Subjects[0].Namespace)
	assert.Equal(t, rbacv1.ServiceAccountKind, retrievedRoleBinding.Subjects[0].Kind)

	assert.Equal(t, testRoleName, retrievedRoleBinding.RoleRef.Name)
	assert.Equal(t, "Role", retrievedRoleBinding.RoleRef.Kind)
	assert.Equal(t, rbacv1.GroupName, retrievedRoleBinding.RoleRef.APIGroup)
}

func TestReconcilePromoterRoleBinding_DoesNotExists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and RoleBinding does not exists
	// Expected behavior: RoleBinding should not be created

	cr := makeTestArgoCD()

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr, sa}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	roleBinding, err := ReconcilePromoterRoleBinding(client, testCompName, testRoleBindingName, testRoleName, sa, cr, true, true)
	assert.NoError(t, err)
	assert.NotNil(t, roleBinding)

	retrievedRoleBinding := &rbacv1.RoleBinding{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testRoleBindingName, Namespace: testNamespace}, retrievedRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterRoleBinding_Exists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and RoleBinding exists
	// Expected behavior: RoleBinding should be deleted

	cr := makeTestArgoCD()

	sa := makeExistingServiceAccount(cr)
	existingRoleBinding := makeExisintgRoleBinding(testRoleBindingName, testRoleName, cr)

	resObjs := []client.Object{cr, sa, &existingRoleBinding}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	roleBinding, err := ReconcilePromoterRoleBinding(client, testCompName, testRoleBindingName, testRoleName, sa, cr, true, true)
	assert.NoError(t, err)
	assert.NotNil(t, roleBinding)

	retrievedRoleBinding := &rbacv1.RoleBinding{}
	err = client.Get(context.Background(), types.NamespacedName{Name: testRoleBindingName, Namespace: testNamespace}, retrievedRoleBinding)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterControllerClusterRoleBindings_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled so Controller ClusterRoleBindings should not be created
	// Expected behavior: cluster role bindings should not get created

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoleBindings, err := ReconcilePromoterControllerClusterRoleBindings(client, testCompName, sa, cr)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoleBindings)

	for _, clusterRoleBinding := range buildPolicyRulesForControllerClusterRoles(testCompName, cr) {
		retrievedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name: clusterRoleBinding.name,
		}, retrievedClusterRoleBinding)
		assert.True(t, errors.IsNotFound(err))
	}
}

func TestReconcilePromoterControllerClusterRoleBindings_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled so Controller ClusterRoleBindings should be created
	// Expected behavior: cluster role bindings should get created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoleBindings, err := ReconcilePromoterControllerClusterRoleBindings(client, testCompName, sa, cr)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoleBindings)

	for _, clusterRoleBinding := range buildPolicyRulesForControllerClusterRoles(testCompName, cr) {
		retrievedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name: clusterRoleBinding.name,
		}, retrievedClusterRoleBinding)
		assert.NoError(t, err)

		assert.Equal(t, clusterRoleBinding.name, retrievedClusterRoleBinding.Name)
		assert.Equal(t, clusterRoleBinding.roleRefName, retrievedClusterRoleBinding.RoleRef.Name)
	}
}

func TestReconcilePromoterAPIServerClusterRoleBindings_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled so API Server ClusterRoleBindings should not be created
	// Expected behavior: cluster role bindings should not get created

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoleBindings, err := ReconcilePromoterAPIServerClusterRoleBindings(client, testCompName, sa, cr)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoleBindings)

	for _, clusterRoleBinding := range buildPolicyRulesForAPIServerClusterRoles(testCompName, cr) {
		retrievedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name: clusterRoleBinding.name,
		}, retrievedClusterRoleBinding)
		assert.True(t, errors.IsNotFound(err))
	}
}

func TestReconcilePromoterAPIServerClusterRoleBindings_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled so API Server ClusterRoleBindings should be created
	// Expected behavior: cluster role bindings should get created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoleBindings, err := ReconcilePromoterAPIServerClusterRoleBindings(client, testCompName, sa, cr)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoleBindings)

	for _, clusterRoleBinding := range buildPolicyRulesForAPIServerClusterRoles(testCompName, cr) {
		retrievedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name: clusterRoleBinding.name,
		}, retrievedClusterRoleBinding)
		assert.NoError(t, err)

		assert.Equal(t, clusterRoleBinding.name, retrievedClusterRoleBinding.Name)
		assert.Equal(t, clusterRoleBinding.roleRefName, retrievedClusterRoleBinding.RoleRef.Name)
	}
}

func TestReconcilePromoterAPIServerClusterRoleBindings_PromoterEnabled_APIServerDisabled(t *testing.T) {
	// Test Case: Promoter is enabled but API Server is disabled so API Server ClusterRoleBindings should not be created
	// Expected behavior: cluster role bindings should not get created

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(false))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	clusterRoleBindings, err := ReconcilePromoterAPIServerClusterRoleBindings(client, testCompName, sa, cr)
	assert.NoError(t, err)
	assert.NotNil(t, clusterRoleBindings)

	for _, clusterRoleBinding := range buildPolicyRulesForAPIServerClusterRoles(testCompName, cr) {
		retrievedClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name: clusterRoleBinding.name,
		}, retrievedClusterRoleBinding)
		assert.True(t, errors.IsNotFound(err))
	}
}

func TestReconcilePromoterAPIServerRoleBindings_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled so API Server RoleBindings should not be created
	// Expected behavior: role bindings should not get created

	cr := makeTestArgoCD(withPromoterEnabled(false))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	roleBindings, err := ReconcilePromoterAPIServerRoleBindings(client, testCompName, sa, cr)
	assert.NoError(t, err)
	assert.NotNil(t, roleBindings)

	for _, roleBinding := range roleBindings {
		retrievedRoleBinding := &rbacv1.RoleBinding{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name:      roleBinding.Name,
			Namespace: cr.Namespace,
		}, retrievedRoleBinding)
		assert.True(t, errors.IsNotFound(err))
	}
}

func TestReconcilePromoterClusterRoleBindings_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled so role bindings should get created
	// Expected behavior: role bindings should get created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	roleBindings, err := ReconcilePromoterAPIServerRoleBindings(client, testCompName, sa, cr)
	assert.NoError(t, err)
	assert.NotNil(t, roleBindings)

	for _, roleBinding := range roleBindings {
		retrievedRoleBinding := &rbacv1.RoleBinding{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name:      roleBinding.Name,
			Namespace: roleBinding.Namespace,
		}, retrievedRoleBinding)
		assert.NoError(t, err)

		assert.Equal(t, roleBinding.Name, retrievedRoleBinding.Name)
		assert.Equal(t, roleBinding.RoleRef.Name, retrievedRoleBinding.RoleRef.Name)
	}
}

func TestReconcilePromoterAPIServerRoleBindings_PromoterEnabled_APIServerDisabled(t *testing.T) {
	// Test Case: Promoter is enabled but API Server is disabled so API Server RoleBindings should not be created
	// Expected behavior: role bindings should not get created

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(false))

	sa := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	roleBindings, err := ReconcilePromoterAPIServerRoleBindings(client, testCompName, sa, cr)
	assert.NoError(t, err)
	assert.NotNil(t, roleBindings)

	for _, roleBinding := range roleBindings {
		retrievedRoleBinding := &rbacv1.RoleBinding{}
		err = client.Get(context.Background(), types.NamespacedName{
			Name:      roleBinding.Name,
			Namespace: roleBinding.Namespace,
		}, retrievedRoleBinding)
		assert.True(t, errors.IsNotFound(err))
	}
}
