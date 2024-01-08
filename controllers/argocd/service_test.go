package argocd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj-labs/argocd-operator/common"
)

func TestEnsureAutoTLSAnnotation(t *testing.T) {
	a := makeTestArgoCD()
	t.Run("Ensure annotation will be set for OpenShift", func(t *testing.T) {
		routeAPIFound = true
		svc := newService(a)

		// Annotation is inserted, update is required
		needUpdate := ensureAutoTLSAnnotation(svc, "some-secret", true)
		assert.Equal(t, needUpdate, true)
		atls, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, true)
		assert.Equal(t, atls, "some-secret")

		// Annotation already set, doesn't need update
		needUpdate = ensureAutoTLSAnnotation(svc, "some-secret", true)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Ensure annotation will be unset for OpenShift", func(t *testing.T) {
		routeAPIFound = true
		svc := newService(a)
		svc.Annotations = make(map[string]string)
		svc.Annotations[common.AnnotationOpenShiftServiceCA] = "some-secret"

		// Annotation getting removed, update required
		needUpdate := ensureAutoTLSAnnotation(svc, "some-secret", false)
		assert.Equal(t, needUpdate, true)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)

		// Annotation does not exist, no update required
		needUpdate = ensureAutoTLSAnnotation(svc, "some-secret", false)
		assert.Equal(t, needUpdate, false)
	})
	t.Run("Ensure annotation will not be set for non-OpenShift", func(t *testing.T) {
		routeAPIFound = false
		svc := newService(a)
		needUpdate := ensureAutoTLSAnnotation(svc, "some-secret", true)
		assert.Equal(t, needUpdate, false)
		_, ok := svc.Annotations[common.AnnotationOpenShiftServiceCA]
		assert.Equal(t, ok, false)
	})
}
