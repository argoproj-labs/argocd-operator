package reposerver

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func makeTestReposerverReconciler(cr *argoproj.ArgoCD, objs ...runtime.Object) *RepoServerReconciler {
	schemeOpt := func(s *runtime.Scheme) {
		monitoringv1.AddToScheme(s)
		argoproj.AddToScheme(s)
	}
	sch := test.MakeTestReconcilerScheme(schemeOpt)

	client := test.MakeTestReconcilerClient(sch, []client.Object{}, []client.Object{}, objs)

	return &RepoServerReconciler{
		Client:   client,
		Scheme:   sch,
		Instance: cr,
		Logger:   util.NewLogger(common.RepoServerController),
	}
}
