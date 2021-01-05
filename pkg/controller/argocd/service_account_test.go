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
	"testing"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestReconcileArgoCD_reconcileServiceAccountPermissions(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	// objective is to verify if the right rule associations have happened.

	expectedRules := policyRuleForApplicationController()
	workloadIdentifier := "xrb"

	assertNoError(t, r.reconcileServiceAccountPermissions(workloadIdentifier, expectedRules, a))

	reconciledServiceAccount := &corev1.ServiceAccount{}
	reconciledRole := &v1.Role{}
	reconcileRoleBinding := &v1.RoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)

	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconcileRoleBinding))
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledServiceAccount))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)

	// undesirable changes
	reconciledRole.Rules = policyRuleForRedisHa()
	assertNoError(t, r.client.Update(context.TODO(), reconciledRole))

	// fetch it
	dirtyRole := &v1.Role{}
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, dirtyRole))
	assert.DeepEqual(t, reconciledRole.Rules, dirtyRole.Rules)

	// Have the reconciler override them
	assertNoError(t, r.reconcileServiceAccountPermissions(workloadIdentifier, expectedRules, a))

	// fetch it
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledRole))
	assert.DeepEqual(t, expectedRules, reconciledRole.Rules)
}

func TestReconcileArgoCD_reconcileServiceAccountClusterPermissions(t *testing.T) {
	logf.SetLogger(logf.ZapLogger(true))
	a := makeTestArgoCD()
	r := makeTestReconciler(t, a)

	workloadIdentifier := "xrb"

	clusterRole := v1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: workloadIdentifier}, Rules: policyRuleForApplicationController()}
	assertNoError(t, r.client.Create(context.TODO(), &clusterRole))

	// objective is to verify if the right SA associations have happened.

	assertNoError(t, r.reconcileServiceAccountClusterPermissions(workloadIdentifier, policyRuleForServerClusterRole(), a))

	reconciledServiceAccount := &corev1.ServiceAccount{}
	reconcileClusterRoleBinding := &v1.ClusterRoleBinding{}
	expectedName := fmt.Sprintf("%s-%s", a.Name, workloadIdentifier)

	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, reconcileClusterRoleBinding))
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: a.Namespace}, reconciledServiceAccount))

	// undesirable changes
	reconcileClusterRoleBinding.RoleRef.Name = "z"
	reconcileClusterRoleBinding.Subjects[0].Name = "z"
	assertNoError(t, r.client.Update(context.TODO(), reconcileClusterRoleBinding))

	// fetch it
	dirtyClusterRoleBinding := &v1.ClusterRoleBinding{}
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, dirtyClusterRoleBinding))
	assert.Equal(t, reconcileClusterRoleBinding.RoleRef.Name, dirtyClusterRoleBinding.RoleRef.Name)

	// Have the reconciler override them
	assertNoError(t, r.reconcileServiceAccountClusterPermissions(workloadIdentifier, policyRuleForServerClusterRole(), a))

	// fetch it
	assertNoError(t, r.client.Get(context.TODO(), types.NamespacedName{Name: expectedName}, reconcileClusterRoleBinding))
	assert.Equal(t, expectedName, reconcileClusterRoleBinding.RoleRef.Name)
}
