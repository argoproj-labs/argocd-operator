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
			if cr.Spec.ManagementScope.Cluster && allowedNamespace(cr.Namespace,
				cr.Spec.ManagementScope.Namespaces) {
				o.Rules = append(o.Rules, policyRulesForClusterConfig()...)
			}
		}
	}
	return nil
}
