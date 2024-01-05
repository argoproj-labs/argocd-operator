package common

// keycloak
const (
	// ArgoCDKeycloakImage is the default Keycloak Image used for the non-openshift platforms when not specified.
	ArgoCDKeycloakImage = "quay.io/keycloak/keycloak"

	// ArgoCDKeycloakVersion is the default Keycloak version used for the non-openshift platform when not specified.
	// Version: 15.0.2
	ArgoCDKeycloakVersion = "sha256:64fb81886fde61dee55091e6033481fa5ccdac62ae30a4fd29b54eb5e97df6a9"

	// ArgoCDKeycloakImageForOpenShift is the default Keycloak Image used for the OpenShift platform when not specified.
	ArgoCDKeycloakImageForOpenShift = "registry.redhat.io/rh-sso-7/sso76-openshift-rhel8"

	// ArgoCDKeycloakVersionForOpenShift is the default Keycloak version used for the OpenShift platform when not specified.
	// Version: 7.5.1
	ArgoCDKeycloakVersionForOpenShift = "sha256:720a7e4c4926c41c1219a90daaea3b971a3d0da5a152a96fed4fb544d80f52e3"
)
