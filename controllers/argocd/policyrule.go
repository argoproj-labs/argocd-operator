package argocd

import (
	"github.com/argoproj-labs/argocd-operator/common"

	v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func policyRuleForDexServer() []v1.PolicyRule {

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
				"get",
				"list",
				"watch",
			},
		},
	}
}

func policyRuleForNotificationsController() []v1.PolicyRule {
	return []v1.PolicyRule{

		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
				"appprojects",
			},
			Verbs: []string{
				"get",
				"list",
				"patch",
				"update",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"configmaps",
				"secrets",
			},
			Verbs: []string{
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			ResourceNames: []string{
				"argocd-notifications-cm",
			},
			Resources: []string{
				"configmaps",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			ResourceNames: []string{
				"argocd-notifications-secret",
			},
			Resources: []string{
				"secrets",
			},
			Verbs: []string{
				"get",
			},
		},
	}
}

func policyRuleForServerApplicationSourceNamespaces() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"patch",
				"update",
				"watch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
			},
			Verbs: []string{
				"create",
			},
		},
	}
}

func getPolicyRuleList(client client.Client) []struct {
	name       string
	policyRule []v1.PolicyRule
} {
	return []struct {
		name       string
		policyRule []v1.PolicyRule
	}{
		{
			name:       common.ArgoCDApplicationControllerComponent,
			policyRule: policyRuleForApplicationController(),
		}, {
			name:       common.ArgoCDDexServerComponent,
			policyRule: policyRuleForDexServer(),
		}, {
			name:       common.ArgoCDServerComponent,
			policyRule: policyRuleForServer(),
		}, {
			name:       common.ArgoCDRedisHAComponent,
			policyRule: policyRuleForRedisHa(client),
		}, {
			name:       common.ArgoCDRedisComponent,
			policyRule: policyRuleForRedis(client),
		},
	}
}

func getPolicyRuleClusterRoleList() []struct {
	name       string
	policyRule []v1.PolicyRule
} {
	return []struct {
		name       string
		policyRule []v1.PolicyRule
	}{
		{
			name:       common.ArgoCDApplicationControllerComponent,
			policyRule: policyRuleForApplicationController(),
		}, {
			name:       common.ArgoCDServerComponent,
			policyRule: policyRuleForServerClusterRole(),
		},
	}
}
