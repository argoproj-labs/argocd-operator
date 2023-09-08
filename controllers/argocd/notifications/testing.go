package notifications

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func makeTestNotificationsReconciler(t *testing.T, objs ...runtime.Object) *NotificationsReconciler {
	s := scheme.Scheme
	assert.NoError(t, v1alpha1.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	logger := ctrl.Log.WithName(ArgoCDNotificationsControllerComponent)

	return &NotificationsReconciler{
		Client:   cl,
		Scheme:   s,
		Instance: argocdcommon.MakeTestArgoCD(),
		Logger:   logger,
	}
}
