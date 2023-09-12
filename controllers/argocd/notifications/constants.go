package notifications

// Values
const (
	// ArgoCDNotificationsControllerComponent is the name of the Notifications controller control plane component
	ArgoCDNotificationsControllerComponent = "notifications-controller"
	NotificationsSecretName                = "argocd-notifications-secret"
	NotificationsConfigMapName             = "argocd-notifications-cm"
	DeploymentKind                         = "Deployment"
	RoleKind                               = "Role"
	RoleBindingKind                        = "RoleBinding"
	ConfigMapKind                          = "ConfigMap"
	SecretKind                             = "Secret"
	ServiceAccountKind                     = "ServiceAccount"
	TLSCerts                               = "tls-certs"
	ArgoCDRepoServerTLS                    = "argocd-repo-server-tls"
	CapabilityDropAll                      = "ALL"
	VolumeMountPathTLS                     = "/app/config/tls"
	VolumeMountPathRepoServerTLS           = "/app/config/reposerver/tls"
	WorkingDirApp                          = "/app"
	ImageUpgradedLabel                     = "image.upgraded"
	TimeFormatMST                          = "01022006-150406-MST"
	APIVersionV1                           = "v1"
	APIVersionAppsV1                       = "apps/v1"
	APIVersionRbacV1                       = "rbac.authorization.k8s.io/v1"
)
