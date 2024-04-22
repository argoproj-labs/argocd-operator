package applicationset

import (
	"testing"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

var testExpectedLabels = common.DefaultResourceLabels(test.TestArgoCDName, test.TestNamespace, common.AppSetControllerComponent)

func makeTestApplicationSetReconciler(t *testing.T, webhookServerRouteEnabled bool, objs ...runtime.Object) *ApplicationSetReconciler {
	s := scheme.Scheme

	assert.NoError(t, routev1.Install(s))
	assert.NoError(t, argoproj.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()

	return &ApplicationSetReconciler{
		Client: cl,
		Scheme: s,
		Logger: util.NewLogger("appset-controller"),
		Instance: test.MakeTestArgoCD(nil, func(a *argoproj.ArgoCD) {
			a.Spec.ApplicationSet = &argoproj.ArgoCDApplicationSet{
				WebhookServer: argoproj.WebhookServerSpec{
					Route: argoproj.ArgoCDRouteSpec{
						Enabled: webhookServerRouteEnabled,
					},
				},
			}
		}),
	}
}
