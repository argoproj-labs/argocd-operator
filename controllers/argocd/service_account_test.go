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

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestReconcileArgoCD_reconcileServiceAccountPermissions(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	assert.NoError(t, createNamespace(r, a.Namespace, ""))

	// objective is to verify if the right rule associations have happened.

	expectedRules := policyRuleForApplicationController()
	workloadIdentifier := "xrb"

	assert.NoError(t, r.reconcileServiceAccountPermissions(workloadIdentifier, expectedRules, a))

	reconciledServiceAccount := &corev1.ServiceAccount{}
	reconciledRole := &v1.Role{}
	reconcileRoleBinding := &v1.RoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)

	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconcileRoleBinding))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledServiceAccount))
	assert.Equal(t, expectedRules, reconciledRole.Rules)

	// undesirable changes
	reconciledRole.Rules = policyRuleForRedisHa(r.Client)
	assert.NoError(t, r.Client.Update(context.TODO(), reconciledRole))

	// fetch it
	dirtyRole := &v1.Role{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, dirtyRole))
	assert.Equal(t, reconciledRole.Rules, dirtyRole.Rules)

	// Have the reconciler override them
	assert.NoError(t, r.reconcileServiceAccountPermissions(workloadIdentifier, expectedRules, a))

	// fetch it
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.Equal(t, expectedRules, reconciledRole.Rules)
}

func TestReconcileArgoCD_reconcileServiceAccountClusterPermissions(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCD()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	workloadIdentifier := "xrb"
	expectedClusterRoleBindingName := fmt.Sprintf("%s-%s-%s", a.Name, a.Namespace, workloadIdentifier)
	expectedClusterRoleName := fmt.Sprintf("%s-%s-%s", a.Name, a.Namespace, workloadIdentifier)
	expectedNameSA := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)

	reconcileServiceAccount := &corev1.ServiceAccount{}
	reconcileClusterRoleBinding := &v1.ClusterRoleBinding{}
	reconcileClusterRole := &v1.ClusterRole{}

	//reconcile ServiceAccountClusterPermissions with no policy rules
	assert.NoError(t, r.reconcileServiceAccountClusterPermissions(workloadIdentifier, testRules(), a))

	//Service account should be created but no ClusterRole/ClusterRoleBinding should be created
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedNameSA, Namespace: a.Namespace}, reconcileServiceAccount))
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding))
	assert.Contains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding).Error(), "not found")
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleName}, reconcileClusterRole), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleName}, reconcileClusterRole))
	assert.Contains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleName}, reconcileClusterRole).Error(), "not found")

	t.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", a.Namespace)

	// objective is to verify if the right SA associations have happened.
	assert.NoError(t, r.reconcileServiceAccountClusterPermissions(workloadIdentifier, testRules(), a))

	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedNameSA, Namespace: a.Namespace}, reconcileServiceAccount))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleName}, reconcileClusterRole))
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding))

	// undesirable changes
	reconcileClusterRoleBinding.RoleRef.Name = "z"
	reconcileClusterRoleBinding.Subjects[0].Name = "z"
	assert.NoError(t, r.Client.Update(context.TODO(), reconcileClusterRoleBinding))

	// fetch it
	dirtyClusterRoleBinding := &v1.ClusterRoleBinding{}
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, dirtyClusterRoleBinding))
	assert.Equal(t, reconcileClusterRoleBinding.RoleRef.Name, dirtyClusterRoleBinding.RoleRef.Name)

	// Have the reconciler override them
	assert.NoError(t, r.reconcileServiceAccountClusterPermissions(workloadIdentifier, testRules(), a))

	// fetch it
	assert.NoError(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding))
	assert.Equal(t, expectedClusterRoleName, reconcileClusterRoleBinding.RoleRef.Name)

	// Check if cluster role and rolebinding gets deleted
	os.Unsetenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")
	assert.NoError(t, r.reconcileServiceAccountClusterPermissions(workloadIdentifier, testRules(), a))
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding))
	assert.Contains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleBindingName}, reconcileClusterRoleBinding).Error(), "not found")
	//assert.ErrorContains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleName}, reconcileClusterRole), "not found")
	//TODO: https://github.com/stretchr/testify/pull/1022 introduced ErrorContains, but is not yet available in a tagged release. Revert to ErrorContains once this becomes available
	assert.Error(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleName}, reconcileClusterRole))
	assert.Contains(t, r.Client.Get(context.TODO(), types.NamespacedName{Name: expectedClusterRoleName}, reconcileClusterRole).Error(), "not found")
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
