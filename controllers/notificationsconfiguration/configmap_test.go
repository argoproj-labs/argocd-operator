package notificationsconfiguration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
)

type notificationsOpts func(*v1alpha1.NotificationsConfiguration)

type SchemeOpt func(*runtime.Scheme) error

func makeTestNotificationsConfiguration(opts ...notificationsOpts) *v1alpha1.NotificationsConfiguration {
	a := &v1alpha1.NotificationsConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default-notifications-configuration",
			Namespace: "default",
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func makeTestReconcilerScheme(sOpts ...SchemeOpt) *runtime.Scheme {
	s := scheme.Scheme
	for _, opt := range sOpts {
		_ = opt(s)
	}

	return s
}

func makeTestReconciler(client client.Client, sch *runtime.Scheme) *NotificationsConfigurationReconciler {
	return &NotificationsConfigurationReconciler{
		Client: client,
		Scheme: sch,
	}
}

func makeTestReconcilerClient(sch *runtime.Scheme, resObjs, subresObjs []client.Object, runtimeObj []runtime.Object) client.Client {
	client := fake.NewClientBuilder().WithScheme(sch)
	if len(resObjs) > 0 {
		client = client.WithObjects(resObjs...)
	}
	if len(subresObjs) > 0 {
		client = client.WithStatusSubresource(subresObjs...)
	}
	if len(runtimeObj) > 0 {
		client = client.WithRuntimeObjects(runtimeObj...)
	}
	return client.Build()
}

func TestReconcileNotifications_CreateConfigMap(t *testing.T) {

	a := makeTestNotificationsConfiguration(func(a *v1alpha1.NotificationsConfiguration) {
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(v1alpha1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	err := r.reconcileNotificationsConfigmap(a)
	assert.NoError(t, err)

	testCM := &corev1.ConfigMap{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "argocd-notifications-cm",
			Namespace: a.Namespace,
		},
		testCM))

	expectedDefaultConfig := getDefaultNotificationsConfig()
	assert.Equal(t, expectedDefaultConfig, testCM.Data)
}
