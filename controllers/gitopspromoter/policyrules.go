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
	"fmt"
	"os"

	rbacv1 "k8s.io/api/rbac/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

// policyRuleConfig provides relevant data for Cluster Roles and Roles that need to be created
type policyRuleConfig struct {
	name        string
	roleRefName string
	policyRule  []rbacv1.PolicyRule
}

// buildPolicyRulesForControllerClusterRoles creates a list of configs for the ClusterRoles that are needed by the controller
// The default ClusterRule can be replaced by your own custom ClusterRole if the env variable is set
func buildPolicyRulesForControllerClusterRoles(compName string, cr *argoproj.ArgoCD) []policyRuleConfig {
	name := os.Getenv(common.GitOpsPromoterControllerClusterRoleEnvName)
	policyRule := []rbacv1.PolicyRule{}
	if name == "" {
		name = generatePromoterResourceNameWithNamespace(compName, cr)
		policyRule = buildPolicyRuleForControllerClusterRole()
	}

	return []policyRuleConfig{
		{
			name:        generatePromoterResourceNameWithNamespace(compName, cr),
			roleRefName: name,
			policyRule:  policyRule,
		},
	}
}

// buildPolicyRulesForControllerClusterRoles creates a list of configs for the ClusterRoles that are needed by the api server
// The default ClusterRule can be replaced by your own custom ClusterRole if the env variable is set
func buildPolicyRulesForAPIServerClusterRoles(compName string, cr *argoproj.ArgoCD) []policyRuleConfig {
	name := os.Getenv(common.GitOpsPromoterAPIServerClusterRoleEnvName)
	policyRule := []rbacv1.PolicyRule{}
	if name == "" {
		name = generatePromoterResourceNameWithNamespace(compName, cr)
		policyRule = buildPolicyRuleForAPIServerClusterRole()
	}

	return []policyRuleConfig{
		{
			name:        generatePromoterResourceNameWithNamespace(compName, cr),
			roleRefName: name,
			policyRule:  policyRule,
		},
		{
			name:        fmt.Sprintf("%s-%s", generatePromoterResourceNameWithNamespace(compName, cr), "promotionstrategydetails-viewer"),
			roleRefName: fmt.Sprintf("%s-%s", generatePromoterResourceNameWithNamespace(compName, cr), "promotionstrategydetails-viewer"),
			policyRule:  buildPolicyRuleForAPIServerPromotionStrategyDetailsViewer(),
		},
	}
}

// buildPolicyRuleForControllerClusterRole creates the default policy rule for the Controller's cluster role
func buildPolicyRuleForControllerClusterRole() []rbacv1.PolicyRule {
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

// buildPolicyRuleForAPIServerClusterRole builds the default policy rule for the API Server
func buildPolicyRuleForAPIServerClusterRole() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"promoter.argoproj.io",
			},
			Resources: []string{
				"promotionstrategies",
				"changetransferpolicies",
				"pullrequests",
				"commitstatuses",
				"argocdcommitstatuses",
				"gitcommitstatuses",
				"timedcommitstatuses",
				"webrequestcommitstatuses",
				"scheduledcommitstatuses",
				"gitrepositories",
				"scmproviders",
				"clusterscmproviders",
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
	}
}

// buildPolicyRuleForAPIServerPromotionStrategyDetailsViewer creates the PolicyRule for the view.promoter.argoproj.io resource which is managed by
// the API Server
func buildPolicyRuleForAPIServerPromotionStrategyDetailsViewer() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"view.promoter.argoproj.io",
			},
			Resources: []string{
				"promotionstrategydetails",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
	}
}
