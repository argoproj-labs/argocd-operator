package notifications

// Values
const (
	// ArgoCDNotificationsControllerComponent is the name of the Notifications controller control plane component
	ArgoCDNotificationsControllerComponent = "notifications-controller"
	RoleKind                               = "Role"
	TLSCerts                               = "tls-certs"
	ArgoCDRepoServerTLS                    = "argocd-repo-server-tls"
	CapabilityDropAll                      = "ALL"
	VolumeMountPathTLS                     = "/app/config/tls"
	VolumeMountPathRepoServerTLS           = "/app/config/reposerver/tls"
	WorkingDirApp                          = "/app"
	ImageUpgradedLabel                     = "image.upgraded"
	TimeFormatMST                          = "01022006-150406-MST"
)
