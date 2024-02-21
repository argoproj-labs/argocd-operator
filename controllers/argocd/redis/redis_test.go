package redis

import (
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func makeTestRedisReconciler(cr *argoproj.ArgoCD, objs ...client.Object) *RedisReconciler {
	schemeOpt := func(s *runtime.Scheme) {
		argoproj.AddToScheme(s)
	}
	sch := test.MakeTestReconcilerScheme(schemeOpt)

	client := test.MakeTestReconcilerClient(sch, objs, []client.Object{cr}, []runtime.Object{cr})

	return &RedisReconciler{
		Client:   client,
		Scheme:   sch,
		Instance: cr,
		Logger:   util.NewLogger(common.RedisComponent),
	}
}

func Test_reconcile(t *testing.T) {
	tests := []struct {
		name              string
		reconciler        *RedisReconciler
		expectedResources []client.Object
	}{
		{
			name: "non HA mode",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedResources: []client.Object{},
		},
	}
}
