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

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// ReconcilePromoterControllerClusterRoles reconciles the ClusterRoles for the Controller
func ReconcilePromoterControllerClusterRoles(client client.Client, compName string, cr *argoproj.ArgoCD) ([]*rbacv1.ClusterRole, error) {
	clusterRolesToReconcile := buildPolicyRulesForControllerClusterRoles(compName, cr)
	reconciledClusterRoles := []*rbacv1.ClusterRole{}

	for _, clusterRole := range clusterRolesToReconcile {
		// If PolicyRule is empty that means user wants a custom already existing ClusterRole so it can be skipped
		if !reflect.DeepEqual(clusterRole.policyRule, []rbacv1.PolicyRule{}) {
			resultClusterRole, err := ReconcilePromoterClusterRole(client, compName, clusterRole.name, clusterRole.policyRule, cr, true)
			if err != nil {
				return nil, err
			}
			reconciledClusterRoles = append(reconciledClusterRoles, resultClusterRole)
		} else {
			// Delete already existing generated cluster role if it exists and a custom cluster role wants to be used
			_, err := ReconcilePromoterClusterRole(client, compName, generatePromoterResourceName(compName, cr), []rbacv1.PolicyRule{}, cr, false)
			if err != nil {
				return nil, err
			}
		}
	}

	return reconciledClusterRoles, nil
}

// ReconcilePromoterAPIServerClusterRoles reconciles the ClusterRoles for the API Server
func ReconcilePromoterAPIServerClusterRoles(client client.Client, compName string, cr *argoproj.ArgoCD) ([]*rbacv1.ClusterRole, error) {
	clusterRolesToReconcile := buildPolicyRulesForAPIServerClusterRoles(compName, cr)
	reconciledClusterRoles := []*rbacv1.ClusterRole{}

	enabled := cr.Spec.Promoter == nil || cr.Spec.Promoter.APIServer.IsEnabled()
	for _, clusterRole := range clusterRolesToReconcile {
		// If PolicyRule is empty that means user wants a custom already existing ClusterRole so it can be skipped
		if !reflect.DeepEqual(clusterRole.policyRule, []rbacv1.PolicyRule{}) {
			resultClusterRole, err := ReconcilePromoterClusterRole(client, compName, clusterRole.name, clusterRole.policyRule, cr, enabled)
			if err != nil {
				return nil, err
			}
			reconciledClusterRoles = append(reconciledClusterRoles, resultClusterRole)
		} else {
			// Delete already existing generated cluster role if it exists and a custom cluster role wants to be used
			_, err := ReconcilePromoterClusterRole(client, compName, generatePromoterResourceName(compName, cr), []rbacv1.PolicyRule{}, cr, false)
			if err != nil {
				return nil, err
			}
		}
	}

	return reconciledClusterRoles, nil
}

// ReconcilePromoterClusterRole is a generic ClusterRole reconcilation function that reconciles a ClusterRole based on the provided PolicyRule
func ReconcilePromoterClusterRole(client client.Client, compName, name string, expectedPolicyRule []rbacv1.PolicyRule, cr *argoproj.ArgoCD, enabled bool) (*rbacv1.ClusterRole, error) {
	clusterRole := buildClusterRole(compName, name, cr)

	allowed := argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace)

	exists := true
	if err := client.Get(context.Background(), types.NamespacedName{Name: clusterRole.Name}, clusterRole); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing promoter cluster role %s: %v", clusterRole.Name, err)
		}
		exists = false
	}

	if exists {
		if !cr.Spec.Promoter.IsEnabled() || !enabled || !allowed {
			argoutil.LogResourceDeletion(log, clusterRole, fmt.Sprintf("promoter cluster role, %s, is being deleted due to being disabled", clusterRole.Name))
			if err := client.Delete(context.Background(), clusterRole); err != nil {
				return nil, fmt.Errorf("failed to delete promoter cluster role %s: %v", clusterRole.Name, err)
			}
			return clusterRole, nil
		}

		if !reflect.DeepEqual(clusterRole.Rules, expectedPolicyRule) {
			clusterRole.Rules = expectedPolicyRule
			argoutil.LogResourceUpdate(log, clusterRole, fmt.Sprintf("rules are not expected value for promoter cluster role: %s", clusterRole.Name))
			if err := client.Update(context.Background(), clusterRole); err != nil {
				return nil, fmt.Errorf("failed to update promoter cluster role %s: %v", clusterRole.Name, err)
			}
			return clusterRole, nil
		}
		return clusterRole, nil
	}

	if !cr.Spec.Promoter.IsEnabled() || !enabled || !allowed {
		return clusterRole, nil
	}

	clusterRole.Rules = expectedPolicyRule
	argoutil.LogResourceCreation(log, clusterRole)
	if err := client.Create(context.Background(), clusterRole); err != nil {
		return nil, fmt.Errorf("failed to create promoter cluster role %s: %v", clusterRole.Name, err)
	}
	return clusterRole, nil
}

// buildClusterRole creates a ClusterRole object with metadata
func buildClusterRole(compName, name string, cr *argoproj.ArgoCD) *rbacv1.ClusterRole {
	labels := buildLabelsForPromoterResources(compName, cr)
	labels[common.ArgoCDKeyName] = argoutil.TruncateWithHash(name, argoutil.GetMaxLabelLength())

	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}
