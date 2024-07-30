package argocd

import (
	"fmt"

	"golang.org/x/mod/semver"

	"github.com/argoproj-labs/argocd-operator/common"

	v1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func policyRuleForApplicationController() []v1.PolicyRule {

	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
	}
}

func policyRuleForApplicationControllerView() []v1.PolicyRule {

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
		}, {
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

func policyRuleForApplicationControllerAdmin() []v1.PolicyRule {
	return []v1.PolicyRule{}
}

func policyRuleForRedis(client client.Client) []v1.PolicyRule {
	rules := []v1.PolicyRule{
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
			},
		},
	}

	// Need additional policy rules if we are running on openshift, else the stateful set won't have the right
	// permissions to start
	rules = appendOpenShiftNonRootSCC(rules, client)

	return rules
}

func policyRuleForRedisHa(client client.Client) []v1.PolicyRule {

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
			},
		},
	}

	// Need additional policy rules if we are running on openshift, else the stateful set won't have the right
	// permissions to start
	rules = appendOpenShiftNonRootSCC(rules, client)

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
				"*",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"get",
				"patch",
				"delete",
			},
		},
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
				"applicationsets",
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
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
				"cronjobs",
				"cronjobs/finalizers",
			},
			Verbs: []string{
				"create",
				"update",
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
				"cronjobs",
				"cronjobs/finalizers",
			},
			Verbs: []string{
				"create",
				"update",
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
				"delete",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applications",
				"applicationsets",
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
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"list",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
				"cronjobs",
				"cronjobs/finalizers",
			},
			Verbs: []string{
				"create",
				"update",
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
		}, {
			name:       common.ArgoCDApplicationControllerComponentView,
			policyRule: policyRuleForApplicationControllerView(),
		}, {
			name:       common.ArgoCDApplicationControllerComponentAdmin,
			policyRule: policyRuleForApplicationControllerAdmin(),
		},
	}
}

func appendOpenShiftNonRootSCC(rules []v1.PolicyRule, client client.Client) []v1.PolicyRule {
	if IsVersionAPIAvailable() {
		// Starting with OpenShift 4.11, we need to use the resource name "nonroot-v2" instead of "nonroot"
		resourceName := "nonroot"
		version, err := getClusterVersion(client)
		if err != nil {
			log.Error(err, "couldn't get OpenShift version")
		}
		if version == "" || semver.Compare(fmt.Sprintf("v%s", version), "v4.10.999") > 0 {
			resourceName = "nonroot-v2"
		}
		orules := v1.PolicyRule{
			APIGroups: []string{
				"security.openshift.io",
			},
			ResourceNames: []string{
				resourceName,
			},
			Resources: []string{
				"securitycontextconstraints",
			},
			Verbs: []string{
				"use",
			},
		}
		rules = append(rules, orules)
	}
	return rules
}

func policyRuleForApplicationSetController() []v1.PolicyRule {
	return []v1.PolicyRule{
		// ApplicationSet
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applications",
				"applicationsets",
				"applicationsets/finalizers",
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
		// ApplicationSet Status
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"applicationsets/status",
			},
			Verbs: []string{
				"get",
				"patch",
				"update",
			},
		},
		// AppProjects
		{
			APIGroups: []string{"argoproj.io"},
			Resources: []string{
				"appprojects",
			},
			Verbs: []string{
				"get",
			},
		},

		// Events
		{
			APIGroups: []string{""},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"patch",
				"watch",
			},
		},

		// ConfigMaps
		{
			APIGroups: []string{""},
			Resources: []string{
				"configmaps",
			},
			Verbs: []string{
				"create",
				"update",
				"delete",
				"get",
				"list",
				"patch",
				"watch",
			},
		},

		// Secrets
		{
			APIGroups: []string{""},
			Resources: []string{
				"secrets",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},

		// Deployments
		{
			APIGroups: []string{"apps", "extensions"},
			Resources: []string{
				"deployments",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},

		// leases
		{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{
				"leases",
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
	}
}

func policyRuleForServerApplicationSetSourceNamespaces() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"applicationsets",
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
	}
}
