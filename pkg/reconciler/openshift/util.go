package openshift

import (
	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/apis"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace             = "argocd"
	testArgoCDName            = "argocd"
	testApplicationController = "argocd-application-controller"
)

type argoCDOpt func(*argoprojv1alpha1.ArgoCD)

// ReconcileArgoCD reconciles a ArgoCD object
type ReconcileArgoCD struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func makeTestReconciler(t *testing.T, objs ...runtime.Object) *ReconcileArgoCD {
	s := scheme.Scheme
	assert.NilError(t, apis.AddToScheme(s))

	cl := fake.NewFakeClientWithScheme(s, objs...)
	return &ReconcileArgoCD{
		client: cl,
		scheme: s,
	}
}

// policyRulesForClusterConfig defines rules for cluster config.
func policyRulesForClusterConfig() []v1.PolicyRule {
	return []v1.PolicyRule{
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

func makeTestArgoCDForClusterConfig(opts ...argoCDOpt) *argoprojv1alpha1.ArgoCD {
	allowedNamespaces := []string{testNamespace, "dummyNamespace"}
	a := &argoprojv1alpha1.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
		Spec: argoprojv1alpha1.ArgoCDSpec{
			ManagementScope: argoprojv1alpha1.ArgoCDScope{
				Namespaces: allowedNamespaces,
				Cluster:    true,
			},
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}
