package reposerver

import (
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var testExpectedLabels = common.DefaultResourceLabels(argocdcommon.TestArgoCDName, argocdcommon.TestNamespace, common.RepoServerControllerComponent)

const testServiceAccount = "test-service-account"

func makeTestRepoServerReconciler(t *testing.T, objs ...runtime.Object) *RepoServerReconciler {
	s := scheme.Scheme

	assert.NoError(t, monitoringv1.AddToScheme(s))
	assert.NoError(t, argoproj.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	logger := ctrl.Log.WithName(common.RepoServerControllerComponent)

	return &RepoServerReconciler{
		Client: cl,
		Scheme: s,
		Instance: argocdcommon.MakeTestArgoCD(func(a *argoproj.ArgoCD) {
			a.Spec.Repo = argoproj.ArgoCDRepoSpec{
				ServiceAccount: testServiceAccount,
				AutoTLS:        common.OpenShift,
			}
			a.ObjectMeta = metav1.ObjectMeta{
				Name:      argocdcommon.TestArgoCDName,
				Namespace: argocdcommon.TestNamespace,
				UID:       argocdcommon.TestUID,
			}
		}),
		Logger: logger,
	}
}
