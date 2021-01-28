package openshift

import (
	"log"
	"os"
	"strings"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argocd"
	rbacv1 "k8s.io/api/rbac/v1"
)

func init() {
	argocd.Register(reconcilerHook)
	if err := argocd.VerifyRouteAPI(); err != nil {
		log.Fatal(err)
	}
}

func reconcilerHook(cr *argoprojv1alpha1.ArgoCD, v interface{}) error {
	switch o := v.(type) {
	case *rbacv1.ClusterRole:
		if o.ObjectMeta.Name == cr.ObjectMeta.Name+"-argocd-application-controller" {
			if allowedNamespace(cr.ObjectMeta.Namespace, os.Getenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES")) {
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
