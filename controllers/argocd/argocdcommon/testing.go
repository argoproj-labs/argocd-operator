package argocdcommon

import (
	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	TestNamespace  = "argocd"
	TestArgoCDName = "argocd"
)

func MakeTestNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: TestNamespace,
		},
	}
}

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
