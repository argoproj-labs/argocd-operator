package notifications

import (
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var testExpectedLabels = common.DefaultResourceLabels(argocdcommon.TestArgoCDName, argocdcommon.TestNamespace, common.NotificationsControllerComponent)

func makeTestNotificationsReconciler(t *testing.T, objs ...runtime.Object) *NotificationsReconciler {
	s := scheme.Scheme
	assert.NoError(t, argoproj.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	logger := ctrl.Log.WithName(common.NotificationsControllerComponent)

	return &NotificationsReconciler{
		Client:   cl,
		Scheme:   s,
		Instance: argocdcommon.MakeTestArgoCD(),
		Logger:   logger,
	}
}

func TestNotificationsReconciler_Reconcile(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	resourceName = argocdcommon.TestArgoCDName
	tests := []struct {
		name         string
		resourceName string
		setupClient  func() *NotificationsReconciler
		wantErr      bool
	}{
		{
			name:         "successful reconcile",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t, ns)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		nr := tt.setupClient()
		err := nr.Reconcile()
		assert.NoError(t, err)
		if (err != nil) != tt.wantErr {
			if tt.wantErr {
				t.Errorf("Expected error but did not get one")
			} else {
				t.Errorf("Unexpected error: %v", err)
			}
		}
	}
}

func TestNotificationsReconciler_DeleteResources(t *testing.T) {
	resourceName = argocdcommon.TestArgoCDName
	tests := []struct {
		name         string
		resourceName string
		setupClient  func() *NotificationsReconciler
		wantErr      bool
	}{
		{
			name:         "successful delete",
			resourceName: argocdcommon.TestArgoCDName,
			setupClient: func() *NotificationsReconciler {
				return makeTestNotificationsReconciler(t)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nr := tt.setupClient()
			if err := nr.DeleteResources(); (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("Expected error but did not get one")
				} else {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
