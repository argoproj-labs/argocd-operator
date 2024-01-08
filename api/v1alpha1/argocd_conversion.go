package v1alpha1

import (
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

<<<<<<< HEAD
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
=======
	"github.com/argoproj-labs/argocd-operator/api/v1beta1"
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
)

var conversionLogger = ctrl.Log.WithName("conversion-webhook")

// ConvertTo converts this (v1alpha1) ArgoCD to the Hub version (v1beta1).
func (src *ArgoCD) ConvertTo(dstRaw conversion.Hub) error {
	conversionLogger.Info("v1alpha1 to v1beta1 conversion requested.")
<<<<<<< HEAD
	dst := dstRaw.(*argoproj.ArgoCD)
=======
	dst := dstRaw.(*v1beta1.ArgoCD)
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d

	// ObjectMeta conversion
	dst.ObjectMeta = src.ObjectMeta

	// Spec conversion

	// sso field
	sso := ConvertAlphaToBetaSSO(src.Spec.SSO)

	// in case of conflict, deprecated fields will have more priority during conversion to beta
	// deprecated keycloak configs set in alpha (.spec.sso.image, .spec.sso.version, .spec.sso.verifyTLS, .spec.sso.resources),
	// override .spec.sso.keycloak in beta
	if src.Spec.SSO != nil && !reflect.DeepEqual(src.Spec.SSO, &ArgoCDSSOSpec{}) {
		if src.Spec.SSO.Image != "" || src.Spec.SSO.Version != "" || src.Spec.SSO.VerifyTLS != nil || src.Spec.SSO.Resources != nil {
			if sso.Keycloak == nil {
<<<<<<< HEAD
				sso.Keycloak = &argoproj.ArgoCDKeycloakSpec{}
=======
				sso.Keycloak = &v1beta1.ArgoCDKeycloakSpec{}
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
<<<<<<< HEAD
			sso = &argoproj.ArgoCDSSOSpec{}
		}
		sso.Provider = argoproj.SSOProviderTypeDex
=======
			sso = &v1beta1.ArgoCDSSOSpec{}
		}
		sso.Provider = v1beta1.SSOProviderTypeDex
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
		sso.Dex = ConvertAlphaToBetaDex(src.Spec.Dex)
	}

	dst.Spec.SSO = sso

	// rest of the fields
	dst.Spec.ApplicationSet = ConvertAlphaToBetaApplicationSet(src.Spec.ApplicationSet)
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.ApplicationInstanceLabelKey = src.Spec.ApplicationInstanceLabelKey
	dst.Spec.ConfigManagementPlugins = src.Spec.ConfigManagementPlugins
	dst.Spec.Controller = *ConvertAlphaToBetaController(&src.Spec.Controller)
	dst.Spec.DisableAdmin = src.Spec.DisableAdmin
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.GATrackingID = src.Spec.GATrackingID
	dst.Spec.GAAnonymizeUsers = src.Spec.GAAnonymizeUsers
	dst.Spec.Grafana = *ConvertAlphaToBetaGrafana(&src.Spec.Grafana)
	dst.Spec.HA = *ConvertAlphaToBetaHA(&src.Spec.HA)
	dst.Spec.HelpChatURL = src.Spec.HelpChatURL
	dst.Spec.HelpChatText = src.Spec.HelpChatText
	dst.Spec.Image = src.Spec.Image
<<<<<<< HEAD
	dst.Spec.Import = (*argoproj.ArgoCDImportSpec)(src.Spec.Import)
	dst.Spec.InitialRepositories = src.Spec.InitialRepositories
	dst.Spec.InitialSSHKnownHosts = argoproj.SSHHostsSpec(src.Spec.InitialSSHKnownHosts)
	dst.Spec.KustomizeBuildOptions = src.Spec.KustomizeBuildOptions
	dst.Spec.KustomizeVersions = ConvertAlphaToBetaKustomizeVersions(src.Spec.KustomizeVersions)
	dst.Spec.OIDCConfig = src.Spec.OIDCConfig
	dst.Spec.Monitoring = argoproj.ArgoCDMonitoringSpec(src.Spec.Monitoring)
	dst.Spec.NodePlacement = (*argoproj.ArgoCDNodePlacementSpec)(src.Spec.NodePlacement)
	dst.Spec.Notifications = argoproj.ArgoCDNotifications(src.Spec.Notifications)
	dst.Spec.Prometheus = *ConvertAlphaToBetaPrometheus(&src.Spec.Prometheus)
	dst.Spec.RBAC = argoproj.ArgoCDRBACSpec(src.Spec.RBAC)
=======
	dst.Spec.Import = (*v1beta1.ArgoCDImportSpec)(src.Spec.Import)
	dst.Spec.InitialRepositories = src.Spec.InitialRepositories
	dst.Spec.InitialSSHKnownHosts = v1beta1.SSHHostsSpec(src.Spec.InitialSSHKnownHosts)
	dst.Spec.KustomizeBuildOptions = src.Spec.KustomizeBuildOptions
	dst.Spec.KustomizeVersions = ConvertAlphaToBetaKustomizeVersions(src.Spec.KustomizeVersions)
	dst.Spec.OIDCConfig = src.Spec.OIDCConfig
	dst.Spec.Monitoring = v1beta1.ArgoCDMonitoringSpec(src.Spec.Monitoring)
	dst.Spec.NodePlacement = (*v1beta1.ArgoCDNodePlacementSpec)(src.Spec.NodePlacement)
	dst.Spec.Notifications = v1beta1.ArgoCDNotifications(src.Spec.Notifications)
	dst.Spec.Prometheus = *ConvertAlphaToBetaPrometheus(&src.Spec.Prometheus)
	dst.Spec.RBAC = v1beta1.ArgoCDRBACSpec(src.Spec.RBAC)
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
	dst.Spec.Redis = *ConvertAlphaToBetaRedis(&src.Spec.Redis)
	dst.Spec.Repo = *ConvertAlphaToBetaRepo(&src.Spec.Repo)
	dst.Spec.RepositoryCredentials = src.Spec.RepositoryCredentials
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
	dst.Spec.UsersAnonymousEnabled = src.Spec.UsersAnonymousEnabled
	dst.Spec.Version = src.Spec.Version
<<<<<<< HEAD
	dst.Spec.Banner = (*argoproj.Banner)(src.Spec.Banner)

	// Status conversion
	dst.Status = argoproj.ArgoCDStatus(src.Status)
=======
	dst.Spec.Banner = (*v1beta1.Banner)(src.Spec.Banner)

	// Status conversion
	dst.Status = v1beta1.ArgoCDStatus(src.Status)
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this (v1alpha1) version.
func (dst *ArgoCD) ConvertFrom(srcRaw conversion.Hub) error {
	conversionLogger.Info("v1beta1 to v1alpha1 conversion requested.")

<<<<<<< HEAD
	src := srcRaw.(*argoproj.ArgoCD)
=======
	src := srcRaw.(*v1beta1.ArgoCD)
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d

	// ObjectMeta conversion
	dst.ObjectMeta = src.ObjectMeta

	// Spec conversion

	// sso field
	// ignoring conversions of sso fields from v1beta1 to deprecated v1alpha1 as
	// there is no data loss since the new fields in v1beta1 are also present in v1alpha1 &
	// v1alpha1 is not used in business logic & only exists for presentation
	sso := ConvertBetaToAlphaSSO(src.Spec.SSO)
	dst.Spec.SSO = sso

	// rest of the fields
	dst.Spec.ApplicationSet = ConvertBetaToAlphaApplicationSet(src.Spec.ApplicationSet)
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.ApplicationInstanceLabelKey = src.Spec.ApplicationInstanceLabelKey
	dst.Spec.ConfigManagementPlugins = src.Spec.ConfigManagementPlugins
	dst.Spec.Controller = *ConvertBetaToAlphaController(&src.Spec.Controller)
	dst.Spec.DisableAdmin = src.Spec.DisableAdmin
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.GATrackingID = src.Spec.GATrackingID
	dst.Spec.GAAnonymizeUsers = src.Spec.GAAnonymizeUsers
	dst.Spec.Grafana = *ConvertBetaToAlphaGrafana(&src.Spec.Grafana)
	dst.Spec.HA = *ConvertBetaToAlphaHA(&src.Spec.HA)
	dst.Spec.HelpChatURL = src.Spec.HelpChatURL
	dst.Spec.HelpChatText = src.Spec.HelpChatText
	dst.Spec.Image = src.Spec.Image
	dst.Spec.Import = (*ArgoCDImportSpec)(src.Spec.Import)
	dst.Spec.InitialRepositories = src.Spec.InitialRepositories
	dst.Spec.InitialSSHKnownHosts = SSHHostsSpec(src.Spec.InitialSSHKnownHosts)
	dst.Spec.KustomizeBuildOptions = src.Spec.KustomizeBuildOptions
	dst.Spec.KustomizeVersions = ConvertBetaToAlphaKustomizeVersions(src.Spec.KustomizeVersions)
	dst.Spec.OIDCConfig = src.Spec.OIDCConfig
	dst.Spec.Monitoring = ArgoCDMonitoringSpec(src.Spec.Monitoring)
	dst.Spec.NodePlacement = (*ArgoCDNodePlacementSpec)(src.Spec.NodePlacement)
	dst.Spec.Notifications = ArgoCDNotifications(src.Spec.Notifications)
	dst.Spec.Prometheus = *ConvertBetaToAlphaPrometheus(&src.Spec.Prometheus)
	dst.Spec.RBAC = ArgoCDRBACSpec(src.Spec.RBAC)
	dst.Spec.Redis = *ConvertBetaToAlphaRedis(&src.Spec.Redis)
	dst.Spec.Repo = *ConvertBetaToAlphaRepo(&src.Spec.Repo)
	dst.Spec.RepositoryCredentials = src.Spec.RepositoryCredentials
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
	dst.Spec.UsersAnonymousEnabled = src.Spec.UsersAnonymousEnabled
	dst.Spec.Version = src.Spec.Version
	dst.Spec.Banner = (*Banner)(src.Spec.Banner)

	// Status conversion
	dst.Status = ArgoCDStatus(src.Status)

	return nil
}

// Conversion funcs for v1alpha1 to v1beta1.
<<<<<<< HEAD
func ConvertAlphaToBetaController(src *ArgoCDApplicationControllerSpec) *argoproj.ArgoCDApplicationControllerSpec {
	var dst *argoproj.ArgoCDApplicationControllerSpec
	if src != nil {
		dst = &argoproj.ArgoCDApplicationControllerSpec{
			Processors:       argoproj.ArgoCDApplicationControllerProcessorsSpec(src.Processors),
=======
func ConvertAlphaToBetaController(src *ArgoCDApplicationControllerSpec) *v1beta1.ArgoCDApplicationControllerSpec {
	var dst *v1beta1.ArgoCDApplicationControllerSpec
	if src != nil {
		dst = &v1beta1.ArgoCDApplicationControllerSpec{
			Processors:       v1beta1.ArgoCDApplicationControllerProcessorsSpec(src.Processors),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			LogLevel:         src.LogLevel,
			LogFormat:        src.LogFormat,
			Resources:        src.Resources,
			ParallelismLimit: src.ParallelismLimit,
			AppSync:          src.AppSync,
<<<<<<< HEAD
			Sharding:         argoproj.ArgoCDApplicationControllerShardSpec(src.Sharding),
=======
			Sharding:         v1beta1.ArgoCDApplicationControllerShardSpec(src.Sharding),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			Env:              src.Env,
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaRedis(src *ArgoCDRedisSpec) *argoproj.ArgoCDRedisSpec {
	var dst *argoproj.ArgoCDRedisSpec
	if src != nil {
		dst = &argoproj.ArgoCDRedisSpec{
=======
func ConvertAlphaToBetaRedis(src *ArgoCDRedisSpec) *v1beta1.ArgoCDRedisSpec {
	var dst *v1beta1.ArgoCDRedisSpec
	if src != nil {
		dst = &v1beta1.ArgoCDRedisSpec{
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			AutoTLS:                src.AutoTLS,
			DisableTLSVerification: src.DisableTLSVerification,
			Image:                  src.Image,
			Resources:              src.Resources,
			Version:                src.Version,
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaRepo(src *ArgoCDRepoSpec) *argoproj.ArgoCDRepoSpec {
	var dst *argoproj.ArgoCDRepoSpec
	if src != nil {
		dst = &argoproj.ArgoCDRepoSpec{
=======
func ConvertAlphaToBetaRepo(src *ArgoCDRepoSpec) *v1beta1.ArgoCDRepoSpec {
	var dst *v1beta1.ArgoCDRepoSpec
	if src != nil {
		dst = &v1beta1.ArgoCDRepoSpec{
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			AutoTLS:              src.AutoTLS,
			Env:                  src.Env,
			ExecTimeout:          src.ExecTimeout,
			ExtraRepoCommandArgs: src.ExtraRepoCommandArgs,
			Image:                src.Image,
			InitContainers:       src.InitContainers,
			LogFormat:            src.LogFormat,
			LogLevel:             src.LogLevel,
			MountSAToken:         src.MountSAToken,
			Replicas:             src.Replicas,
			Resources:            src.Resources,
			ServiceAccount:       src.ServiceAccount,
			SidecarContainers:    src.SidecarContainers,
			VerifyTLS:            src.VerifyTLS,
			Version:              src.Version,
			VolumeMounts:         src.VolumeMounts,
			Volumes:              src.Volumes,
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaWebhookServer(src *WebhookServerSpec) *argoproj.WebhookServerSpec {
	var dst *argoproj.WebhookServerSpec
	if src != nil {
		dst = &argoproj.WebhookServerSpec{
			Host:    src.Host,
			Ingress: argoproj.ArgoCDIngressSpec(src.Ingress),
			Route:   argoproj.ArgoCDRouteSpec(src.Route),
=======
func ConvertAlphaToBetaWebhookServer(src *WebhookServerSpec) *v1beta1.WebhookServerSpec {
	var dst *v1beta1.WebhookServerSpec
	if src != nil {
		dst = &v1beta1.WebhookServerSpec{
			Host:    src.Host,
			Ingress: v1beta1.ArgoCDIngressSpec(src.Ingress),
			Route:   v1beta1.ArgoCDRouteSpec(src.Route),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaApplicationSet(src *ArgoCDApplicationSet) *argoproj.ArgoCDApplicationSet {
	var dst *argoproj.ArgoCDApplicationSet
	if src != nil {
		dst = &argoproj.ArgoCDApplicationSet{
=======
func ConvertAlphaToBetaApplicationSet(src *ArgoCDApplicationSet) *v1beta1.ArgoCDApplicationSet {
	var dst *v1beta1.ArgoCDApplicationSet
	if src != nil {
		dst = &v1beta1.ArgoCDApplicationSet{
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
			Image:            src.Image,
			Version:          src.Version,
			Resources:        src.Resources,
			LogLevel:         src.LogLevel,
			WebhookServer:    *ConvertAlphaToBetaWebhookServer(&src.WebhookServer),
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaGrafana(src *ArgoCDGrafanaSpec) *argoproj.ArgoCDGrafanaSpec {
	var dst *argoproj.ArgoCDGrafanaSpec
	if src != nil {
		dst = &argoproj.ArgoCDGrafanaSpec{
			Enabled: src.Enabled,
			Host:    src.Host,
			Image:   src.Image,
			Ingress: argoproj.ArgoCDIngressSpec(src.Ingress),
=======
func ConvertAlphaToBetaGrafana(src *ArgoCDGrafanaSpec) *v1beta1.ArgoCDGrafanaSpec {
	var dst *v1beta1.ArgoCDGrafanaSpec
	if src != nil {
		dst = &v1beta1.ArgoCDGrafanaSpec{
			Enabled: src.Enabled,
			Host:    src.Host,
			Image:   src.Image,
			Ingress: v1beta1.ArgoCDIngressSpec(src.Ingress),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaPrometheus(src *ArgoCDPrometheusSpec) *argoproj.ArgoCDPrometheusSpec {
	var dst *argoproj.ArgoCDPrometheusSpec
	if src != nil {
		dst = &argoproj.ArgoCDPrometheusSpec{
			Enabled: src.Enabled,
			Host:    src.Host,
			Ingress: argoproj.ArgoCDIngressSpec(src.Ingress),
			Route:   argoproj.ArgoCDRouteSpec(src.Route),
=======
func ConvertAlphaToBetaPrometheus(src *ArgoCDPrometheusSpec) *v1beta1.ArgoCDPrometheusSpec {
	var dst *v1beta1.ArgoCDPrometheusSpec
	if src != nil {
		dst = &v1beta1.ArgoCDPrometheusSpec{
			Enabled: src.Enabled,
			Host:    src.Host,
			Ingress: v1beta1.ArgoCDIngressSpec(src.Ingress),
			Route:   v1beta1.ArgoCDRouteSpec(src.Route),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			Size:    src.Size,
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaSSO(src *ArgoCDSSOSpec) *argoproj.ArgoCDSSOSpec {
	var dst *argoproj.ArgoCDSSOSpec
	if src != nil {
		dst = &argoproj.ArgoCDSSOSpec{
			Provider: argoproj.SSOProviderType(src.Provider),
			Dex:      ConvertAlphaToBetaDex(src.Dex),
			Keycloak: (*argoproj.ArgoCDKeycloakSpec)(src.Keycloak),
=======
func ConvertAlphaToBetaSSO(src *ArgoCDSSOSpec) *v1beta1.ArgoCDSSOSpec {
	var dst *v1beta1.ArgoCDSSOSpec
	if src != nil {
		dst = &v1beta1.ArgoCDSSOSpec{
			Provider: v1beta1.SSOProviderType(src.Provider),
			Dex:      ConvertAlphaToBetaDex(src.Dex),
			Keycloak: (*v1beta1.ArgoCDKeycloakSpec)(src.Keycloak),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaDex(src *ArgoCDDexSpec) *argoproj.ArgoCDDexSpec {
	var dst *argoproj.ArgoCDDexSpec
	if src != nil {
		dst = &argoproj.ArgoCDDexSpec{
=======
func ConvertAlphaToBetaDex(src *ArgoCDDexSpec) *v1beta1.ArgoCDDexSpec {
	var dst *v1beta1.ArgoCDDexSpec
	if src != nil {
		dst = &v1beta1.ArgoCDDexSpec{
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			Config:         src.Config,
			Groups:         src.Groups,
			Image:          src.Image,
			OpenShiftOAuth: src.OpenShiftOAuth,
			Resources:      src.Resources,
			Version:        src.Version,
			Env:            nil,
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaHA(src *ArgoCDHASpec) *argoproj.ArgoCDHASpec {
	var dst *argoproj.ArgoCDHASpec
	if src != nil {
		dst = &argoproj.ArgoCDHASpec{
=======
func ConvertAlphaToBetaHA(src *ArgoCDHASpec) *v1beta1.ArgoCDHASpec {
	var dst *v1beta1.ArgoCDHASpec
	if src != nil {
		dst = &v1beta1.ArgoCDHASpec{
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			Enabled:           src.Enabled,
			RedisProxyImage:   src.RedisProxyImage,
			RedisProxyVersion: src.RedisProxyVersion,
			Resources:         src.Resources,
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaTLS(src *ArgoCDTLSSpec) *argoproj.ArgoCDTLSSpec {
	var dst *argoproj.ArgoCDTLSSpec
	if src != nil {
		dst = &argoproj.ArgoCDTLSSpec{
			CA:           argoproj.ArgoCDCASpec(src.CA),
=======
func ConvertAlphaToBetaTLS(src *ArgoCDTLSSpec) *v1beta1.ArgoCDTLSSpec {
	var dst *v1beta1.ArgoCDTLSSpec
	if src != nil {
		dst = &v1beta1.ArgoCDTLSSpec{
			CA:           v1beta1.ArgoCDCASpec(src.CA),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			InitialCerts: src.InitialCerts,
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaServer(src *ArgoCDServerSpec) *argoproj.ArgoCDServerSpec {
	var dst *argoproj.ArgoCDServerSpec
	if src != nil {
		dst = &argoproj.ArgoCDServerSpec{
			Autoscale:        argoproj.ArgoCDServerAutoscaleSpec(src.Autoscale),
			GRPC:             *ConvertAlphaToBetaGRPC(&src.GRPC),
			Host:             src.Host,
			Ingress:          argoproj.ArgoCDIngressSpec(src.Ingress),
=======
func ConvertAlphaToBetaServer(src *ArgoCDServerSpec) *v1beta1.ArgoCDServerSpec {
	var dst *v1beta1.ArgoCDServerSpec
	if src != nil {
		dst = &v1beta1.ArgoCDServerSpec{
			Autoscale:        v1beta1.ArgoCDServerAutoscaleSpec(src.Autoscale),
			GRPC:             *ConvertAlphaToBetaGRPC(&src.GRPC),
			Host:             src.Host,
			Ingress:          v1beta1.ArgoCDIngressSpec(src.Ingress),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			Insecure:         src.Insecure,
			LogLevel:         src.LogLevel,
			LogFormat:        src.LogFormat,
			Replicas:         src.Replicas,
			Resources:        src.Resources,
<<<<<<< HEAD
			Route:            argoproj.ArgoCDRouteSpec(src.Route),
			Service:          argoproj.ArgoCDServerServiceSpec(src.Service),
=======
			Route:            v1beta1.ArgoCDRouteSpec(src.Route),
			Service:          v1beta1.ArgoCDServerServiceSpec(src.Service),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaGRPC(src *ArgoCDServerGRPCSpec) *argoproj.ArgoCDServerGRPCSpec {
	var dst *argoproj.ArgoCDServerGRPCSpec
	if src != nil {
		dst = &argoproj.ArgoCDServerGRPCSpec{
			Host:    src.Host,
			Ingress: argoproj.ArgoCDIngressSpec(src.Ingress),
=======
func ConvertAlphaToBetaGRPC(src *ArgoCDServerGRPCSpec) *v1beta1.ArgoCDServerGRPCSpec {
	var dst *v1beta1.ArgoCDServerGRPCSpec
	if src != nil {
		dst = &v1beta1.ArgoCDServerGRPCSpec{
			Host:    src.Host,
			Ingress: v1beta1.ArgoCDIngressSpec(src.Ingress),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaKustomizeVersions(src []KustomizeVersionSpec) []argoproj.KustomizeVersionSpec {
	var dst []argoproj.KustomizeVersionSpec
	for _, s := range src {
		dst = append(dst, argoproj.KustomizeVersionSpec{
=======
func ConvertAlphaToBetaKustomizeVersions(src []KustomizeVersionSpec) []v1beta1.KustomizeVersionSpec {
	var dst []v1beta1.KustomizeVersionSpec
	for _, s := range src {
		dst = append(dst, v1beta1.KustomizeVersionSpec{
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			Version: s.Version,
			Path:    s.Path,
		},
		)
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaResourceIgnoreDifferences(src *ResourceIgnoreDifference) *argoproj.ResourceIgnoreDifference {
	var dst *argoproj.ResourceIgnoreDifference
	if src != nil {
		dst = &argoproj.ResourceIgnoreDifference{
			All:                 (*argoproj.IgnoreDifferenceCustomization)(src.All),
=======
func ConvertAlphaToBetaResourceIgnoreDifferences(src *ResourceIgnoreDifference) *v1beta1.ResourceIgnoreDifference {
	var dst *v1beta1.ResourceIgnoreDifference
	if src != nil {
		dst = &v1beta1.ResourceIgnoreDifference{
			All:                 (*v1beta1.IgnoreDifferenceCustomization)(src.All),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			ResourceIdentifiers: ConvertAlphaToBetaResourceIdentifiers(src.ResourceIdentifiers),
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaResourceIdentifiers(src []ResourceIdentifiers) []argoproj.ResourceIdentifiers {
	var dst []argoproj.ResourceIdentifiers
	for _, s := range src {
		dst = append(dst, argoproj.ResourceIdentifiers{
			Group:         s.Group,
			Kind:          s.Kind,
			Customization: argoproj.IgnoreDifferenceCustomization(s.Customization),
=======
func ConvertAlphaToBetaResourceIdentifiers(src []ResourceIdentifiers) []v1beta1.ResourceIdentifiers {
	var dst []v1beta1.ResourceIdentifiers
	for _, s := range src {
		dst = append(dst, v1beta1.ResourceIdentifiers{
			Group:         s.Group,
			Kind:          s.Kind,
			Customization: v1beta1.IgnoreDifferenceCustomization(s.Customization),
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
		},
		)
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaResourceActions(src []ResourceAction) []argoproj.ResourceAction {
	var dst []argoproj.ResourceAction
	for _, s := range src {
		dst = append(dst, argoproj.ResourceAction{
=======
func ConvertAlphaToBetaResourceActions(src []ResourceAction) []v1beta1.ResourceAction {
	var dst []v1beta1.ResourceAction
	for _, s := range src {
		dst = append(dst, v1beta1.ResourceAction{
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
			Group:  s.Group,
			Kind:   s.Kind,
			Action: s.Action,
		},
		)
	}
	return dst
}

<<<<<<< HEAD
func ConvertAlphaToBetaResourceHealthChecks(src []ResourceHealthCheck) []argoproj.ResourceHealthCheck {
	var dst []argoproj.ResourceHealthCheck
	for _, s := range src {
		dst = append(dst, argoproj.ResourceHealthCheck{
=======
func ConvertAlphaToBetaResourceHealthChecks(src []ResourceHealthCheck) []v1beta1.ResourceHealthCheck {
	var dst []v1beta1.ResourceHealthCheck
	for _, s := range src {
		dst = append(dst, v1beta1.ResourceHealthCheck{
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaController(src *argoproj.ArgoCDApplicationControllerSpec) *ArgoCDApplicationControllerSpec {
=======
func ConvertBetaToAlphaController(src *v1beta1.ArgoCDApplicationControllerSpec) *ArgoCDApplicationControllerSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaWebhookServer(src *argoproj.WebhookServerSpec) *WebhookServerSpec {
=======
func ConvertBetaToAlphaWebhookServer(src *v1beta1.WebhookServerSpec) *WebhookServerSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaApplicationSet(src *argoproj.ArgoCDApplicationSet) *ArgoCDApplicationSet {
=======
func ConvertBetaToAlphaApplicationSet(src *v1beta1.ArgoCDApplicationSet) *ArgoCDApplicationSet {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
	var dst *ArgoCDApplicationSet
	if src != nil {
		dst = &ArgoCDApplicationSet{
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
			Image:            src.Image,
			Version:          src.Version,
			Resources:        src.Resources,
			LogLevel:         src.LogLevel,
			WebhookServer:    *ConvertBetaToAlphaWebhookServer(&src.WebhookServer),
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertBetaToAlphaGrafana(src *argoproj.ArgoCDGrafanaSpec) *ArgoCDGrafanaSpec {
=======
func ConvertBetaToAlphaGrafana(src *v1beta1.ArgoCDGrafanaSpec) *ArgoCDGrafanaSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaPrometheus(src *argoproj.ArgoCDPrometheusSpec) *ArgoCDPrometheusSpec {
=======
func ConvertBetaToAlphaPrometheus(src *v1beta1.ArgoCDPrometheusSpec) *ArgoCDPrometheusSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaSSO(src *argoproj.ArgoCDSSOSpec) *ArgoCDSSOSpec {
=======
func ConvertBetaToAlphaSSO(src *v1beta1.ArgoCDSSOSpec) *ArgoCDSSOSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
	var dst *ArgoCDSSOSpec
	if src != nil {
		dst = &ArgoCDSSOSpec{
			Provider: SSOProviderType(src.Provider),
			Dex:      ConvertBetaToAlphaDex(src.Dex),
			Keycloak: (*ArgoCDKeycloakSpec)(src.Keycloak),
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertBetaToAlphaDex(src *argoproj.ArgoCDDexSpec) *ArgoCDDexSpec {
=======
func ConvertBetaToAlphaDex(src *v1beta1.ArgoCDDexSpec) *ArgoCDDexSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
	var dst *ArgoCDDexSpec
	if src != nil {
		dst = &ArgoCDDexSpec{
			Config:         src.Config,
			Groups:         src.Groups,
			Image:          src.Image,
			OpenShiftOAuth: src.OpenShiftOAuth,
			Resources:      src.Resources,
			Version:        src.Version,
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertBetaToAlphaHA(src *argoproj.ArgoCDHASpec) *ArgoCDHASpec {
=======
func ConvertBetaToAlphaHA(src *v1beta1.ArgoCDHASpec) *ArgoCDHASpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaTLS(src *argoproj.ArgoCDTLSSpec) *ArgoCDTLSSpec {
=======
func ConvertBetaToAlphaTLS(src *v1beta1.ArgoCDTLSSpec) *ArgoCDTLSSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaServer(src *argoproj.ArgoCDServerSpec) *ArgoCDServerSpec {
=======
func ConvertBetaToAlphaServer(src *v1beta1.ArgoCDServerSpec) *ArgoCDServerSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
	var dst *ArgoCDServerSpec
	if src != nil {
		dst = &ArgoCDServerSpec{
			Autoscale:        ArgoCDServerAutoscaleSpec(src.Autoscale),
			GRPC:             *ConvertBetaToAlphaGRPC(&src.GRPC),
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
func ConvertBetaToAlphaGRPC(src *argoproj.ArgoCDServerGRPCSpec) *ArgoCDServerGRPCSpec {
=======
func ConvertBetaToAlphaGRPC(src *v1beta1.ArgoCDServerGRPCSpec) *ArgoCDServerGRPCSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaKustomizeVersions(src []argoproj.KustomizeVersionSpec) []KustomizeVersionSpec {
=======
func ConvertBetaToAlphaKustomizeVersions(src []v1beta1.KustomizeVersionSpec) []KustomizeVersionSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaResourceIgnoreDifferences(src *argoproj.ResourceIgnoreDifference) *ResourceIgnoreDifference {
=======
func ConvertBetaToAlphaResourceIgnoreDifferences(src *v1beta1.ResourceIgnoreDifference) *ResourceIgnoreDifference {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
	var dst *ResourceIgnoreDifference
	if src != nil {
		dst = &ResourceIgnoreDifference{
			All:                 (*IgnoreDifferenceCustomization)(src.All),
			ResourceIdentifiers: ConvertBetaToAlphaResourceIdentifiers(src.ResourceIdentifiers),
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertBetaToAlphaResourceIdentifiers(src []argoproj.ResourceIdentifiers) []ResourceIdentifiers {
=======
func ConvertBetaToAlphaResourceIdentifiers(src []v1beta1.ResourceIdentifiers) []ResourceIdentifiers {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaResourceActions(src []argoproj.ResourceAction) []ResourceAction {
=======
func ConvertBetaToAlphaResourceActions(src []v1beta1.ResourceAction) []ResourceAction {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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
func ConvertBetaToAlphaResourceHealthChecks(src []argoproj.ResourceHealthCheck) []ResourceHealthCheck {
=======
func ConvertBetaToAlphaResourceHealthChecks(src []v1beta1.ResourceHealthCheck) []ResourceHealthCheck {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
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

<<<<<<< HEAD
func ConvertBetaToAlphaRedis(src *argoproj.ArgoCDRedisSpec) *ArgoCDRedisSpec {
=======
func ConvertBetaToAlphaRedis(src *v1beta1.ArgoCDRedisSpec) *ArgoCDRedisSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
	var dst *ArgoCDRedisSpec
	if src != nil {
		dst = &ArgoCDRedisSpec{
			AutoTLS:                src.AutoTLS,
			DisableTLSVerification: src.DisableTLSVerification,
			Image:                  src.Image,
			Resources:              src.Resources,
			Version:                src.Version,
		}
	}
	return dst
}

<<<<<<< HEAD
func ConvertBetaToAlphaRepo(src *argoproj.ArgoCDRepoSpec) *ArgoCDRepoSpec {
=======
func ConvertBetaToAlphaRepo(src *v1beta1.ArgoCDRepoSpec) *ArgoCDRepoSpec {
>>>>>>> d424ebd71f4d1e67ade00a8b329e3a6e8688950d
	var dst *ArgoCDRepoSpec
	if src != nil {
		dst = &ArgoCDRepoSpec{
			AutoTLS:              src.AutoTLS,
			Env:                  src.Env,
			ExecTimeout:          src.ExecTimeout,
			ExtraRepoCommandArgs: src.ExtraRepoCommandArgs,
			Image:                src.Image,
			InitContainers:       src.InitContainers,
			LogFormat:            src.LogFormat,
			LogLevel:             src.LogLevel,
			MountSAToken:         src.MountSAToken,
			Replicas:             src.Replicas,
			Resources:            src.Resources,
			ServiceAccount:       src.ServiceAccount,
			SidecarContainers:    src.SidecarContainers,
			VerifyTLS:            src.VerifyTLS,
			Version:              src.Version,
			VolumeMounts:         src.VolumeMounts,
			Volumes:              src.Volumes,
		}
	}
	return dst
}
