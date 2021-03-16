package argocd

import (
	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	v1 "k8s.io/api/rbac/v1"
)

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
				"cronjobs",
				"jobs",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
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
	}
}

func policyRuleForRedisHa(cr *argoprojv1alpha1.ArgoCD) []v1.PolicyRule {

	rules := []v1.PolicyRule{
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

	if err := applyReconcilerHook(cr, &rules, "policyRuleForRedisHa"); err != nil {
		log.Error(err, "error from reconcile hook")
	}

	return rules
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

func policyRuleForApplicationControllerClusterRole() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			NonResourceURLs: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"list",
			},
		},
	}
}

func policyRuleForServerClusterRole() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
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
				"list",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
				"pods/log",
			},
			Verbs: []string{
				"get",
			},
		},
	}
}
