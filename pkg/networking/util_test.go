package networking

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/stretchr/testify/assert"
)

func TestEnsureAutoTLSAnnotation(t *testing.T) {

	secretName := "some-secret"

	tlsAnnotations := map[string]string{
		common.ArgoCDArgoprojKeyName:             testInstance,
		common.ArgoCDArgoprojKeyNamespace:        testInstanceNamespace,
		common.ServiceBetaOpenshiftKeyCertSecret: secretName,
	}

	noTLSAnnotations := map[string]string{
		common.ArgoCDArgoprojKeyName:      testInstance,
		common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
	}

	t.Run("Ensure annotation will be set & unset for OpenShift", func(t *testing.T) {
		routeAPIFound = true
		svc := getTestService()

		// Annotation should be injected
		EnsureAutoTLSAnnotation(svc, secretName, true)
		assert.Equal(t, tlsAnnotations, svc.ObjectMeta.Annotations)

		// Annotation already set, no duplicate addition
		EnsureAutoTLSAnnotation(svc, secretName, true)
		assert.Equal(t, tlsAnnotations, svc.ObjectMeta.Annotations)

		// Annotation should be removed
		EnsureAutoTLSAnnotation(svc, secretName, false)
		assert.Equal(t, noTLSAnnotations, svc.ObjectMeta.Annotations)
	})

	t.Run("Ensure annotation will not be set for non-OpenShift", func(t *testing.T) {
		routeAPIFound = false
		svc := getTestService()

		// Annotation should be not be injected
		EnsureAutoTLSAnnotation(svc, secretName, true)
		assert.Equal(t, noTLSAnnotations, svc.ObjectMeta.Annotations)

	})

}
