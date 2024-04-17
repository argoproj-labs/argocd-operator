// Copyright 2020 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

// general keys
const (
	ImageUpgradedKey = "image.upgraded"
)

// ArgoCD keys
const (
	// ArgoCDKeyAdminEnabled is the configuration key for the admin enabled setting..
	ArgoCDKeyAdminEnabled = "admin.enabled"

	// ArgoCDKeyApplicationInstanceLabelKey is the configuration key for the application instance label.
	ArgoCDKeyApplicationInstanceLabelKey = "application.instanceLabelKey"

	// ArgoCDKeyAdminPassword is the admin password key for labels.
	ArgoCDKeyAdminPassword = "admin.password"

	// ArgoCDKeyAdminPasswordMTime is the admin password last modified key for labels.
	ArgoCDKeyAdminPasswordMTime = "admin.passwordMtime"

	// ArgoCDKeyBackupKey is the "backup key" key for ConfigMaps.
	ArgoCDKeyBackupKey = "backup.key"

	// ArgoCDKeyConfigManagementPlugins is the configuration key for config management plugins.
	ArgoCDKeyConfigManagementPlugins = "configManagementPlugins"

	// ArgoCDKeyGATrackingID is the configuration key for the Google  Analytics Tracking ID.
	ArgoCDKeyGATrackingID = "ga.trackingid"

	// ArgoCDKeyGAAnonymizeUsers is the configuration key for the Google Analytics user anonymization.
	ArgoCDKeyGAAnonymizeUsers = "ga.anonymizeusers"

	// ArgoCDKeyHelpChatURL is the congifuration key for the help chat URL.
	ArgoCDKeyHelpChatURL = "help.chatUrl"

	// ArgoCDKeyHelpChatText is the congifuration key for the help chat text.
	ArgoCDKeyHelpChatText = "help.chatText"

	// ArgoCDKeyKustomizeBuildOptions is the configuration key for the kustomize build options.
	ArgoCDKeyKustomizeBuildOptions = "kustomize.buildOptions"

	// ArgoCDKeyMetrics is the resource metrics key for labels.
	ArgoCDKeyMetrics = "metrics"

	// ArgoCDKeyOIDCConfig is the configuration key for the OIDC configuration.
	ArgoCDKeyOIDCConfig = "oidc.config"

	// ArgoCDKeyRBACPolicyCSV is the configuration key for the Argo CD RBAC policy CSV.
	ArgoCDKeyRBACPolicyCSV = "policy.csv"

	// ArgoCDKeyRBACPolicyDefault is the configuration key for the Argo CD RBAC default policy.
	ArgoCDKeyRBACPolicyDefault = "policy.default"

	// ArgoCDKeyRBACScopes is the configuration key for the Argo CD RBAC scopes.
	ArgoCDKeyRBACScopes = "scopes"

	// ArgoCDKeyResourceExclusions is the configuration key for resource exclusions.
	ArgoCDKeyResourceExclusions = "resource.exclusions"

	// ArgoCDKeyResourceInclusions is the configuration key for resource inclusions.
	ArgoCDKeyResourceInclusions = "resource.inclusions"

	// ArgoCDKeyResourceTrackingMethod is the configuration key for resource tracking method
	ArgoCDKeyResourceTrackingMethod = "application.resourceTrackingMethod"

	// ArgoCDKeyRepositories is the configuration key for repositories.
	ArgoCDKeyRepositories = "repositories"

	// ArgoCDKeyRepositoryCredentials is the configuration key for repository.credentials.
	ArgoCDKeyRepositoryCredentials = "repository.credentials"

	// ArgoCDKeyServerSecretKey is the server secret key property name for the Argo secret.
	ArgoCDKeyServerSecretKey = "server.secretkey"

	// ArgoCDKeyServerURL is the key for server url.
	ArgoCDKeyServerURL = "url"

	// ArgoCDKeySSHKnownHosts is the resource ssh_known_hosts key for labels.
	ArgoCDKeySSHKnownHosts = "ssh_known_hosts"

	// ArgoCDKeyStatusBadgeEnabled is the configuration key for enabling the status badge.
	ArgoCDKeyStatusBadgeEnabled = "statusbadge.enabled"

	// ArgoCDKeyBannerContent is the configuration key for a banner message content.
	ArgoCDKeyBannerContent = "ui.bannercontent"

	// ArgoCDKeyBannerURL is the configuration key for a banner message URL.
	ArgoCDKeyBannerURL = "ui.bannerurl"

	// ArgoCDKeyTLSCACert is the key for TLS CA certificates.
	ArgoCDKeyTLSCACert = "ca.crt"

	// ArgoCDKeyPolicyMatcherMode is the key for matchers function for casbin.
	// There are two options for this, 'glob' for glob matcher or 'regex' for regex matcher.
	ArgoCDKeyPolicyMatcherMode = "policy.matchMode"

	// ArgoCDKeyUsersAnonymousEnabled is the configuration key for anonymous user access.
	ArgoCDKeyUsersAnonymousEnabled = "users.anonymous.enabled"

	// ArgoCDDexSecretKey is used to reference Dex secret from Argo CD secret into Argo CD configmap
	ArgoCDDexSecretKey = "oidc.dex.clientSecret"

	ArgoCDKeyKustomizeVersion       = "kustomize.version."
	ArgoCDKeyResourceCustomizations = "resource.customizations."
)

// openshift.io keys
const (
	// SAOpenshiftKeyOAuthRedirectURI is the key for the OAuth Redirect URI annotation.
	SAOpenshiftKeyOAuthRedirectURI = "serviceaccounts.openshift.io/oauth-redirecturi.argocd"

	// ServiceBetaOpenshiftKeyCertSecret is the annotation on services used to
	// request a TLS certificate from OpenShift's Service CA for AutoTLS
	ServiceBetaOpenshiftKeyCertSecret = "service.beta.openshift.io/serving-cert-secret-name"
)

// kubernetes.io keys
const (
	// AppK8sKeyName is the resource name key for labels.
	AppK8sKeyName = "app.kubernetes.io/name"

	// AppK8sKeyInstance is the instance name key for labels.
	AppK8sKeyInstance = "app.kubernetes.io/instance"

	// AppK8sKeyPartOf is the resource part-of key for labels.
	AppK8sKeyPartOf = "app.kubernetes.io/part-of"

	// AppK8sKeyComponent is the resource component key for labels.
	AppK8sKeyComponent = "app.kubernetes.io/component"

	// AppK8sKeyManagedBy is the managed-by key for labels.
	AppK8sKeyManagedBy = "app.kubernetes.io/managed-by"

	// StatefulSetK8sKeyPodName is the resource StatefulSet Pod Name key for labels.
	StatefulSetK8sKeyPodName = "statefulset.kubernetes.io/pod-name"

	// K8sKeyOS is the os key for labels.
	K8sKeyOS = "kubernetes.io/os"

	// K8sKeyHostname is the resource hostname key for labels.
	K8sKeyHostname = "kubernetes.io/hostname"

	// K8sKeyIngressClass is the ingress class key for labels.
	K8sKeyIngressClass = "kubernetes.io/ingress.class"

	// NginxIngressK8sKeyBackendProtocol is the backend-protocol key for labels.
	NginxIngressK8sKeyBackendProtocol = "nginx.ingress.kubernetes.io/backend-protocol"

	// NginxIngressK8sKeyForceSSLRedirect is the ssl force-redirect key for labels.
	NginxIngressK8sKeyForceSSLRedirect = "nginx.ingress.kubernetes.io/force-ssl-redirect"

	// NginxIngressK8sKeySSLPassthrough is the ssl passthrough key for labels.
	NginxIngressK8sKeySSLPassthrough = "nginx.ingress.kubernetes.io/ssl-passthrough"

	// ServiceAlphaK8sKeyTolerateUnreadyEndpoints is the resource tolerate unready endpoints key for labels.
	ServiceAlphaK8sKeyTolerateUnreadyEndpoints = "service.alpha.kubernetes.io/tolerate-unready-endpoints"

	// FailureDomainBetaK8sKeyZone is the failure-domain zone key for labels.
	FailureDomainBetaK8sKeyZone = "failure-domain.beta.kubernetes.io/zone"
)

// arogproj.io keys
const (
	// ArgoCDArgoprojKeyName is the annotation on child resources that specifies which ArgoCD instance
	// name a specific object is associated with
	ArgoCDArgoprojKeyName = "argocds.argoproj.io/name"

	// ArgoCDArgoprojKeyNamespace is the annotation on child resources that specifies which ArgoCD instance
	// namespace a specific object is associated with
	ArgoCDArgoprojKeyNamespace = "argocds.argoproj.io/namespace"

	// ArgoCDDeletionFinalizer is a finalizer to implement pre-delete hooks
	ArgoprojKeyFinalizer = "argoproj.io/finalizer"

	// ArgoCDArgoprojKeySecretType is needed for cluster secrets
	ArgoCDArgoprojKeySecretType = "argocd.argoproj.io/secret-type"

	// ArgoCDArgoprojKeyRBACType is the label to describe if what the rbac resource is meant for in non-control plane namespaces
	ArgoCDArgoprojKeyRBACType = "argocd.argoproj.io/rbac-type"

	// ArgoCDManagedByLabel is needed to identify namespace managed by an instance on ArgoCD
	ArgoCDArgoprojKeyManagedBy = "argocd.argoproj.io/managed-by"

	// ArgoCDArgoprojKeyAppsManagedBy is needed to identify namespace mentioned as an app sourceNamespace on ArgoCD
	ArgoCDArgoprojKeyAppsManagedBy = "argocd.argoproj.io/apps-managed-by"

	// ArgoCDArgoprojKeyAppSetsManagedBy is needed to identify namespace mentioned as an appset sourceNamespace on ArgoCD
	ArgoCDArgoprojKeyAppSetsManagedBy = "argocd.argoproj.io/appsets-managed-by"
)

// misc
const (
	TLSSecretNameKey   = "tls-secret-name"
	WantAutoTLSKey     = "wantAutoTLS"
	RedirectURI        = "redirect-uri"
	WantOpenShiftOAuth = "wantOpenShiftOAuth"
)
