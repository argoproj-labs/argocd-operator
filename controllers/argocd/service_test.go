package argocd

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	testclient "k8s.io/client-go/kubernetes/fake"

	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

func TestEnsureAutoTLSAnnotation(t *testing.T) {
	a := makeTestArgoCD()
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	fakeClient := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	t.Run("Ensure annotation will be set for OpenShift", func(t *testing.T) {
		routeAPIFound = true
		svc := newService(a)

		// Annotation is inserted, update is required
		needUpdate, err := ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", true)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, true)
		atls, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, true)
		assert.Equal(t, atls, "some-secret")

		// Annotation already set, doesn't need update
		needUpdate, err = ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", true)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Ensure annotation will be unset for OpenShift", func(t *testing.T) {
		routeAPIFound = true
		svc := newService(a)
		svc.Annotations = make(map[string]string)
		svc.Annotations[common.AnnotationOpenShiftServiceCA] = "some-secret"

		// Annotation getting removed, update required
		needUpdate, err := ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", false)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, true)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)

		// Annotation does not exist, no update required
		needUpdate, err = ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", false)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Ensure annotation will not be set for non-OpenShift", func(t *testing.T) {
		routeAPIFound = false
		svc := newService(a)
		needUpdate, err := ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", true)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, false)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)
	})
	t.Run("Ensure annotation will not be set if the TLS secret is already present", func(t *testing.T) {
		routeAPIFound = true
		svc := newService(a)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: svc.Namespace,
			},
		}
		err := fakeClient.Create(context.Background(), secret)
		assert.NoError(t, err)
		needUpdate, err := ensureAutoTLSAnnotation(fakeClient, svc, secret.Name, true)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, false)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)

		// Annotation does not exist, no update required
		needUpdate, err = ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", false)
		assert.Nil(t, err)
		assert.Equal(t, needUpdate, false)
	})
}

func TestReconcileServerService(t *testing.T) {
	a := makeTestArgoCD()
	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())
	a = makeTestArgoCD(func(a *argoproj.ArgoCD) {
		a.Spec.Server.Service.Type = "NodePort"
	})
	serverService := newServiceWithSuffix("server", "server", a)
	t.Run("Server Service Created when the Server Service is not found ", func(t *testing.T) {
		err := r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-server",
			Namespace: testNamespace,
		}, serverService)
		assert.True(t, errors.IsNotFound(err))

		err = r.reconcileServerService(a)
		assert.NoError(t, err)

		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-server",
			Namespace: testNamespace,
		}, serverService)
		assert.NoError(t, err)
		assert.Equal(t, a.Spec.Server.Service.Type, serverService.Spec.Type)
	})

	t.Run("Server Service Type update ", func(t *testing.T) {
		// Reconcile with previous existing Server Service with a different Type
		a.Spec.Server.Service.Type = "ClusterIP"
		assert.NotEqual(t, a.Spec.Server.Service.Type, serverService.Spec.Type)

		err := r.reconcileServerService(a)
		assert.NoError(t, err)

		// Existing Server is found and has the argoCD new Server Service Type
		err = r.Get(context.TODO(), types.NamespacedName{
			Name:      "argocd-server",
			Namespace: testNamespace,
		}, serverService)
		assert.NoError(t, err)
		assert.Equal(t, a.Spec.Server.Service.Type, serverService.Spec.Type)
	})
}

// If `remote` field is used in CR, then the component resources should not be created
func TestReconcileArgoCD_reconcileRedisWithRemoteEn(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	redisRemote := "https://remote.redis.instance"

	cr.Spec.Redis.Remote = &redisRemote
	assert.NoError(t, r.reconcileRedisService(cr))

	s := &corev1.Service{}

	assert.ErrorContains(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-redis", Namespace: cr.Namespace}, s),
		"services \"argocd-redis\" not found")
}

func TestReconcileArgoCD_reconcileRepoServerWithRemoteEnabled(t *testing.T) {
	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	subresObjs := []client.Object{cr}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoproj.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch, testclient.NewSimpleClientset())

	repoServerRemote := "https://remote.repo-server.instance"

	cr.Spec.Repo.Remote = &repoServerRemote
	assert.NoError(t, r.reconcileRepoService(cr))

	s := &corev1.Service{}

	assert.ErrorContains(t, r.Get(context.TODO(), types.NamespacedName{Name: cr.Name + "-repo-server", Namespace: cr.Namespace}, s),
		"services \"argocd-repo-server\" not found")
}
