package networking

import (
	"github.com/argoproj-labs/argocd-operator/common"
	corev1 "k8s.io/api/core/v1"
)

// ensureAutoTLSAnnotation ensures that the service svc has the desired state
// of the auto TLS annotation set, which is either set (when enabled is true)
// or unset (when enabled is false).
func EnsureAutoTLSAnnotation(svc *corev1.Service, secretName string, enabled bool) {
	var autoTLSAnnotationName, autoTLSAnnotationValue string

	// We currently only support OpenShift for automatic TLS
	if IsRouteAPIAvailable() {
		autoTLSAnnotationName = common.ServiceBetaOpenshiftKeyCertSecret
		if svc.Annotations == nil {
			svc.Annotations = make(map[string]string)
		}
		autoTLSAnnotationValue = secretName
	}

	if autoTLSAnnotationName != "" {
		val, ok := svc.Annotations[autoTLSAnnotationName]
		if enabled {
			if !ok || val != secretName {
				svc.Annotations[autoTLSAnnotationName] = autoTLSAnnotationValue
			}
		} else {
			if ok {
				delete(svc.Annotations, autoTLSAnnotationName)
			}
		}
	}
}
