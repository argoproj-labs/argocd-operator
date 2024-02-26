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

	a.Spec = v1alpha1.NotificationsConfigurationSpec{
		// Add a default template for test
		Templates: map[string]string{
			"template.app-created": `email:
			subject: Application {{.app.metadata.name}} has been created.
		  message: Application {{.app.metadata.name}} has been created.
		  teams:
			title: Application {{.app.metadata.name}} has been created.`,
		},
		// Add a default template for test
		Triggers: map[string]string{
			"trigger.on-created": `- description: Application is created.
			oncePer: app.metadata.name
			send:
			- app-created
			when: "true"`,
		},
	}

	err := r.reconcileNotificationsConfigmap(a)
	assert.NoError(t, err)

	// Verify if the ConfigMap is created
	testCM := &corev1.ConfigMap{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      ArgoCDNotificationsConfigMap,
			Namespace: a.Namespace,
		},
		testCM))

	// Verify that the configmap has the default template
	assert.NotEqual(t, testCM.Data["template.app-created"], "")

	// Verify that the configmap has the default trigger
	assert.NotEqual(t, testCM.Data["trigger.on-created"], "")
}

func TestReconcileNotifications_UpdateConfigMap(t *testing.T) {

	a := makeTestNotificationsConfiguration(func(a *v1alpha1.NotificationsConfiguration) {
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(v1alpha1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	a.Spec = v1alpha1.NotificationsConfigurationSpec{
		// Add a default template for test
		Templates: map[string]string{
			"template.app-created": `email:
			subject: Application {{.app.metadata.name}} has been created.
		  message: Application {{.app.metadata.name}} has been created.
		  teams:
			title: Application {{.app.metadata.name}} has been created.`,
		},
		// Add a default template for test
		Triggers: map[string]string{
			"trigger.on-created": `- description: Application is created.
			oncePer: app.metadata.name
			send:
			- app-created
			when: "true"`,
		},
	}

	err := r.reconcileNotificationsConfigmap(a)
	assert.NoError(t, err)

	// Verify if the ConfigMap is created
	testCM := &corev1.ConfigMap{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      ArgoCDNotificationsConfigMap,
			Namespace: a.Namespace,
		},
		testCM))

	// Update the NotificationsConfiguration
	a.Spec.Triggers["trigger.on-sync-status-test"] = "- when: app.status.sync.status == 'Unknown' \n send: [my-custom-template]"

	err = r.reconcileNotificationsConfigmap(a)
	assert.NoError(t, err)

	testCM = &corev1.ConfigMap{}
	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      ArgoCDNotificationsConfigMap,
			Namespace: a.Namespace,
		},
		testCM))

	// Verify that the updated configuration
	assert.Equal(t, testCM.Data["trigger.on-sync-status-test"],
		"- when: app.status.sync.status == 'Unknown' \n send: [my-custom-template]")
}

func TestReconcileNotifications_DeleteConfigMap(t *testing.T) {

	a := makeTestNotificationsConfiguration(func(a *v1alpha1.NotificationsConfiguration) {
	})

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(v1alpha1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	// Update the NotificationsConfiguration
	a.Spec = v1alpha1.NotificationsConfigurationSpec{
		Triggers: map[string]string{
			"trigger.on-sync-status-test": "- when: app.status.sync.status == 'Unknown' \n send: [my-custom-template]",
		},
	}

	err := r.reconcileNotificationsConfigmap(a)
	assert.NoError(t, err)

	// Delete the Notifications ConfigMap
	testCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ArgoCDNotificationsConfigMap,
			Namespace: a.Namespace,
		},
	}
	assert.NoError(t, r.Client.Delete(
		context.TODO(), testCM))

	// Reconcile to check if the ConfigMap is recreated
	err = r.reconcileNotificationsConfigmap(a)
	assert.NoError(t, err)

	assert.NoError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      ArgoCDNotificationsConfigMap,
			Namespace: a.Namespace,
		},
		testCM))

	// Verify if ConfigMap is created with required data
	assert.Equal(t, testCM.Data["trigger.on-sync-status-test"],
		"- when: app.status.sync.status == 'Unknown' \n send: [my-custom-template]")
}
