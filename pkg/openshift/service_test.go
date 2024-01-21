package openshift

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type serviceOpt func(*corev1.Service)

func getTestService(opts ...serviceOpt) *corev1.Service {
	desiredService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Labels: map[string]string{
				common.AppK8sKeyName:      testInstance,
				common.AppK8sKeyPartOf:    common.ArgoCDAppName,
				common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
				common.AppK8sKeyComponent: testComponent,
			},
			Annotations: map[string]string{
				common.ArgoCDArgoprojKeyName:      testInstance,
				common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}

	for _, opt := range opts {
		opt(desiredService)
	}
	return desiredService
}

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
		SetRouteAPIFound(true)
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
		SetRouteAPIFound(false)
		svc := getTestService()

		// Annotation should be not be injected
		EnsureAutoTLSAnnotation(svc, secretName, true)
		assert.Equal(t, noTLSAnnotations, svc.ObjectMeta.Annotations)

	})

}
