package argocdcommon

import (
	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TestNamespace  = "argocd"
	TestArgoCDName = "argocd"
)

type argoCDOpt func(*v1alpha1.ArgoCD)

func MakeTestArgoCD(opts ...argoCDOpt) *v1alpha1.ArgoCD {
	a := &v1alpha1.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestArgoCDName,
			Namespace: TestNamespace,
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}
