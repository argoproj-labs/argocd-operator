package common

const (
	// AnnotationName is the annotation on child resources that specifies which ArgoCD instance
	// name a specific object is associated with
	AnnotationName = "argocds.argoproj.io/name"

	// AnnotationNamespace is the annotation on child resources that specifies which ArgoCD instance
	// namespace a specific object is associated with
	AnnotationNamespace = "argocds.argoproj.io/namespace"

	// AnnotationOpenShiftServiceCA is the annotation on services used to
	// request a TLS certificate from OpenShift's Service CA for AutoTLS
	AnnotationOpenShiftServiceCA = "service.beta.openshift.io/serving-cert-secret-name"
)
