package openshift

import (
	"os"
	"strings"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argocd"

	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

func init() {
	argocd.Register(reconcilerHook)
}

func reconcilerHook(cr *argoprojv1alpha1.ArgoCD, v interface{}) error {
	switch o := v.(type) {
	case *rbacv1.ClusterRole:
		if o.ObjectMeta.Name == argocd.GenerateUniqueResourceName("argocd-application-controller", cr) {
			if allowedNamespace(cr.ObjectMeta.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
				o.Rules = append(o.Rules, policyRulesForClusterConfig()...)
			}
		}
	case *appsv1.Deployment:
		if o.ObjectMeta.Name == cr.ObjectMeta.Name+"-redis" {
			o.Spec.Template.Spec.Containers[0].Args = append(getArgsForRedhatRedis(), o.Spec.Template.Spec.Containers[0].Args...)
		}
	case *rbacv1.RoleBinding:
		if o.ObjectMeta.Name == cr.ObjectMeta.Name+"-argocd-application-controller" {
			o.RoleRef.Kind = "ClusterRole"
			o.RoleRef.Name = "admin"
		}
	}
	return nil
}

// For OpenShift, we use a custom build of Redis provided by Red Hat
// which requires additional args in comparison to stock redis.
func getArgsForRedhatRedis() []string {
	return []string{
		"redis-server",
		"--protected-mode",
		"no",
	}
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
		}, {
			APIGroups: []string{
				"machine.openshift.io",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		}, {
			APIGroups: []string{
				"compliance.openshift.io",
			},
			Resources: []string{
				"scansettings",
				"compliancesuites",
				"compliancescans",
				"compliancecheckresults",
				"complianceremediations",
			},
			Verbs: []string{
				"get",
				"watch",
				"list",
			},
		},  {
			APIGroups: []string{
				"compliance.openshift.io",
			},
			Resources: []string{
				"scansettingbindings",
			},
			Verbs: []string{
				"*",
			},
		},
	}
}

func allowedNamespace(current string, namespaces string) bool {

	clusterConfigNamespaces := splitList(namespaces)
	if len(clusterConfigNamespaces) > 0 {
		if clusterConfigNamespaces[0] == "*" {
			return true
		}

		for _, n := range clusterConfigNamespaces {
			if n == current {
				return true
			}
		}
	}
	return false
}

func splitList(s string) []string {
	elems := strings.Split(s, ",")
	for i := range elems {
		elems[i] = strings.TrimSpace(elems[i])
	}
	return elems
}
