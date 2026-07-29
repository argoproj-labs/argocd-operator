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

func ReconcilePromoterControllerClusterRole(client client.Client, compName string, cr *argoproj.ArgoCD) (*rbacv1.ClusterRole, error) {
	clusterRole := buildClusterRole(compName, cr)
	expectedPolicyRule := buildPolicyRuleForControllerClusterRole(compName, cr)

	exists := true
	if err := client.Get(context.Background(), types.NamespacedName{Name: clusterRole.Name}, clusterRole); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing promoter cluster role %s: %v", clusterRole.Name, err)
		}
		exists = false
	}

	if exists {
		// TODO: Need to add in some custom rbac functionality
		if !cr.Spec.Promoter.IsEnabled() {
			argoutil.LogResourceDeletion(log, clusterRole, fmt.Sprintf("promoter cluster role for component %s is being deleted due to being disabled", compName))
			if err := client.Delete(context.Background(), clusterRole); err != nil {
				return nil, fmt.Errorf("failed to delete promoter cluster role %s: %v", clusterRole.Name, err)
			}
			return clusterRole, nil
		}

		if !reflect.DeepEqual(clusterRole.Rules, expectedPolicyRule) {
			clusterRole.Rules = expectedPolicyRule
			argoutil.LogResourceUpdate(log, clusterRole, fmt.Sprintf("rules are not expected value for promoter cluster role for component %s", compName))
			if err := client.Update(context.Background(), clusterRole); err != nil {
				return nil, fmt.Errorf("failed to update promoter cluster role %s: %v", clusterRole.Name, err)
			}
			return clusterRole, nil
		}
		return clusterRole, nil
	}

	if !cr.Spec.Promoter.IsEnabled() {
		return clusterRole, nil
	}

	clusterRole.Rules = expectedPolicyRule
	argoutil.LogResourceCreation(log, clusterRole)
	if err := client.Create(context.Background(), clusterRole); err != nil {
		return nil, fmt.Errorf("failed to create promoter %s cluster role %s: %v", compName, clusterRole.Name, err)
	}
	return clusterRole, nil
}

func ReconcilePromoterAPIServerClusterRole(client client.Client, compName string, cr *argoproj.ArgoCD) (*rbacv1.ClusterRole, error) {
	return nil, nil
}

func buildClusterRole(compName string, cr *argoproj.ArgoCD) *rbacv1.ClusterRole {
	labels := buildLabelsForPromoterResources(compName, cr)
	labels[common.ArgoCDKeyName] = generatePromoterResourceNameWithNamespace(compName, cr)

	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   generatePromoterResourceNameWithNamespace(compName, cr),
			Labels: labels,
		},
	}
}

func buildPolicyRuleForControllerClusterRole(compName string, cr *argoproj.ArgoCD) []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"namespaces",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
				"events.k8s.io",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"update",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"promoter.argoproj.io",
			},
			Resources: []string{
				"argocdcommitstatuses",
				"gitcommitstatuses",
				"promotionstrategies",
				"revertcommits",
				"scheduledcommitstatuses",
				"timedcommitstatuses",
				"webrequestcommitstatuses",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"promoter.argoproj.io",
			},
			Resources: []string{
				"argocdcommitstatuses/finalizers",
				"changetransferpolicies/finalizers",
				"clusterscmproviders/finalizers",
				"gitcommitstatuses/finalizers",
				"gitrepositories/finalizers",
				"promotionstrategies/finalizers",
				"pullrequests/finalizers",
				"scheduledcommitstatuses/finalizers",
				"scmproviders/finalizers",
				"timedcommitstatuses/finalizers",
				"webrequestcommitstatuses/finalizers",
			},
			Verbs: []string{
				"update",
			},
		},
		{
			APIGroups: []string{
				"promoter.argoproj.io",
			},
			Resources: []string{
				"argocdcommitstatuses/status",
				"changetransferpolicies/status",
				"clusterscmproviders/status",
				"gitcommitstatuses/status",
				"gitrepositories/status",
				"promotionstrategies/status",
				"pullrequests/status",
				"scheduledcommitstatuses/status",
				"scmproviders/status",
				"timedcommitstatuses/status",
				"webrequestcommitstatuses/status",
			},
			Verbs: []string{
				"get",
				"patch",
				"update",
			},
		},
		{
			APIGroups: []string{
				"promoter.argoproj.io",
			},
			Resources: []string{
				"changetransferpolicies",
				"pullrequests",
			},
			Verbs: []string{
				"create",
				"delete",
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"promoter.argoproj.io",
			},
			Resources: []string{
				"clusterscmproviders",
				"gitrepositories",
				"scmproviders",
			},
			Verbs: []string{
				"get",
				"list",
				"update",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"promoter.argoproj.io",
			},
			Resources: []string{
				"clusterscmproviders",
				"gitrepositories",
				"scmproviders",
			},
			Verbs: []string{
				"get",
				"list",
				"update",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"promoter.argoproj.io",
			},
			Resources: []string{
				"commitstatuses",
			},
			Verbs: []string{
				"create",
				"delete",
				"get",
				"list",
				"patch",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"promoter.argoproj.io",
			},
			Resources: []string{
				"controllerconfigurations",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"promoter.argoproj.io",
			},
			Resources: []string{
				"controllerconfigurations/status",
			},
			Verbs: []string{
				"get",
				"patch",
				"update",
			},
		},
	}
}
