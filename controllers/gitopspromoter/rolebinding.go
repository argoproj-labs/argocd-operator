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

func ReconcilePromoterControllerClusterRoleBindings(client client.Client, compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD) ([]*rbacv1.ClusterRoleBinding, error) {
	clusterRoleBindingsToReconcile := buildPolicyRulesForControllerClusterRoles(compName, cr)
	reconciledClusterRoleBindings := []*rbacv1.ClusterRoleBinding{}

	for _, clusterRole := range clusterRoleBindingsToReconcile {
		resultClusterRoleBinding, err := ReconcilePromoterClusterRoleBinding(client, compName, clusterRole.name, clusterRole.name, sa, cr, true)
		if err != nil {
			return nil, err
		}
		reconciledClusterRoleBindings = append(reconciledClusterRoleBindings, resultClusterRoleBinding)
	}

	return reconciledClusterRoleBindings, nil
}

func ReconcilePromoterAPIServerClusterRoleBindings(client client.Client, compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD) ([]*rbacv1.ClusterRoleBinding, error) {
	clusterRoleBindingsToReconcile := buildPolicyRulesForAPIServerClusterRoles(compName, cr)
	reconciledClusterRoleBindings := []*rbacv1.ClusterRoleBinding{}

	enabled := cr.Spec.Promoter == nil || cr.Spec.Promoter.APIServer.IsEnabled()
	for _, clusterRole := range clusterRoleBindingsToReconcile {
		resultClusterRoleBinding, err := ReconcilePromoterClusterRoleBinding(client, compName, clusterRole.name, clusterRole.name, sa, cr, enabled)
		if err != nil {
			return nil, err
		}
		reconciledClusterRoleBindings = append(reconciledClusterRoleBindings, resultClusterRoleBinding)
	}

	// The Promoter API server requires an extra cluster role for the kubernetes system:auth-delegator role
	bindingName := fmt.Sprintf("%s-%s", generatePromoterResourceNameWithNamespace(compName, cr), "auth-delegator")
	resultClusterRoleBinding, err := ReconcilePromoterClusterRoleBinding(client, compName, bindingName, "system:auth-delegator", sa, cr, enabled)
	if err != nil {
		return nil, err
	}
	reconciledClusterRoleBindings = append(reconciledClusterRoleBindings, resultClusterRoleBinding)

	return reconciledClusterRoleBindings, nil
}

func ReconcilePromoterAPIServerRoleBindings(client client.Client, compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD) ([]*rbacv1.RoleBinding, error) {
	reconciledRoleBindings := []*rbacv1.RoleBinding{}

	enabled := cr.Spec.Promoter == nil || cr.Spec.Promoter.APIServer.IsEnabled()
	// As of right now the Promoter requires a RoleBinding to the extension-apiserver-authentication-reader role in the kube-system namespace
	// Because of this a copy of the CR is needed with the kube-system namespace
	crCopy := cr.DeepCopy()
	crCopy.SetNamespace("kube-system")
	bindingName := fmt.Sprintf("%s-%s", generatePromoterResourceNameWithNamespace(compName, cr), "extension-auth-reader")
	ReconcilePromoterRoleBinding(client, compName, bindingName, "extension-apiserver-authentication-reader", sa, crCopy, enabled)

	return reconciledRoleBindings, nil
}

func ReconcilePromoterClusterRoleBinding(client client.Client, compName, bindingName, roleRefName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD, enabled bool) (*rbacv1.ClusterRoleBinding, error) {
	clusterRoleBinding := buildClusterRoleBinding(compName, bindingName, cr)
	expectedSubjects := buildSubject(sa)
	expectedRoleRef := buildRoleRef(roleRefName, "ClusterRole")

	exists := true
	if err := client.Get(context.Background(), types.NamespacedName{Name: clusterRoleBinding.Name}, clusterRoleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing promoter cluster role binding %s: %v", clusterRoleBinding.Name, err)
		}
		exists = false
	}

	if exists {
		if !cr.Spec.Promoter.IsEnabled() || !enabled {
			argoutil.LogResourceDeletion(log, clusterRoleBinding, fmt.Sprintf("promoter cluster role binding %s is being deleted due to being disabled", bindingName))
			if err := client.Delete(context.Background(), clusterRoleBinding); err != nil {
				return nil, fmt.Errorf("failed to delete promoter cluster role %s: %v", clusterRoleBinding.Name, err)
			}
			return clusterRoleBinding, nil
		}

		if !reflect.DeepEqual(clusterRoleBinding.Subjects, expectedSubjects) ||
			!reflect.DeepEqual(clusterRoleBinding.RoleRef, expectedRoleRef) {

			clusterRoleBinding.Subjects = expectedSubjects
			clusterRoleBinding.RoleRef = expectedRoleRef

			argoutil.LogResourceUpdate(log, clusterRoleBinding, fmt.Sprintf("promoter cluster role binding %s has the wrong subject or role ref", bindingName))
			if err := client.Update(context.Background(), clusterRoleBinding); err != nil {
				return nil, fmt.Errorf("failed to update promoter cluster role %s: %v", clusterRoleBinding.Name, err)
			}
			return clusterRoleBinding, nil
		}
		return clusterRoleBinding, nil
	}

	if !cr.Spec.Promoter.IsEnabled() || !enabled {
		return clusterRoleBinding, nil
	}

	// create a new ClusterRoleBinding to avoid resourceVersion issues
	newClusterRoleBinding := buildClusterRoleBinding(compName, bindingName, cr)
	newClusterRoleBinding.Subjects = buildSubject(sa)
	newClusterRoleBinding.RoleRef = buildRoleRef(roleRefName, "ClusterRole")

	argoutil.LogResourceCreation(log, newClusterRoleBinding)
	if err := client.Create(context.Background(), newClusterRoleBinding); err != nil {
		return nil, fmt.Errorf("failed to create promoter cluster role binding %s: %v", newClusterRoleBinding.Name, err)
	}
	return newClusterRoleBinding, nil
}

func ReconcilePromoterRoleBinding(client client.Client, compName, bindingName, roleRefName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD, enabled bool) (*rbacv1.RoleBinding, error) {
	roleBinding := buildRoleBinding(compName, bindingName, cr)
	expectedSubjects := buildSubject(sa)
	expectedRoleRef := buildRoleRef(roleRefName, "Role")

	exists := true
	if err := argoutil.FetchObject(client, cr.Namespace, cr.Name, roleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing promoter cluster role binding %s: %v", roleBinding.Name, err)
		}
		exists = false
	}

	if exists {
		if !cr.Spec.Promoter.IsEnabled() || !enabled {
			argoutil.LogResourceDeletion(log, roleBinding, fmt.Sprintf("promoter cluster role binding %s is being deleted due to being disabled", bindingName))
			if err := client.Delete(context.Background(), roleBinding); err != nil {
				return nil, fmt.Errorf("failed to delete promoter cluster role %s: %v", roleBinding.Name, err)
			}
			return roleBinding, nil
		}

		if !reflect.DeepEqual(roleBinding.Subjects, expectedSubjects) ||
			!reflect.DeepEqual(roleBinding.RoleRef, expectedRoleRef) {

			roleBinding.Subjects = expectedSubjects
			roleBinding.RoleRef = expectedRoleRef

			argoutil.LogResourceUpdate(log, roleBinding, fmt.Sprintf("promoter cluster role binding %s has the wrong subject or role ref", bindingName))
			if err := client.Update(context.Background(), roleBinding); err != nil {
				return nil, fmt.Errorf("failed to update promoter cluster role %s: %v", roleBinding.Name, err)
			}
			return roleBinding, nil
		}
		return roleBinding, nil
	}

	if !cr.Spec.Promoter.IsEnabled() || !enabled {
		return roleBinding, nil
	}

	// create a new RoleBinding to avoid resourceVersion issues
	newRoleBinding := buildRoleBinding(compName, bindingName, cr)
	newRoleBinding.Subjects = buildSubject(sa)
	newRoleBinding.RoleRef = buildRoleRef(roleRefName, "Role")

	argoutil.LogResourceCreation(log, newRoleBinding)
	if err := client.Create(context.Background(), newRoleBinding); err != nil {
		return nil, fmt.Errorf("failed to create promoter cluster role binding %s: %v", newRoleBinding.Name, err)
	}
	return newRoleBinding, nil
}

func buildClusterRoleBinding(compName, name string, cr *argoproj.ArgoCD) *rbacv1.ClusterRoleBinding {
	labels := buildLabelsForPromoterResources(compName, cr)
	labels[common.ArgoCDKeyName] = argoutil.TruncateWithHash(name, argoutil.GetMaxLabelLength())

	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func buildRoleBinding(compName, name string, cr *argoproj.ArgoCD) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			Labels:    buildLabelsForPromoterResources(compName, cr),
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

func buildRoleRef(name, refType string) rbacv1.RoleRef {
	return rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     refType,
		Name:     name,
	}
}
