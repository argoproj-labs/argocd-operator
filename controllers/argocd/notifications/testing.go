package notifications

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	testKey      = "test"
	testVal      = "test"
	testReplicas = int32(1)
	testRoleRef  = rbacv1.RoleRef{
		Kind:     RoleKind,
		Name:     argocdcommon.TestArgoCDName,
		APIGroup: "rbac.authorization.k8s.io",
	}

	testSubjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      argocdcommon.TestArgoCDName,
			Namespace: argocdcommon.TestNamespace,
		},
	}

	testConfigMapData = GetDefaultNotificationsConfig()

	testKVP = map[string]string{
		testKey: testVal,
	}

	testLabels = common.DefaultLabels(argocdcommon.TestArgoCDName, argocdcommon.TestNamespace, ArgoCDNotificationsControllerComponent)
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
