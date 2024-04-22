package server

import (
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/redis"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/reposerver"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/sso"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/sso/dex"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func MakeTestServerReconciler(cr *argoproj.ArgoCD, objs ...client.Object) *ServerReconciler {
	schemeOpt := func(s *runtime.Scheme) {
		argoproj.AddToScheme(s)
	}
	sch := test.MakeTestReconcilerScheme(schemeOpt)

	client := test.MakeTestReconcilerClient(sch, objs, []client.Object{cr}, []runtime.Object{cr})

	reposerverController := &reposerver.RepoServerReconciler{
		Client:   client,
		Scheme:   sch,
		Instance: cr,
		Logger:   util.NewLogger(common.RepoServerController, "instance", cr.Name, "instance-namespace", cr.Namespace),
	}

	redisController := &redis.RedisReconciler{
		Client:   client,
		Scheme:   sch,
		Instance: cr,
		Logger:   util.NewLogger(common.RedisController, "instance", cr.Name, "instance-namespace", cr.Namespace),
	}

	dexController := &dex.DexReconciler{
		Client:   client,
		Scheme:   sch,
		Instance: cr,
	}

	ssoController := &sso.SSOReconciler{
		Client:        client,
		Scheme:        sch,
		Instance:      cr,
		DexController: dexController,
	}

	return &ServerReconciler{
		Client:     client,
		Scheme:     sch,
		Instance:   cr,
		Logger:     util.NewLogger(common.RedisComponent),
		RepoServer: reposerverController,
		Redis:      redisController,
		SSO:        ssoController,
	}
}

func TestServerReconciler_Reconcile(t *testing.T) {
	tests := []struct {
		name        string
		reconciler  *ServerReconciler
		expectError bool
	}{
		{
			name: "successful reconcile",
			reconciler: MakeTestServerReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectError: false,
		},
	}
	for _, tt := range tests {
		tt.reconciler.varSetter()
		err := tt.reconciler.Reconcile()
		assert.NoError(t, err)
		if (err != nil) != tt.expectError {
			if tt.expectError {
				t.Errorf("Expected error but did not get one")
			} else {
				t.Errorf("Unexpected error: %v", err)
			}
		}
	}
}

func TestServerReconciler_DeleteResources(t *testing.T) {
	tests := []struct {
		name        string
		reconciler  *ServerReconciler
		expectError bool
	}{
		{
			name: "successful delete",
			reconciler: MakeTestServerReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.varSetter()
			tt.reconciler.Reconcile()

			if err := tt.reconciler.DeleteResources(); (err != nil) != tt.expectError {
				if tt.expectError {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
