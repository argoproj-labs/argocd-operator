package argocdcommon

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

const (
	TestNamespace  = "argocd"
	TestArgoCDName = "argocd"
	TestUID        = "test-uid"
)

var (
	TestKey     = "test"
	TestVal     = "test"
	TestRoleRef = rbacv1.RoleRef{
		Kind:     common.RoleKind,
		Name:     TestArgoCDName,
		APIGroup: rbacv1.GroupName,
	}

	TestSubjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      TestArgoCDName,
			Namespace: TestNamespace,
		},
	}

	TestKVP = map[string]string{
		TestKey: TestVal,
	}
)

func MakeTestNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: TestNamespace,
		},
	}
}
func MakeTestServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TestArgoCDName,
			Namespace: TestNamespace,
		},
		Secrets: []corev1.ObjectReference{},
	}
}

type argoCDOpt func(*argoproj.ArgoCD)

func MakeTestArgoCD(opts ...argoCDOpt) *argoproj.ArgoCD {
	a := &argoproj.ArgoCD{
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
