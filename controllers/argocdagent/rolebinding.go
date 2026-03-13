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
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// ReconcilePrincipalRoleBinding reconciles a RoleBinding for the ArgoCD agent principal component.
// This function handles the creation, update, and deletion of RoleBindings based on the principal's enabled state.
func ReconcilePrincipalRoleBinding(client client.Client, compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {
	roleBinding := buildRoleBinding(compName, cr)
	expectedSubjects := buildSubjects(sa, cr)
	expectedRoleRef := buildRoleRef(generateAgentResourceName(cr.Name, compName), "Role")

	// Check if the RoleBinding already exists
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, roleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal roleBinding %s in namespace %s: %v", roleBinding.Name, roleBinding.Namespace, err)
		}
		exists = false
	}

	// If ClusterRoleBinding exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, roleBinding, "principal roleBinding is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), roleBinding); err != nil {
				return fmt.Errorf("failed to delete principal roleBinding %s: %v", roleBinding.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(roleBinding.Subjects, expectedSubjects) ||
			!reflect.DeepEqual(roleBinding.RoleRef, expectedRoleRef) {

			roleBinding.Subjects = expectedSubjects
			roleBinding.RoleRef = expectedRoleRef

			argoutil.LogResourceUpdate(log, roleBinding, "principal roleBinding is being updated")
			if err := client.Update(context.TODO(), roleBinding); err != nil {
				return fmt.Errorf("failed to update principal roleBinding %s: %v", roleBinding.Name, err)
			}
		}
		return nil
	}

	// If RoleBinding doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
		return nil
	}

	// Create a fresh RoleBinding object for creation to avoid resourceVersion issues
	newRoleBinding := buildRoleBinding(compName, cr)
	if err := controllerutil.SetControllerReference(cr, newRoleBinding, scheme); err != nil {
		return fmt.Errorf("failed to set ArgoCD CR %s as owner for roleBinding %s: %v", cr.Name, newRoleBinding.Name, err)
	}

	newRoleBinding.Subjects = expectedSubjects
	newRoleBinding.RoleRef = expectedRoleRef

	argoutil.LogResourceCreation(log, newRoleBinding)
	if err := client.Create(context.TODO(), newRoleBinding); err != nil {
		return fmt.Errorf("failed to create principal roleBinding %s: %v", newRoleBinding.Name, err)
	}
	return nil
}

// ReconcilePrincipalClusterRoleBinding reconciles a ClusterRoleBinding for the ArgoCD agent principal component.
// This function handles the creation, update, and deletion of ClusterRoleBindings based on the principal's enabled state.
func ReconcilePrincipalClusterRoleBinding(client client.Client, compName string, sa *corev1.ServiceAccount, cr *argoproj.ArgoCD, scheme *runtime.Scheme) error {
	clusterRoleBinding := buildClusterRoleBinding(compName, cr)
	expectedSubjects := buildSubjects(sa, cr)
	expectedRoleRef := buildRoleRef(generateAgentResourceName(cr.Name+"-"+cr.Namespace, compName), "ClusterRole")

	allowed := argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace)

	// Check if the ClusterRoleBinding already exists
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: clusterRoleBinding.Name}, clusterRoleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get existing principal clusterRoleBinding %s: %v", clusterRoleBinding.Name, err)
		}
		exists = false
	}

	// If ClusterRoleBinding exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() || !allowed {
			argoutil.LogResourceDeletion(log, clusterRoleBinding, "principal clusterRoleBinding is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), clusterRoleBinding); err != nil {
				return fmt.Errorf("failed to delete principal clusterRoleBinding %s: %v", clusterRoleBinding.Name, err)
			}
			return nil
		}

		if !reflect.DeepEqual(clusterRoleBinding.Subjects, expectedSubjects) ||
			!reflect.DeepEqual(clusterRoleBinding.RoleRef, expectedRoleRef) {

			clusterRoleBinding.Subjects = expectedSubjects
			clusterRoleBinding.RoleRef = expectedRoleRef

			argoutil.LogResourceUpdate(log, clusterRoleBinding, "principal clusterRoleBinding is being updated")
			if err := client.Update(context.TODO(), clusterRoleBinding); err != nil {
				return fmt.Errorf("failed to update principal clusterRoleBinding %s: %v", clusterRoleBinding.Name, err)
			}
		}
		return nil
	}

	// If ClusterRoleBinding doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() || !allowed {
		return nil
	}

	// Create a fresh ClusterRoleBinding object for creation to avoid resourceVersion issues
	newClusterRoleBinding := buildClusterRoleBinding(compName, cr)
	newClusterRoleBinding.Subjects = expectedSubjects
	newClusterRoleBinding.RoleRef = expectedRoleRef

	argoutil.LogResourceCreation(log, newClusterRoleBinding)
	if err := client.Create(context.TODO(), newClusterRoleBinding); err != nil {
		return fmt.Errorf("failed to create principal clusterRoleBinding %s: %v", newClusterRoleBinding.Name, err)
	}
	return nil
}

func buildSubjects(sa *corev1.ServiceAccount, cr *argoproj.ArgoCD) []v1.Subject {
	return []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: cr.Namespace,
		},
	}
}

func buildRoleRef(name, kind string) v1.RoleRef {
	return v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     kind,
		Name:     name,
	}
}

func buildRoleBinding(compName string, cr *argoproj.ArgoCD) *v1.RoleBinding {
	return &v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, compName),
			Labels:    buildLabelsForAgentPrincipal(cr.Name, compName),
			Namespace: cr.Namespace,
		},
	}
}

func buildClusterRoleBinding(compName string, cr *argoproj.ArgoCD) *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, compName),
			Labels: buildLabelsForAgentPrincipal(cr.Name, compName),
		},
	}
}
