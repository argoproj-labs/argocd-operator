package applicationset

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/api/v1beta1"
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var testExpectedLabels = common.DefaultLabels(argocdcommon.TestArgoCDName, argocdcommon.TestNamespace, ArgoCDApplicationSetControllerComponent)

func makeTestApplicationSetReconciler(t *testing.T, objs ...runtime.Object) *ApplicationSetReconciler {
	s := scheme.Scheme
	assert.NoError(t, v1beta1.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	logger := ctrl.Log.WithName(ArgoCDApplicationSetControllerComponent)

	return &ApplicationSetReconciler{
		Client: cl,
		Scheme: s,
		Instance: argocdcommon.MakeTestArgoCD(func(a *argoproj.ArgoCD) {
			a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{}
		}),
		Logger: logger,
	}
}
