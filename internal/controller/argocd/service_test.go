package argocd

import (
	"context"
	"testing"

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
		needUpdate := ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", true)
		assert.Equal(t, needUpdate, true)
		atls, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, true)
		assert.Equal(t, atls, "some-secret")

		// Annotation already set, doesn't need update
		needUpdate = ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", true)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Ensure annotation will be unset for OpenShift", func(t *testing.T) {
		routeAPIFound = true
		svc := newService(a)
		svc.Annotations = make(map[string]string)
		svc.Annotations[common.AnnotationOpenShiftServiceCA] = "some-secret"

		// Annotation getting removed, update required
		needUpdate := ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", false)
		assert.Equal(t, needUpdate, true)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)

		// Annotation does not exist, no update required
		needUpdate = ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", false)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Ensure annotation will not be set for non-OpenShift", func(t *testing.T) {
		routeAPIFound = false
		svc := newService(a)
		needUpdate := ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", true)
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
		needUpdate := ensureAutoTLSAnnotation(fakeClient, svc, secret.Name, true)
		assert.Equal(t, needUpdate, false)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)

		// Annotation does not exist, no update required
		needUpdate = ensureAutoTLSAnnotation(fakeClient, svc, "some-secret", false)
		assert.Equal(t, needUpdate, false)
	})
}
