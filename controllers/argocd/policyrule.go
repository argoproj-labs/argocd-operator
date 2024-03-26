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
