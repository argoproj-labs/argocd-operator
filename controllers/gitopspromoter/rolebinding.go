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
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func ReconcilePromoterControllerClusterRoleBinding(client client.Client, compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD) (*rbacv1.ClusterRoleBinding, error) {
	clusterRoleBinding := buildClusterRoleBinding(compName, cr)
	expectedSubjects := buildSubject(sa)
	expectedRoleRef := buildRoleRef(generatePromoterResourceNameWithNamespace(compName, cr))

	exists := true
	if err := client.Get(context.Background(), types.NamespacedName{Name: clusterRoleBinding.Name}, clusterRoleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing promoter cluster role binding %s: %v", clusterRoleBinding.Name, err)
		}
		exists = false
	}

	if exists {
		if !cr.Spec.Promoter.IsEnabled() {
			argoutil.LogResourceDeletion(log, clusterRoleBinding, fmt.Sprintf("promoter cluster role binding for component %s is being deleted due to being disabled", compName))
			if err := client.Delete(context.Background(), clusterRoleBinding); err != nil {
				return nil, fmt.Errorf("failed to delete promoter cluster role %s: %v", clusterRoleBinding.Name, err)
			}
			return clusterRoleBinding, nil
		}

		if !reflect.DeepEqual(clusterRoleBinding.Subjects, expectedSubjects) ||
			!reflect.DeepEqual(clusterRoleBinding.RoleRef, expectedRoleRef) {

			clusterRoleBinding.Subjects = expectedSubjects
			clusterRoleBinding.RoleRef = expectedRoleRef

			argoutil.LogResourceUpdate(log, clusterRoleBinding, fmt.Sprintf("promoter cluster role binding for component %s has the wrong subject or role ref", compName))
			if err := client.Update(context.Background(), clusterRoleBinding); err != nil {
				return nil, fmt.Errorf("failed to update promoter cluster role %s: %v", clusterRoleBinding.Name, err)
			}
			return clusterRoleBinding, nil
		}
		return clusterRoleBinding, nil
	}

	if !cr.Spec.Promoter.IsEnabled() {
		return clusterRoleBinding, nil
	}

	// create a new ClusterRoleBinding to avoid resourceVersion issues
	newClusterRoleBinding := buildClusterRoleBinding(compName, cr)
	newClusterRoleBinding.Subjects = buildSubject(sa)
	newClusterRoleBinding.RoleRef = buildRoleRef(generatePromoterResourceNameWithNamespace(compName, cr))

	argoutil.LogResourceCreation(log, newClusterRoleBinding)
	if err := client.Create(context.Background(), newClusterRoleBinding); err != nil {
		return nil, fmt.Errorf("failed to create promoter %s cluster role binding %s: %v", compName, newClusterRoleBinding.Name, err)
	}
	return newClusterRoleBinding, nil
}

func buildClusterRoleBinding(compName string, cr *argoproj.ArgoCD) *rbacv1.ClusterRoleBinding {
	labels := buildLabelsForPromoterResources(compName, cr)
	labels[common.ArgoCDKeyName] = generatePromoterResourceNameWithNamespace(compName, cr)

	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generatePromoterResourceNameWithNamespace(compName, cr),
			Labels: labels,
		},
	}
}

func buildSubject(sa *corev1.ServiceAccount) []rbacv1.Subject {
	return []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}
}

func buildRoleRef(name string) rbacv1.RoleRef {
	return rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     name,
	}
}
