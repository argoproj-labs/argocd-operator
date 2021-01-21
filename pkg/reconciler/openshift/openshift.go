package openshift

import (
	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/argoproj-labs/argocd-operator/pkg/controller/argocd"
)

func init() {
	argocd.Register(reconcilerHook)
}

func reconcilerHook(cr *argoprojv1alpha1.ArgoCD, v interface{}) error {
	switch o := v.(type) {
	case *rbacv1.ClusterRole:
		if o.ObjectMeta.Name == cr.ObjectMeta.Name+"-argocd-application-controller" {
			if len(cr.Spec.ManagementScope.ClusterConfigNamespaces) > 0 {
				o.Rules = append(o.Rules, policyRulesForClusterConfig()...)
			}
		}
	}
	return nil
}

// policyRulesForClusterConfig defines rules for cluster config.
func policyRulesForClusterConfig() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{
				"operators.coreos.com",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"operator.openshift.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"user.openshift.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"config.openshift.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
		{
			APIGroups: []string{
				"console.openshift.io",
			},
			Resources: []string{
				"*",
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
				"namespaces",
				"persistentvolumeclaims",
				"persistentvolumes",
				"configmaps",
			},
			Verbs: []string{
				"*",
			},
		}, {
			APIGroups: []string{
				"rbac.authorization.k8s.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		}, {
			APIGroups: []string{
				"storage.k8s.io",
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

func allowedNamespace(current string, configuredList []string) bool {
	if len(configuredList) > 0 {
		if configuredList[0] == "*" {
			return true
		}

		for _, n := range configuredList {
			if n == current {
				return true
			}
		}
	}
	return false
}
