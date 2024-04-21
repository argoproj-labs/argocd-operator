package common

// notifications
const (
	NotificationsController          = "notifications-controller"
	NotificationsControllerSuffix    = "notifications-controller"
	NotificationsControllerComponent = "argocd-notifications-controller"
	NotificationsSecretName          = "argocd-notifications-secret"
	NotificationsConfigMapName       = "argocd-notifications-cm"
	// NotificationsControllerMetricsPort is the port that is used to expose notifications controller metrics.
	NotificationsControllerMetricsPort = 9001
)
