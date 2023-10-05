package applicationset

import (
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var testExpectedLabels = common.DefaultLabels(argocdcommon.TestArgoCDName, argocdcommon.TestNamespace, AppSetControllerComponent)

func makeTestApplicationSetReconciler(t *testing.T, webhookServerRouteEnabled bool, objs ...runtime.Object) *ApplicationSetReconciler {
	s := scheme.Scheme

	assert.NoError(t, routev1.Install(s))
	assert.NoError(t, argoproj.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	logger := ctrl.Log.WithName(AppSetControllerComponent)

	return &ApplicationSetReconciler{
		Client: cl,
		Scheme: s,
		Instance: argocdcommon.MakeTestArgoCD(func(a *argoproj.ArgoCD) {
			a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
				WebhookServer: argoproj.WebhookServerSpec{
					Route: argoproj.ArgoCDRouteSpec{
						Enabled: webhookServerRouteEnabled,
					},
				},
			}
		}),
		Logger: logger,
	}
}
