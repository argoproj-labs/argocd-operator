package argocd

import v1 "k8s.io/api/rbac/v1"

func policyRuleForApplicationController() []v1.PolicyRule {

	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
				"services",
				"services/finalizers",
				"endpoints",
				"events",
				"configmaps",
				"secrets",
			},
			Verbs: []string{
				"*",
			},
		},

		{
			APIGroups: []string{
				"apps",
			},
			Resources: []string{
				"daemonsets",
				"deployments",
				"replicasets",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"list",
			},
		},
		{

			APIGroups: []string{
				"batch",
			},

			Resources: []string{
				"CronJob",
				"Job",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"apiGroups",
				"argoproj.io",
			},

			Resources: []string{
				"applications",
				"appprojects",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"list",
			},
		},
	}
}

func policyRuleForRedisHa() []v1.PolicyRule {

	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"endpoints",
			},
			Verbs: []string{
				"get",
			},
		},
	}
}

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

func policyRuleForServer() []v1.PolicyRule {
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
				"argoproj.io",
			},
			Resources: []string{
				"applications",
				"appprojects",
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
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"list",
			},
		},
	}
}
