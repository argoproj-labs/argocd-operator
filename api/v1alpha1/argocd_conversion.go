package v1alpha1

import (
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

var conversionLogger = ctrl.Log.WithName("conversion-webhook")

// ConvertTo converts this (v1alpha1) ArgoCD to the Hub version (v1beta1).
func (src *ArgoCD) ConvertTo(dstRaw conversion.Hub) error {
<<<<<<< HEAD
	conversionLogger.WithValues("instance", src.Name, "instance-namespace", src.Namespace).V(1).Info("processing v1alpha1 to v1beta1 conversion")
=======
	conversionLogger.Info("v1alpha1 to v1beta1 conversion requested.")
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst := dstRaw.(*v1beta1.ArgoCD)

	// ObjectMeta conversion
	dst.ObjectMeta = src.ObjectMeta

	// Spec conversion

	// sso field
<<<<<<< HEAD
	sso := convertAlphaToBetaSSO(src.Spec.SSO)
=======
	sso := ConvertAlphaToBetaSSO(src.Spec.SSO)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5

	// in case of conflict, deprecated fields will have more priority during conversion to beta
	// deprecated keycloak configs set in alpha (.spec.sso.image, .spec.sso.version, .spec.sso.verifyTLS, .spec.sso.resources),
	// override .spec.sso.keycloak in beta
	if src.Spec.SSO != nil && !reflect.DeepEqual(src.Spec.SSO, &ArgoCDSSOSpec{}) {
		if src.Spec.SSO.Image != "" || src.Spec.SSO.Version != "" || src.Spec.SSO.VerifyTLS != nil || src.Spec.SSO.Resources != nil {
			if sso.Keycloak == nil {
				sso.Keycloak = &v1beta1.ArgoCDKeycloakSpec{}
			}
			sso.Keycloak.Image = src.Spec.SSO.Image
			sso.Keycloak.Version = src.Spec.SSO.Version
			sso.Keycloak.VerifyTLS = src.Spec.SSO.VerifyTLS
			sso.Keycloak.Resources = src.Spec.SSO.Resources
		}
	}

	// deprecated dex configs set in alpha (.spec.dex), override .spec.sso.dex in beta
	if src.Spec.Dex != nil && !reflect.DeepEqual(src.Spec.Dex, &ArgoCDDexSpec{}) && (src.Spec.Dex.Config != "" || src.Spec.Dex.OpenShiftOAuth) {
		if sso == nil {
			sso = &v1beta1.ArgoCDSSOSpec{}
		}
		sso.Provider = v1beta1.SSOProviderTypeDex
		sso.Dex = (*v1beta1.ArgoCDDexSpec)(src.Spec.Dex)
	}

	dst.Spec.SSO = sso

	// rest of the fields
<<<<<<< HEAD
	dst.Spec.ApplicationSet = convertAlphaToBetaApplicationSet(src.Spec.ApplicationSet)
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.ApplicationInstanceLabelKey = src.Spec.ApplicationInstanceLabelKey
	dst.Spec.ConfigManagementPlugins = src.Spec.ConfigManagementPlugins
	dst.Spec.Controller = *convertAlphaToBetaController(&src.Spec.Controller)
=======
	dst.Spec.ApplicationSet = ConvertAlphaToBetaApplicationSet(src.Spec.ApplicationSet)
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.ApplicationInstanceLabelKey = src.Spec.ApplicationInstanceLabelKey
	dst.Spec.ConfigManagementPlugins = src.Spec.ConfigManagementPlugins
	dst.Spec.Controller = *ConvertAlphaToBetaController(&src.Spec.Controller)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst.Spec.DisableAdmin = src.Spec.DisableAdmin
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.GATrackingID = src.Spec.GATrackingID
	dst.Spec.GAAnonymizeUsers = src.Spec.GAAnonymizeUsers
<<<<<<< HEAD
	dst.Spec.Grafana = *convertAlphaToBetaGrafana(&src.Spec.Grafana)
	dst.Spec.HA = *convertAlphaToBetaHA(&src.Spec.HA)
=======
	dst.Spec.Grafana = *ConvertAlphaToBetaGrafana(&src.Spec.Grafana)
	dst.Spec.HA = *ConvertAlphaToBetaHA(&src.Spec.HA)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst.Spec.HelpChatURL = src.Spec.HelpChatURL
	dst.Spec.HelpChatText = src.Spec.HelpChatText
	dst.Spec.Image = src.Spec.Image
	dst.Spec.Import = (*v1beta1.ArgoCDImportSpec)(src.Spec.Import)
	dst.Spec.InitialRepositories = src.Spec.InitialRepositories
	dst.Spec.InitialSSHKnownHosts = v1beta1.SSHHostsSpec(src.Spec.InitialSSHKnownHosts)
	dst.Spec.KustomizeBuildOptions = src.Spec.KustomizeBuildOptions
<<<<<<< HEAD
	dst.Spec.KustomizeVersions = convertAlphaToBetaKustomizeVersions(src.Spec.KustomizeVersions)
=======
	dst.Spec.KustomizeVersions = ConvertAlphaToBetaKustomizeVersions(src.Spec.KustomizeVersions)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst.Spec.OIDCConfig = src.Spec.OIDCConfig
	dst.Spec.Monitoring = v1beta1.ArgoCDMonitoringSpec(src.Spec.Monitoring)
	dst.Spec.NodePlacement = (*v1beta1.ArgoCDNodePlacementSpec)(src.Spec.NodePlacement)
	dst.Spec.Notifications = v1beta1.ArgoCDNotifications(src.Spec.Notifications)
<<<<<<< HEAD
	dst.Spec.Prometheus = *convertAlphaToBetaPrometheus(&src.Spec.Prometheus)
=======
	dst.Spec.Prometheus = *ConvertAlphaToBetaPrometheus(&src.Spec.Prometheus)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst.Spec.RBAC = v1beta1.ArgoCDRBACSpec(src.Spec.RBAC)
	dst.Spec.Redis = v1beta1.ArgoCDRedisSpec(src.Spec.Redis)
	dst.Spec.Repo = v1beta1.ArgoCDRepoSpec(src.Spec.Repo)
	dst.Spec.RepositoryCredentials = src.Spec.RepositoryCredentials
<<<<<<< HEAD
	dst.Spec.ResourceHealthChecks = convertAlphaToBetaResourceHealthChecks(src.Spec.ResourceHealthChecks)
	dst.Spec.ResourceIgnoreDifferences = convertAlphaToBetaResourceIgnoreDifferences(src.Spec.ResourceIgnoreDifferences)
	dst.Spec.ResourceActions = convertAlphaToBetaResourceActions(src.Spec.ResourceActions)
	dst.Spec.ResourceExclusions = src.Spec.ResourceExclusions
	dst.Spec.ResourceInclusions = src.Spec.ResourceInclusions
	dst.Spec.ResourceTrackingMethod = src.Spec.ResourceTrackingMethod
	dst.Spec.Server = *convertAlphaToBetaServer(&src.Spec.Server)
	dst.Spec.SourceNamespaces = src.Spec.SourceNamespaces
	dst.Spec.StatusBadgeEnabled = src.Spec.StatusBadgeEnabled
	dst.Spec.TLS = *convertAlphaToBetaTLS(&src.Spec.TLS)
=======
	dst.Spec.ResourceHealthChecks = ConvertAlphaToBetaResourceHealthChecks(src.Spec.ResourceHealthChecks)
	dst.Spec.ResourceIgnoreDifferences = ConvertAlphaToBetaResourceIgnoreDifferences(src.Spec.ResourceIgnoreDifferences)
	dst.Spec.ResourceActions = ConvertAlphaToBetaResourceActions(src.Spec.ResourceActions)
	dst.Spec.ResourceExclusions = src.Spec.ResourceExclusions
	dst.Spec.ResourceInclusions = src.Spec.ResourceInclusions
	dst.Spec.ResourceTrackingMethod = src.Spec.ResourceTrackingMethod
	dst.Spec.Server = *ConvertAlphaToBetaServer(&src.Spec.Server)
	dst.Spec.SourceNamespaces = src.Spec.SourceNamespaces
	dst.Spec.StatusBadgeEnabled = src.Spec.StatusBadgeEnabled
	dst.Spec.TLS = *ConvertAlphaToBetaTLS(&src.Spec.TLS)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst.Spec.UsersAnonymousEnabled = src.Spec.UsersAnonymousEnabled
	dst.Spec.Version = src.Spec.Version
	dst.Spec.Banner = (*v1beta1.Banner)(src.Spec.Banner)

	// Status conversion
	dst.Status = v1beta1.ArgoCDStatus(src.Status)

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this (v1alpha1) version.
func (dst *ArgoCD) ConvertFrom(srcRaw conversion.Hub) error {
<<<<<<< HEAD
	conversionLogger.WithValues("instance", dst.Name, "instance-namespace", dst.Namespace).V(1).Info("processing v1beta1 to v1alpha1 conversion")
=======
	conversionLogger.Info("v1beta1 to v1alpha1 conversion requested.")
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5

	src := srcRaw.(*v1beta1.ArgoCD)

	// ObjectMeta conversion
	dst.ObjectMeta = src.ObjectMeta

	// Spec conversion

	// sso field
	// ignoring conversions of sso fields from v1beta1 to deprecated v1alpha1 as
	// there is no data loss since the new fields in v1beta1 are also present in v1alpha1 &
	// v1alpha1 is not used in business logic & only exists for presentation
<<<<<<< HEAD
	sso := convertBetaToAlphaSSO(src.Spec.SSO)
	dst.Spec.SSO = sso

	// rest of the fields
	dst.Spec.ApplicationSet = convertBetaToAlphaApplicationSet(src.Spec.ApplicationSet)
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.ApplicationInstanceLabelKey = src.Spec.ApplicationInstanceLabelKey
	dst.Spec.ConfigManagementPlugins = src.Spec.ConfigManagementPlugins
	dst.Spec.Controller = *convertBetaToAlphaController(&src.Spec.Controller)
=======
	sso := ConvertBetaToAlphaSSO(src.Spec.SSO)
	dst.Spec.SSO = sso

	// rest of the fields
	dst.Spec.ApplicationSet = ConvertBetaToAlphaApplicationSet(src.Spec.ApplicationSet)
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.ApplicationInstanceLabelKey = src.Spec.ApplicationInstanceLabelKey
	dst.Spec.ConfigManagementPlugins = src.Spec.ConfigManagementPlugins
	dst.Spec.Controller = *ConvertBetaToAlphaController(&src.Spec.Controller)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst.Spec.DisableAdmin = src.Spec.DisableAdmin
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.GATrackingID = src.Spec.GATrackingID
	dst.Spec.GAAnonymizeUsers = src.Spec.GAAnonymizeUsers
<<<<<<< HEAD
	dst.Spec.Grafana = *convertBetaToAlphaGrafana(&src.Spec.Grafana)
	dst.Spec.HA = *convertBetaToAlphaHA(&src.Spec.HA)
=======
	dst.Spec.Grafana = *ConvertBetaToAlphaGrafana(&src.Spec.Grafana)
	dst.Spec.HA = *ConvertBetaToAlphaHA(&src.Spec.HA)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst.Spec.HelpChatURL = src.Spec.HelpChatURL
	dst.Spec.HelpChatText = src.Spec.HelpChatText
	dst.Spec.Image = src.Spec.Image
	dst.Spec.Import = (*ArgoCDImportSpec)(src.Spec.Import)
	dst.Spec.InitialRepositories = src.Spec.InitialRepositories
	dst.Spec.InitialSSHKnownHosts = SSHHostsSpec(src.Spec.InitialSSHKnownHosts)
	dst.Spec.KustomizeBuildOptions = src.Spec.KustomizeBuildOptions
<<<<<<< HEAD
	dst.Spec.KustomizeVersions = convertBetaToAlphaKustomizeVersions(src.Spec.KustomizeVersions)
=======
	dst.Spec.KustomizeVersions = ConvertBetaToAlphaKustomizeVersions(src.Spec.KustomizeVersions)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst.Spec.OIDCConfig = src.Spec.OIDCConfig
	dst.Spec.Monitoring = ArgoCDMonitoringSpec(src.Spec.Monitoring)
	dst.Spec.NodePlacement = (*ArgoCDNodePlacementSpec)(src.Spec.NodePlacement)
	dst.Spec.Notifications = ArgoCDNotifications(src.Spec.Notifications)
<<<<<<< HEAD
	dst.Spec.Prometheus = *convertBetaToAlphaPrometheus(&src.Spec.Prometheus)
=======
	dst.Spec.Prometheus = *ConvertBetaToAlphaPrometheus(&src.Spec.Prometheus)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst.Spec.RBAC = ArgoCDRBACSpec(src.Spec.RBAC)
	dst.Spec.Redis = ArgoCDRedisSpec(src.Spec.Redis)
	dst.Spec.Repo = ArgoCDRepoSpec(src.Spec.Repo)
	dst.Spec.RepositoryCredentials = src.Spec.RepositoryCredentials
<<<<<<< HEAD
	dst.Spec.ResourceHealthChecks = convertBetaToAlphaResourceHealthChecks(src.Spec.ResourceHealthChecks)
	dst.Spec.ResourceIgnoreDifferences = convertBetaToAlphaResourceIgnoreDifferences(src.Spec.ResourceIgnoreDifferences)
	dst.Spec.ResourceActions = convertBetaToAlphaResourceActions(src.Spec.ResourceActions)
	dst.Spec.ResourceExclusions = src.Spec.ResourceExclusions
	dst.Spec.ResourceInclusions = src.Spec.ResourceInclusions
	dst.Spec.ResourceTrackingMethod = src.Spec.ResourceTrackingMethod
	dst.Spec.Server = *convertBetaToAlphaServer(&src.Spec.Server)
	dst.Spec.SourceNamespaces = src.Spec.SourceNamespaces
	dst.Spec.StatusBadgeEnabled = src.Spec.StatusBadgeEnabled
	dst.Spec.TLS = *convertBetaToAlphaTLS(&src.Spec.TLS)
=======
	dst.Spec.ResourceHealthChecks = ConvertBetaToAlphaResourceHealthChecks(src.Spec.ResourceHealthChecks)
	dst.Spec.ResourceIgnoreDifferences = ConvertBetaToAlphaResourceIgnoreDifferences(src.Spec.ResourceIgnoreDifferences)
	dst.Spec.ResourceActions = ConvertBetaToAlphaResourceActions(src.Spec.ResourceActions)
	dst.Spec.ResourceExclusions = src.Spec.ResourceExclusions
	dst.Spec.ResourceInclusions = src.Spec.ResourceInclusions
	dst.Spec.ResourceTrackingMethod = src.Spec.ResourceTrackingMethod
	dst.Spec.Server = *ConvertBetaToAlphaServer(&src.Spec.Server)
	dst.Spec.SourceNamespaces = src.Spec.SourceNamespaces
	dst.Spec.StatusBadgeEnabled = src.Spec.StatusBadgeEnabled
	dst.Spec.TLS = *ConvertBetaToAlphaTLS(&src.Spec.TLS)
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	dst.Spec.UsersAnonymousEnabled = src.Spec.UsersAnonymousEnabled
	dst.Spec.Version = src.Spec.Version
	dst.Spec.Banner = (*Banner)(src.Spec.Banner)

	// Status conversion
	dst.Status = ArgoCDStatus(src.Status)

	return nil
}

// Conversion funcs for v1alpha1 to v1beta1.
<<<<<<< HEAD
func convertAlphaToBetaController(src *ArgoCDApplicationControllerSpec) *v1beta1.ArgoCDApplicationControllerSpec {
=======
func ConvertAlphaToBetaController(src *ArgoCDApplicationControllerSpec) *v1beta1.ArgoCDApplicationControllerSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.ArgoCDApplicationControllerSpec
	if src != nil {
		dst = &v1beta1.ArgoCDApplicationControllerSpec{
			Processors:       v1beta1.ArgoCDApplicationControllerProcessorsSpec(src.Processors),
			LogLevel:         src.LogLevel,
			LogFormat:        src.LogFormat,
			Resources:        src.Resources,
			ParallelismLimit: src.ParallelismLimit,
			AppSync:          src.AppSync,
			Sharding:         v1beta1.ArgoCDApplicationControllerShardSpec(src.Sharding),
			Env:              src.Env,
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaWebhookServer(src *WebhookServerSpec) *v1beta1.WebhookServerSpec {
=======
func ConvertAlphaToBetaWebhookServer(src *WebhookServerSpec) *v1beta1.WebhookServerSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.WebhookServerSpec
	if src != nil {
		dst = &v1beta1.WebhookServerSpec{
			Host:    src.Host,
			Ingress: v1beta1.ArgoCDIngressSpec(src.Ingress),
			Route:   v1beta1.ArgoCDRouteSpec(src.Route),
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaApplicationSet(src *ArgoCDApplicationSet) *v1beta1.ArgoCDApplicationSet {
=======
func ConvertAlphaToBetaApplicationSet(src *ArgoCDApplicationSet) *v1beta1.ArgoCDApplicationSet {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.ArgoCDApplicationSet
	if src != nil {
		dst = &v1beta1.ArgoCDApplicationSet{
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
			Image:            src.Image,
			Version:          src.Version,
			Resources:        src.Resources,
			LogLevel:         src.LogLevel,
<<<<<<< HEAD
			WebhookServer:    *convertAlphaToBetaWebhookServer(&src.WebhookServer),
=======
			WebhookServer:    *ConvertAlphaToBetaWebhookServer(&src.WebhookServer),
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaGrafana(src *ArgoCDGrafanaSpec) *v1beta1.ArgoCDGrafanaSpec {
=======
func ConvertAlphaToBetaGrafana(src *ArgoCDGrafanaSpec) *v1beta1.ArgoCDGrafanaSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.ArgoCDGrafanaSpec
	if src != nil {
		dst = &v1beta1.ArgoCDGrafanaSpec{
			Enabled: src.Enabled,
			Host:    src.Host,
			Image:   src.Image,
			Ingress: v1beta1.ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaPrometheus(src *ArgoCDPrometheusSpec) *v1beta1.ArgoCDPrometheusSpec {
=======
func ConvertAlphaToBetaPrometheus(src *ArgoCDPrometheusSpec) *v1beta1.ArgoCDPrometheusSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.ArgoCDPrometheusSpec
	if src != nil {
		dst = &v1beta1.ArgoCDPrometheusSpec{
			Enabled: src.Enabled,
			Host:    src.Host,
			Ingress: v1beta1.ArgoCDIngressSpec(src.Ingress),
			Route:   v1beta1.ArgoCDRouteSpec(src.Route),
			Size:    src.Size,
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaSSO(src *ArgoCDSSOSpec) *v1beta1.ArgoCDSSOSpec {
=======
func ConvertAlphaToBetaSSO(src *ArgoCDSSOSpec) *v1beta1.ArgoCDSSOSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.ArgoCDSSOSpec
	if src != nil {
		dst = &v1beta1.ArgoCDSSOSpec{
			Provider: v1beta1.SSOProviderType(src.Provider),
			Dex:      (*v1beta1.ArgoCDDexSpec)(src.Dex),
			Keycloak: (*v1beta1.ArgoCDKeycloakSpec)(src.Keycloak),
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaHA(src *ArgoCDHASpec) *v1beta1.ArgoCDHASpec {
=======
func ConvertAlphaToBetaHA(src *ArgoCDHASpec) *v1beta1.ArgoCDHASpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.ArgoCDHASpec
	if src != nil {
		dst = &v1beta1.ArgoCDHASpec{
			Enabled:           src.Enabled,
			RedisProxyImage:   src.RedisProxyImage,
			RedisProxyVersion: src.RedisProxyVersion,
			Resources:         src.Resources,
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaTLS(src *ArgoCDTLSSpec) *v1beta1.ArgoCDTLSSpec {
=======
func ConvertAlphaToBetaTLS(src *ArgoCDTLSSpec) *v1beta1.ArgoCDTLSSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.ArgoCDTLSSpec
	if src != nil {
		dst = &v1beta1.ArgoCDTLSSpec{
			CA:           v1beta1.ArgoCDCASpec(src.CA),
			InitialCerts: src.InitialCerts,
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaServer(src *ArgoCDServerSpec) *v1beta1.ArgoCDServerSpec {
=======
func ConvertAlphaToBetaServer(src *ArgoCDServerSpec) *v1beta1.ArgoCDServerSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.ArgoCDServerSpec
	if src != nil {
		dst = &v1beta1.ArgoCDServerSpec{
			Autoscale:        v1beta1.ArgoCDServerAutoscaleSpec(src.Autoscale),
<<<<<<< HEAD
			GRPC:             *convertAlphaToBetaGRPC(&src.GRPC),
=======
			GRPC:             *ConvertAlphaToBetaGRPC(&src.GRPC),
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
			Host:             src.Host,
			Ingress:          v1beta1.ArgoCDIngressSpec(src.Ingress),
			Insecure:         src.Insecure,
			LogLevel:         src.LogLevel,
			LogFormat:        src.LogFormat,
			Replicas:         src.Replicas,
			Resources:        src.Resources,
			Route:            v1beta1.ArgoCDRouteSpec(src.Route),
			Service:          v1beta1.ArgoCDServerServiceSpec(src.Service),
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaGRPC(src *ArgoCDServerGRPCSpec) *v1beta1.ArgoCDServerGRPCSpec {
=======
func ConvertAlphaToBetaGRPC(src *ArgoCDServerGRPCSpec) *v1beta1.ArgoCDServerGRPCSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.ArgoCDServerGRPCSpec
	if src != nil {
		dst = &v1beta1.ArgoCDServerGRPCSpec{
			Host:    src.Host,
			Ingress: v1beta1.ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaKustomizeVersions(src []KustomizeVersionSpec) []v1beta1.KustomizeVersionSpec {
=======
func ConvertAlphaToBetaKustomizeVersions(src []KustomizeVersionSpec) []v1beta1.KustomizeVersionSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst []v1beta1.KustomizeVersionSpec
	for _, s := range src {
		dst = append(dst, v1beta1.KustomizeVersionSpec{
			Version: s.Version,
			Path:    s.Path,
		},
		)
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaResourceIgnoreDifferences(src *ResourceIgnoreDifference) *v1beta1.ResourceIgnoreDifference {
=======
func ConvertAlphaToBetaResourceIgnoreDifferences(src *ResourceIgnoreDifference) *v1beta1.ResourceIgnoreDifference {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *v1beta1.ResourceIgnoreDifference
	if src != nil {
		dst = &v1beta1.ResourceIgnoreDifference{
			All:                 (*v1beta1.IgnoreDifferenceCustomization)(src.All),
<<<<<<< HEAD
			ResourceIdentifiers: convertAlphaToBetaResourceIdentifiers(src.ResourceIdentifiers),
=======
			ResourceIdentifiers: ConvertAlphaToBetaResourceIdentifiers(src.ResourceIdentifiers),
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
		}
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaResourceIdentifiers(src []ResourceIdentifiers) []v1beta1.ResourceIdentifiers {
=======
func ConvertAlphaToBetaResourceIdentifiers(src []ResourceIdentifiers) []v1beta1.ResourceIdentifiers {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst []v1beta1.ResourceIdentifiers
	for _, s := range src {
		dst = append(dst, v1beta1.ResourceIdentifiers{
			Group:         s.Group,
			Kind:          s.Kind,
			Customization: v1beta1.IgnoreDifferenceCustomization(s.Customization),
		},
		)
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaResourceActions(src []ResourceAction) []v1beta1.ResourceAction {
=======
func ConvertAlphaToBetaResourceActions(src []ResourceAction) []v1beta1.ResourceAction {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst []v1beta1.ResourceAction
	for _, s := range src {
		dst = append(dst, v1beta1.ResourceAction{
			Group:  s.Group,
			Kind:   s.Kind,
			Action: s.Action,
		},
		)
	}
	return dst
}

<<<<<<< HEAD
func convertAlphaToBetaResourceHealthChecks(src []ResourceHealthCheck) []v1beta1.ResourceHealthCheck {
=======
func ConvertAlphaToBetaResourceHealthChecks(src []ResourceHealthCheck) []v1beta1.ResourceHealthCheck {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst []v1beta1.ResourceHealthCheck
	for _, s := range src {
		dst = append(dst, v1beta1.ResourceHealthCheck{
			Group: s.Group,
			Kind:  s.Kind,
			Check: s.Check,
		},
		)
	}
	return dst
}

// Conversion funcs for v1beta1 to v1alpha1.
<<<<<<< HEAD
func convertBetaToAlphaController(src *v1beta1.ArgoCDApplicationControllerSpec) *ArgoCDApplicationControllerSpec {
=======
func ConvertBetaToAlphaController(src *v1beta1.ArgoCDApplicationControllerSpec) *ArgoCDApplicationControllerSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *ArgoCDApplicationControllerSpec
	if src != nil {
		dst = &ArgoCDApplicationControllerSpec{
			Processors:       ArgoCDApplicationControllerProcessorsSpec(src.Processors),
			LogLevel:         src.LogLevel,
			LogFormat:        src.LogFormat,
			Resources:        src.Resources,
			ParallelismLimit: src.ParallelismLimit,
			AppSync:          src.AppSync,
			Sharding:         ArgoCDApplicationControllerShardSpec(src.Sharding),
			Env:              src.Env,
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaWebhookServer(src *v1beta1.WebhookServerSpec) *WebhookServerSpec {
=======
func ConvertBetaToAlphaWebhookServer(src *v1beta1.WebhookServerSpec) *WebhookServerSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *WebhookServerSpec
	if src != nil {
		dst = &WebhookServerSpec{
			Host:    src.Host,
			Ingress: ArgoCDIngressSpec(src.Ingress),
			Route:   ArgoCDRouteSpec(src.Route),
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaApplicationSet(src *v1beta1.ArgoCDApplicationSet) *ArgoCDApplicationSet {
=======
func ConvertBetaToAlphaApplicationSet(src *v1beta1.ArgoCDApplicationSet) *ArgoCDApplicationSet {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *ArgoCDApplicationSet
	if src != nil {
		dst = &ArgoCDApplicationSet{
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
			Image:            src.Image,
			Version:          src.Version,
			Resources:        src.Resources,
			LogLevel:         src.LogLevel,
<<<<<<< HEAD
			WebhookServer:    *convertBetaToAlphaWebhookServer(&src.WebhookServer),
=======
			WebhookServer:    *ConvertBetaToAlphaWebhookServer(&src.WebhookServer),
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaGrafana(src *v1beta1.ArgoCDGrafanaSpec) *ArgoCDGrafanaSpec {
=======
func ConvertBetaToAlphaGrafana(src *v1beta1.ArgoCDGrafanaSpec) *ArgoCDGrafanaSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *ArgoCDGrafanaSpec
	if src != nil {
		dst = &ArgoCDGrafanaSpec{
			Enabled: src.Enabled,
			Host:    src.Host,
			Image:   src.Image,
			Ingress: ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaPrometheus(src *v1beta1.ArgoCDPrometheusSpec) *ArgoCDPrometheusSpec {
=======
func ConvertBetaToAlphaPrometheus(src *v1beta1.ArgoCDPrometheusSpec) *ArgoCDPrometheusSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *ArgoCDPrometheusSpec
	if src != nil {
		dst = &ArgoCDPrometheusSpec{
			Enabled: src.Enabled,
			Host:    src.Host,
			Ingress: ArgoCDIngressSpec(src.Ingress),
			Route:   ArgoCDRouteSpec(src.Route),
			Size:    src.Size,
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaSSO(src *v1beta1.ArgoCDSSOSpec) *ArgoCDSSOSpec {
=======
func ConvertBetaToAlphaSSO(src *v1beta1.ArgoCDSSOSpec) *ArgoCDSSOSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *ArgoCDSSOSpec
	if src != nil {
		dst = &ArgoCDSSOSpec{
			Provider: SSOProviderType(src.Provider),
			Dex:      (*ArgoCDDexSpec)(src.Dex),
			Keycloak: (*ArgoCDKeycloakSpec)(src.Keycloak),
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaHA(src *v1beta1.ArgoCDHASpec) *ArgoCDHASpec {
=======
func ConvertBetaToAlphaHA(src *v1beta1.ArgoCDHASpec) *ArgoCDHASpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *ArgoCDHASpec
	if src != nil {
		dst = &ArgoCDHASpec{
			Enabled:           src.Enabled,
			RedisProxyImage:   src.RedisProxyImage,
			RedisProxyVersion: src.RedisProxyVersion,
			Resources:         src.Resources,
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaTLS(src *v1beta1.ArgoCDTLSSpec) *ArgoCDTLSSpec {
=======
func ConvertBetaToAlphaTLS(src *v1beta1.ArgoCDTLSSpec) *ArgoCDTLSSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *ArgoCDTLSSpec
	if src != nil {
		dst = &ArgoCDTLSSpec{
			CA:           ArgoCDCASpec(src.CA),
			InitialCerts: src.InitialCerts,
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaServer(src *v1beta1.ArgoCDServerSpec) *ArgoCDServerSpec {
=======
func ConvertBetaToAlphaServer(src *v1beta1.ArgoCDServerSpec) *ArgoCDServerSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *ArgoCDServerSpec
	if src != nil {
		dst = &ArgoCDServerSpec{
			Autoscale:        ArgoCDServerAutoscaleSpec(src.Autoscale),
<<<<<<< HEAD
			GRPC:             *convertBetaToAlphaGRPC(&src.GRPC),
=======
			GRPC:             *ConvertBetaToAlphaGRPC(&src.GRPC),
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
			Host:             src.Host,
			Ingress:          ArgoCDIngressSpec(src.Ingress),
			Insecure:         src.Insecure,
			LogLevel:         src.LogLevel,
			LogFormat:        src.LogFormat,
			Replicas:         src.Replicas,
			Resources:        src.Resources,
			Route:            ArgoCDRouteSpec(src.Route),
			Service:          ArgoCDServerServiceSpec(src.Service),
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaGRPC(src *v1beta1.ArgoCDServerGRPCSpec) *ArgoCDServerGRPCSpec {
=======
func ConvertBetaToAlphaGRPC(src *v1beta1.ArgoCDServerGRPCSpec) *ArgoCDServerGRPCSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *ArgoCDServerGRPCSpec
	if src != nil {
		dst = &ArgoCDServerGRPCSpec{
			Host:    src.Host,
			Ingress: ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaKustomizeVersions(src []v1beta1.KustomizeVersionSpec) []KustomizeVersionSpec {
=======
func ConvertBetaToAlphaKustomizeVersions(src []v1beta1.KustomizeVersionSpec) []KustomizeVersionSpec {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst []KustomizeVersionSpec
	for _, s := range src {
		dst = append(dst, KustomizeVersionSpec{
			Version: s.Version,
			Path:    s.Path,
		},
		)
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaResourceIgnoreDifferences(src *v1beta1.ResourceIgnoreDifference) *ResourceIgnoreDifference {
=======
func ConvertBetaToAlphaResourceIgnoreDifferences(src *v1beta1.ResourceIgnoreDifference) *ResourceIgnoreDifference {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst *ResourceIgnoreDifference
	if src != nil {
		dst = &ResourceIgnoreDifference{
			All:                 (*IgnoreDifferenceCustomization)(src.All),
<<<<<<< HEAD
			ResourceIdentifiers: convertBetaToAlphaResourceIdentifiers(src.ResourceIdentifiers),
=======
			ResourceIdentifiers: ConvertBetaToAlphaResourceIdentifiers(src.ResourceIdentifiers),
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
		}
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaResourceIdentifiers(src []v1beta1.ResourceIdentifiers) []ResourceIdentifiers {
=======
func ConvertBetaToAlphaResourceIdentifiers(src []v1beta1.ResourceIdentifiers) []ResourceIdentifiers {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst []ResourceIdentifiers
	for _, s := range src {
		dst = append(dst, ResourceIdentifiers{
			Group:         s.Group,
			Kind:          s.Kind,
			Customization: IgnoreDifferenceCustomization(s.Customization),
		},
		)
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaResourceActions(src []v1beta1.ResourceAction) []ResourceAction {
=======
func ConvertBetaToAlphaResourceActions(src []v1beta1.ResourceAction) []ResourceAction {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst []ResourceAction
	for _, s := range src {
		dst = append(dst, ResourceAction{
			Group:  s.Group,
			Kind:   s.Kind,
			Action: s.Action,
		},
		)
	}
	return dst
}

<<<<<<< HEAD
func convertBetaToAlphaResourceHealthChecks(src []v1beta1.ResourceHealthCheck) []ResourceHealthCheck {
=======
func ConvertBetaToAlphaResourceHealthChecks(src []v1beta1.ResourceHealthCheck) []ResourceHealthCheck {
>>>>>>> 75d6cf4d3e7f0c1f5e024a43e669bba4e4dae7a5
	var dst []ResourceHealthCheck
	for _, s := range src {
		dst = append(dst, ResourceHealthCheck{
			Group: s.Group,
			Kind:  s.Kind,
			Check: s.Check,
		},
		)
	}
	return dst
}
