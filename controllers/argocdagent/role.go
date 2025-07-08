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

// ReconcilePrincipalRole manages the lifecycle of a Role resource for the ArgoCD agent principal.
// This function creates, updates, or deletes the Role based on the principal's enabled status.
func ReconcilePrincipalRole(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) (*v1.Role, error) {
	role := buildRole(compName, cr)
	expectedPolicyRule := buildPolicyRuleForRole()

	// Check if the Role already exists
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: role.Namespace}, role); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing principal role %s in namespace %s: %v", role.Name, role.Namespace, err)
		}
		exists = false
	}

	// If Role exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, role, "principal role is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), role); err != nil {
				return role, fmt.Errorf("failed to delete principal role %s: %w", role.Name, err)
			}
			return role, nil
		}

		if !reflect.DeepEqual(expectedPolicyRule, role.Rules) {
			role.Rules = expectedPolicyRule
			argoutil.LogResourceUpdate(log, role, "principal role rules are being updated")
			if err := client.Update(context.TODO(), role); err != nil {
				return nil, fmt.Errorf("failed to update principal role %s: %w", role.Name, err)
			}
		}
		return role, nil
	}

	// If Role doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
		return role, nil
	}

	if err := controllerutil.SetControllerReference(cr, role, scheme); err != nil {
		return nil, fmt.Errorf("failed to set ArgoCD CR %s as owner for role %s: %v", cr.Name, role.Name, err)
	}

	role.Rules = expectedPolicyRule

	argoutil.LogResourceCreation(log, role)
	if err := client.Create(context.TODO(), role); err != nil {
		return nil, fmt.Errorf("failed to create principal role %s: %v", role.Name, err)
	}
	return role, nil
}

// ReconcilePrincipalClusterRoles manages the lifecycle of a ClusterRole resource for the ArgoCD agent principal.
// This function creates, updates, or deletes the ClusterRole based on the principal's enabled status.
func ReconcilePrincipalClusterRoles(client client.Client, compName string, cr *argoproj.ArgoCD, scheme *runtime.Scheme) (*v1.ClusterRole, error) {
	clusterRole := buildClusterRole(compName, cr)
	expectedPolicyRule := buildPolicyRuleForClusterRole()

	// Check if the ClusterRole already exists
	exists := true
	if err := client.Get(context.TODO(), types.NamespacedName{Name: clusterRole.Name}, clusterRole); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing principal clusterRole %s: %v", clusterRole.Name, err)
		}
		exists = false
	}

	// If ClusterRole exists, handle updates or deletion
	if exists {
		if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
			argoutil.LogResourceDeletion(log, clusterRole, "principal clusterRole is being deleted as principal is disabled")
			if err := client.Delete(context.TODO(), clusterRole); err != nil {
				return clusterRole, fmt.Errorf("failed to delete principal clusterRole %s: %v", clusterRole.Name, err)
			}
			return clusterRole, nil
		}

		if !reflect.DeepEqual(expectedPolicyRule, clusterRole.Rules) {
			clusterRole.Rules = expectedPolicyRule
			argoutil.LogResourceUpdate(log, clusterRole, "principal clusterRole rules are being updated")
			if err := client.Update(context.TODO(), clusterRole); err != nil {
				return nil, fmt.Errorf("failed to update principal clusterRole %s: %v", clusterRole.Name, err)
			}
		}
		return clusterRole, nil
	}

	// If ClusterRole doesn't exist and principal is disabled, nothing to do
	if cr.Spec.ArgoCDAgent == nil || cr.Spec.ArgoCDAgent.Principal == nil || !cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
		return clusterRole, nil
	}

	clusterRole.Rules = expectedPolicyRule

	argoutil.LogResourceCreation(log, clusterRole)
	if err := client.Create(context.TODO(), clusterRole); err != nil {
		return nil, fmt.Errorf("failed to create principal clusterRole %s: %v", clusterRole.Name, err)
	}
	return clusterRole, nil
}

func buildRole(compName string, cr *argoproj.ArgoCD) *v1.Role {
	return &v1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateAgentResourceName(cr.Name, compName),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForAgentPrincipal(cr.Name, compName),
		},
	}
}

func buildClusterRole(compName string, cr *argoproj.ArgoCD) *v1.ClusterRole {
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generateAgentResourceName(cr.Name+"-"+cr.Namespace, compName),
			Labels: buildLabelsForAgentPrincipal(cr.Name, compName),
		},
	}
}

// buildPolicyRuleForRole defines the namespace-scoped permissions for the ArgoCD agent principal.
// Grants access to:
// - secrets and configmaps: full CRUD operations for configuration management
// - events: create and list permissions for logging and monitoring
func buildPolicyRuleForRole() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		}, {
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"event",
			},
			Verbs: []string{
				"create",
				"list",
			},
		},
	}
}

// buildPolicyRuleForClusterRole defines the cluster-scoped permissions for the ArgoCD agent principal.
// Grants access to:
// - ArgoCD resources (applications, appprojects, applicationsets): full CRUD operations
// - namespaces: read and create permissions for managing application deployments across namespaces
func buildPolicyRuleForClusterRole() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
				"appprojects",
				"applicationsets",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"delete",
				"patch",
			},
		}, {
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"namespaces",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
			},
		},
	}
}
