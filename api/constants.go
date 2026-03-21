package api

const (

	// ArgoCDAppName is the application name for labels.
	ArgoCDAppName = "argocd"

	// ArgoCDDeletionFinalizer is a finalizer to implement pre-delete hooks
	ArgoCDDeletionFinalizer = "argoproj.io/finalizer"

	// ArgoCDDefaultApplicationInstanceLabelKey is the default app name as a tracking label.
	ArgoCDDefaultApplicationInstanceLabelKey = "app.kubernetes.io/instance"

	// ArgoCDTrackedByOperatorLabel for resources tracked by the operator
	ArgoCDTrackedByOperatorLabel = "operator.argoproj.io/tracked-by"

	// Label Selector is an env variable for ArgoCD instance reconcilliation.
	ArgoCDLabelSelectorKey = "ARGOCD_LABEL_SELECTOR"

	// ArgoCDImagePullPolicyEnvName is the environment variable used to get the global image pull policy
	// for all ArgoCD components managed by the operator.
	ArgoCDImagePullPolicyEnvName = "IMAGE_PULL_POLICY"

	// ArgoCDDefaultLabelSelector is the default Label Selector which will reconcile all ArgoCD instances.
	ArgoCDDefaultLabelSelector = ""
)
