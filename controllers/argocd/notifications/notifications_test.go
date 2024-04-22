package notifications

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

var testExpectedLabels = common.DefaultResourceLabels(test.TestArgoCDName, test.TestNamespace, common.ArgoCDNotificationsControllerComponent)

func makeTestNotificationsReconciler(t *testing.T, objs ...runtime.Object) *NotificationsReconciler {
	s := scheme.Scheme
	assert.NoError(t, argoproj.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()

	return &NotificationsReconciler{
		Client:   cl,
		Scheme:   s,
		Instance: test.MakeTestArgoCD(nil),
		// Logger:   logger,
	}
}

// func TestNotificationsReconciler_Reconcile(t *testing.T) {
// 	ns := test.MakeTestNamespace(nil)
// 	resourceName = test.TestArgoCDName
// 	tests := []struct {
// 		name         string
// 		resourceName string
// 		setupClient  func() *NotificationsReconciler
// 		wantErr      bool
// 	}{
// 		{
// 			name:         "successful reconcile",
// 			resourceName: test.TestArgoCDName,
// 			setupClient: func() *NotificationsReconciler {
// 				return makeTestNotificationsReconciler(t, ns)
// 			},
// 			wantErr: false,
// 		},
// 	}
// 	for _, tt := range tests {
// 		nr := tt.setupClient()
// 		err := nr.Reconcile()
// 		assert.NoError(t, err)
// 		if (err != nil) != tt.wantErr {
// 			if tt.wantErr {
// 				t.Errorf("Expected error but did not get one")
// 			} else {
// 				t.Errorf("Unexpected error: %v", err)
// 			}
// 		}
// 	}
// }

// func TestNotificationsReconciler_DeleteResources(t *testing.T) {
// 	resourceName = test.TestArgoCDName
// 	tests := []struct {
// 		name         string
// 		resourceName string
// 		setupClient  func() *NotificationsReconciler
// 		wantErr      bool
// 	}{
// 		{
// 			name:         "successful delete",
// 			resourceName: test.TestArgoCDName,
// 			setupClient: func() *NotificationsReconciler {
// 				return makeTestNotificationsReconciler(t)
// 			},
// 			wantErr: false,
// 		},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			nr := tt.setupClient()
// 			if err := nr.DeleteResources(); (err != nil) != tt.wantErr {
// 				if tt.wantErr {
// 					t.Errorf("Expected error but did not get one")
// 				} else {
// 					t.Errorf("Unexpected error: %v", err)
// 				}
// 			}
// 		})
// 	}
// }
