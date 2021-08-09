// Copyright 2019 ArgoCD Operator Developers
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
	"testing"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileServiceAccountPermissions(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)
	assert.NilError(t, createNamespace(r, a.Namespace, a.Namespace))

	// objective is to verify if the right rule associations have happened.

	expectedRules := policyRuleForApplicationController()
	workloadIdentifier := "xrb"

	assert.NilError(t, r.reconcileServiceAccountPermissions(workloadIdentifier, expectedRules, a))

	reconciledServiceAccount := &corev1.ServiceAccount{}
	reconciledRole := &v1.Role{}
	reconcileRoleBinding := &v1.RoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)

	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconcileRoleBinding))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledServiceAccount))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)

	// undesirable changes
	reconciledRole.Rules = policyRuleForRedisHa(a)
	assert.NilError(t, r.client.Update(context.TODO(), reconciledRole))

	// fetch it
	dirtyRole := &v1.Role{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, dirtyRole))
	assert.DeepEqual(t, reconciledRole.Rules, dirtyRole.Rules)

	// Have the reconciler override them
	assert.NilError(t, r.reconcileServiceAccountPermissions(workloadIdentifier, expectedRules, a))

	// fetch it
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)
}

func TestReconcileArgoCD_reconcileServiceAccountClusterPermissions(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := "xrb"
	expectedClusterRoleBindingName := fmt.Sprintf("%s-%s-%s", a.Name, a.Namespace, workloadIdentifier)
	expectedClusterRoleName := fmt.Sprintf("%s-%s-%s", a.Name, a.Namespace, workloadIdentifier)
	expectedNameSA := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)

	reconcileServiceAccount := &corev1.ServiceAccount{}
	reconcileClusterRoleBinding := &v1.ClusterRoleBinding{}
	reconcileClusterRole := &v1.ClusterRole{}

	//reconcile ServiceAccountClusterPermissions with no policy rules
	assert.NilError(t, r.reconcileServiceAccountClusterPermissions(workloadIdentifier, testRules(), a))

	//Service account should be created but no ClusterRole/ClusterRoleBinding should be created
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedNameSA, Namespace: a.Namespace}, reconcileServiceAccount))
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding), "not found")
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleName}, reconcileClusterRole), "not found")

	os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", a.Namespace)

	// objective is to verify if the right SA associations have happened.
	assert.NilError(t, r.reconcileServiceAccountClusterPermissions(workloadIdentifier, testRules(), a))

	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedNameSA, Namespace: a.Namespace}, reconcileServiceAccount))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleName}, reconcileClusterRole))
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding))

	// undesirable changes
	reconcileClusterRoleBinding.RoleRef.Name = "z"
	reconcileClusterRoleBinding.Subjects[0].Name = "z"
	assert.NilError(t, r.client.Update(context.TODO(), reconcileClusterRoleBinding))

	// fetch it
	dirtyClusterRoleBinding := &v1.ClusterRoleBinding{}
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, dirtyClusterRoleBinding))
	assert.Equal(t, reconcileClusterRoleBinding.RoleRef.Name, dirtyClusterRoleBinding.RoleRef.Name)

	// Have the reconciler override them
	assert.NilError(t, r.reconcileServiceAccountClusterPermissions(workloadIdentifier, testRules(), a))

	// fetch it
	assert.NilError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding))
	assert.Equal(t, expectedClusterRoleName, reconcileClusterRoleBinding.RoleRef.Name)

	// Check if cluster role and rolebinding gets deleted
	os.Unsetenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")
	assert.NilError(t, r.reconcileServiceAccountClusterPermissions(workloadIdentifier, testRules(), a))
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding), "not found")
	assert.ErrorContains(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleName}, reconcileClusterRole), "not found")
}

func testRules() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
			},
		},
	}
}
