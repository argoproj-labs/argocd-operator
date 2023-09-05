package notifications

import (
	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	testNamespace  = "argocd"
	testArgoCDName = "argocd"
	// testApplicationController = "argocd-application-controller"
)

type argoCDOpt func(*v1alpha1.ArgoCD)

func makeTestArgoCD(opts ...argoCDOpt) *v1alpha1.ArgoCD {
	a := &v1alpha1.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func makeTestNotificationsLogger() logr.Logger {
	return ctrl.Log.WithName(ArgoCDNotificationsControllerComponent).WithValues("instance", testArgoCDName, "instance-namespace", testNamespace)
}
