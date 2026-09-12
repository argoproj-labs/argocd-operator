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

	// AnnotationOpenShiftOriginatingServiceName is the annotation on secrets used to
	// identify the service that created the secret.
	AnnotationOpenShiftOriginatingServiceName = "service.beta.openshift.io/originating-service-name"

	// AnnotationOwnedPrincipalServiceAnnotations is the annotation on the principal
	// service that tracks which annotation keys were applied by the operator from the
	// ArgoCD CR spec, so only those keys are removed when the spec changes.
	AnnotationOwnedPrincipalServiceAnnotations = "argocds.argoproj.io/owned-principal-service-annotations"
)
