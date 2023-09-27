package v1alpha1

import (
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

var conversionLogger = ctrl.Log.WithName("conversion-webhook")

// ConvertTo converts this (v1alpha1) ArgoCD to the Hub version (v1beta1).
func (src *ArgoCD) ConvertTo(dstRaw conversion.Hub) error {
	conversionLogger.WithValues("instance", src.Name, "instance-namespace", src.Namespace).V(1).Info("processing v1alpha1 to v1beta1 conversion")
	dst := dstRaw.(*argoproj.ArgoCD)

	// ObjectMeta conversion
	dst.ObjectMeta = src.ObjectMeta

	// Spec conversion

	// sso field
	sso := convertAlphaToBetaSSO(src.Spec.SSO)

	// in case of conflict, deprecated fields will have more priority during conversion to beta
	// deprecated keycloak configs set in alpha (.spec.sso.image, .spec.sso.version, .spec.sso.verifyTLS, .spec.sso.resources),
	// override .spec.sso.keycloak in beta
	if src.Spec.SSO != nil && !reflect.DeepEqual(src.Spec.SSO, &ArgoCDSSOSpec{}) {
		if src.Spec.SSO.Image != "" || src.Spec.SSO.Version != "" || src.Spec.SSO.VerifyTLS != nil || src.Spec.SSO.Resources != nil {
			if sso.Keycloak == nil {
				sso.Keycloak = &argoproj.ArgoCDKeycloakSpec{}
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
			sso = &argoproj.ArgoCDSSOSpec{}
		}
		sso.Provider = argoproj.SSOProviderTypeDex
		sso.Dex = (*argoproj.ArgoCDDexSpec)(src.Spec.Dex)
	}

	dst.Spec.SSO = sso

	// rest of the fields
	dst.Spec.ApplicationSet = convertAlphaToBetaApplicationSet(src.Spec.ApplicationSet)
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.ApplicationInstanceLabelKey = src.Spec.ApplicationInstanceLabelKey
	dst.Spec.ConfigManagementPlugins = src.Spec.ConfigManagementPlugins
	dst.Spec.Controller = *convertAlphaToBetaController(&src.Spec.Controller)
	dst.Spec.DisableAdmin = src.Spec.DisableAdmin
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.GATrackingID = src.Spec.GATrackingID
	dst.Spec.GAAnonymizeUsers = src.Spec.GAAnonymizeUsers
	dst.Spec.Grafana = *convertAlphaToBetaGrafana(&src.Spec.Grafana)
	dst.Spec.HA = *convertAlphaToBetaHA(&src.Spec.HA)
	dst.Spec.HelpChatURL = src.Spec.HelpChatURL
	dst.Spec.HelpChatText = src.Spec.HelpChatText
	dst.Spec.Image = src.Spec.Image
	dst.Spec.Import = (*argoproj.ArgoCDImportSpec)(src.Spec.Import)
	dst.Spec.InitialRepositories = src.Spec.InitialRepositories
	dst.Spec.InitialSSHKnownHosts = argoproj.SSHHostsSpec(src.Spec.InitialSSHKnownHosts)
	dst.Spec.KustomizeBuildOptions = src.Spec.KustomizeBuildOptions
	dst.Spec.KustomizeVersions = convertAlphaToBetaKustomizeVersions(src.Spec.KustomizeVersions)
	dst.Spec.OIDCConfig = src.Spec.OIDCConfig
	dst.Spec.Monitoring = argoproj.ArgoCDMonitoringSpec(src.Spec.Monitoring)
	dst.Spec.NodePlacement = (*argoproj.ArgoCDNodePlacementSpec)(src.Spec.NodePlacement)
	dst.Spec.Notifications = argoproj.ArgoCDNotifications(src.Spec.Notifications)
	dst.Spec.Prometheus = *convertAlphaToBetaPrometheus(&src.Spec.Prometheus)
	dst.Spec.RBAC = argoproj.ArgoCDRBACSpec(src.Spec.RBAC)
	dst.Spec.Redis = argoproj.ArgoCDRedisSpec(src.Spec.Redis)
	dst.Spec.Repo = argoproj.ArgoCDRepoSpec(src.Spec.Repo)
	dst.Spec.RepositoryCredentials = src.Spec.RepositoryCredentials
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
	dst.Spec.UsersAnonymousEnabled = src.Spec.UsersAnonymousEnabled
	dst.Spec.Version = src.Spec.Version
	dst.Spec.Banner = (*argoproj.Banner)(src.Spec.Banner)

	// Status conversion
	dst.Status = argoproj.ArgoCDStatus(src.Status)

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this (v1alpha1) version.
func (dst *ArgoCD) ConvertFrom(srcRaw conversion.Hub) error {
	conversionLogger.WithValues("instance", dst.Name, "instance-namespace", dst.Namespace).V(1).Info("processing v1beta1 to v1alpha1 conversion")

	src := srcRaw.(*argoproj.ArgoCD)

	// ObjectMeta conversion
	dst.ObjectMeta = src.ObjectMeta

	// Spec conversion

	// sso field
	// ignoring conversions of sso fields from v1beta1 to deprecated v1alpha1 as
	// there is no data loss since the new fields in v1beta1 are also present in v1alpha1 &
	// v1alpha1 is not used in business logic & only exists for presentation
	sso := convertBetaToAlphaSSO(src.Spec.SSO)
	dst.Spec.SSO = sso

	// rest of the fields
	dst.Spec.ApplicationSet = convertBetaToAlphaApplicationSet(src.Spec.ApplicationSet)
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.ApplicationInstanceLabelKey = src.Spec.ApplicationInstanceLabelKey
	dst.Spec.ConfigManagementPlugins = src.Spec.ConfigManagementPlugins
	dst.Spec.Controller = *convertBetaToAlphaController(&src.Spec.Controller)
	dst.Spec.DisableAdmin = src.Spec.DisableAdmin
	dst.Spec.ExtraConfig = src.Spec.ExtraConfig
	dst.Spec.GATrackingID = src.Spec.GATrackingID
	dst.Spec.GAAnonymizeUsers = src.Spec.GAAnonymizeUsers
	dst.Spec.Grafana = *convertBetaToAlphaGrafana(&src.Spec.Grafana)
	dst.Spec.HA = *convertBetaToAlphaHA(&src.Spec.HA)
	dst.Spec.HelpChatURL = src.Spec.HelpChatURL
	dst.Spec.HelpChatText = src.Spec.HelpChatText
	dst.Spec.Image = src.Spec.Image
	dst.Spec.Import = (*ArgoCDImportSpec)(src.Spec.Import)
	dst.Spec.InitialRepositories = src.Spec.InitialRepositories
	dst.Spec.InitialSSHKnownHosts = SSHHostsSpec(src.Spec.InitialSSHKnownHosts)
	dst.Spec.KustomizeBuildOptions = src.Spec.KustomizeBuildOptions
	dst.Spec.KustomizeVersions = convertBetaToAlphaKustomizeVersions(src.Spec.KustomizeVersions)
	dst.Spec.OIDCConfig = src.Spec.OIDCConfig
	dst.Spec.Monitoring = ArgoCDMonitoringSpec(src.Spec.Monitoring)
	dst.Spec.NodePlacement = (*ArgoCDNodePlacementSpec)(src.Spec.NodePlacement)
	dst.Spec.Notifications = ArgoCDNotifications(src.Spec.Notifications)
	dst.Spec.Prometheus = *convertBetaToAlphaPrometheus(&src.Spec.Prometheus)
	dst.Spec.RBAC = ArgoCDRBACSpec(src.Spec.RBAC)
	dst.Spec.Redis = ArgoCDRedisSpec(src.Spec.Redis)
	dst.Spec.Repo = ArgoCDRepoSpec(src.Spec.Repo)
	dst.Spec.RepositoryCredentials = src.Spec.RepositoryCredentials
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
	dst.Spec.UsersAnonymousEnabled = src.Spec.UsersAnonymousEnabled
	dst.Spec.Version = src.Spec.Version
	dst.Spec.Banner = (*Banner)(src.Spec.Banner)

	// Status conversion
	dst.Status = ArgoCDStatus(src.Status)

	return nil
}

// Conversion funcs for v1alpha1 to v1beta1.
func convertAlphaToBetaController(src *ArgoCDApplicationControllerSpec) *argoproj.ArgoCDApplicationControllerSpec {
	var dst *argoproj.ArgoCDApplicationControllerSpec
	if src != nil {
		dst = &argoproj.ArgoCDApplicationControllerSpec{
			Processors:       argoproj.ArgoCDApplicationControllerProcessorsSpec(src.Processors),
			LogLevel:         src.LogLevel,
			LogFormat:        src.LogFormat,
			Resources:        src.Resources,
			ParallelismLimit: src.ParallelismLimit,
			AppSync:          src.AppSync,
			Sharding:         argoproj.ArgoCDApplicationControllerShardSpec(src.Sharding),
			Env:              src.Env,
		}
	}
	return dst
}

func convertAlphaToBetaWebhookServer(src *WebhookServerSpec) *argoproj.WebhookServerSpec {
	var dst *argoproj.WebhookServerSpec
	if src != nil {
		dst = &argoproj.WebhookServerSpec{
			Host:    src.Host,
			Ingress: argoproj.ArgoCDIngressSpec(src.Ingress),
			Route:   argoproj.ArgoCDRouteSpec(src.Route),
		}
	}
	return dst
}

func convertAlphaToBetaApplicationSet(src *ArgoCDApplicationSet) *argoproj.ArgoCDApplicationSet {
	var dst *argoproj.ArgoCDApplicationSet
	if src != nil {
		dst = &argoproj.ArgoCDApplicationSet{
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
			Image:            src.Image,
			Version:          src.Version,
			Resources:        src.Resources,
			LogLevel:         src.LogLevel,
			WebhookServer:    *convertAlphaToBetaWebhookServer(&src.WebhookServer),
		}
	}
	return dst
}

func convertAlphaToBetaGrafana(src *ArgoCDGrafanaSpec) *argoproj.ArgoCDGrafanaSpec {
	var dst *argoproj.ArgoCDGrafanaSpec
	if src != nil {
		dst = &argoproj.ArgoCDGrafanaSpec{
			Enabled: src.Enabled,
			Host:    src.Host,
			Image:   src.Image,
			Ingress: argoproj.ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

func convertAlphaToBetaPrometheus(src *ArgoCDPrometheusSpec) *argoproj.ArgoCDPrometheusSpec {
	var dst *argoproj.ArgoCDPrometheusSpec
	if src != nil {
		dst = &argoproj.ArgoCDPrometheusSpec{
			Enabled: src.Enabled,
			Host:    src.Host,
			Ingress: argoproj.ArgoCDIngressSpec(src.Ingress),
			Route:   argoproj.ArgoCDRouteSpec(src.Route),
			Size:    src.Size,
		}
	}
	return dst
}

func convertAlphaToBetaSSO(src *ArgoCDSSOSpec) *argoproj.ArgoCDSSOSpec {
	var dst *argoproj.ArgoCDSSOSpec
	if src != nil {
		dst = &argoproj.ArgoCDSSOSpec{
			Provider: argoproj.SSOProviderType(src.Provider),
			Dex:      (*argoproj.ArgoCDDexSpec)(src.Dex),
			Keycloak: (*argoproj.ArgoCDKeycloakSpec)(src.Keycloak),
		}
	}
	return dst
}

func convertAlphaToBetaHA(src *ArgoCDHASpec) *argoproj.ArgoCDHASpec {
	var dst *argoproj.ArgoCDHASpec
	if src != nil {
		dst = &argoproj.ArgoCDHASpec{
			Enabled:           src.Enabled,
			RedisProxyImage:   src.RedisProxyImage,
			RedisProxyVersion: src.RedisProxyVersion,
			Resources:         src.Resources,
		}
	}
	return dst
}

func convertAlphaToBetaTLS(src *ArgoCDTLSSpec) *argoproj.ArgoCDTLSSpec {
	var dst *argoproj.ArgoCDTLSSpec
	if src != nil {
		dst = &argoproj.ArgoCDTLSSpec{
			CA:           argoproj.ArgoCDCASpec(src.CA),
			InitialCerts: src.InitialCerts,
		}
	}
	return dst
}

func convertAlphaToBetaServer(src *ArgoCDServerSpec) *argoproj.ArgoCDServerSpec {
	var dst *argoproj.ArgoCDServerSpec
	if src != nil {
		dst = &argoproj.ArgoCDServerSpec{
			Autoscale:        argoproj.ArgoCDServerAutoscaleSpec(src.Autoscale),
			GRPC:             *convertAlphaToBetaGRPC(&src.GRPC),
			Host:             src.Host,
			Ingress:          argoproj.ArgoCDIngressSpec(src.Ingress),
			Insecure:         src.Insecure,
			LogLevel:         src.LogLevel,
			LogFormat:        src.LogFormat,
			Replicas:         src.Replicas,
			Resources:        src.Resources,
			Route:            argoproj.ArgoCDRouteSpec(src.Route),
			Service:          argoproj.ArgoCDServerServiceSpec(src.Service),
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
		}
	}
	return dst
}

func convertAlphaToBetaGRPC(src *ArgoCDServerGRPCSpec) *argoproj.ArgoCDServerGRPCSpec {
	var dst *argoproj.ArgoCDServerGRPCSpec
	if src != nil {
		dst = &argoproj.ArgoCDServerGRPCSpec{
			Host:    src.Host,
			Ingress: argoproj.ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

func convertAlphaToBetaKustomizeVersions(src []KustomizeVersionSpec) []argoproj.KustomizeVersionSpec {
	var dst []argoproj.KustomizeVersionSpec
	for _, s := range src {
		dst = append(dst, argoproj.KustomizeVersionSpec{
			Version: s.Version,
			Path:    s.Path,
		},
		)
	}
	return dst
}

func convertAlphaToBetaResourceIgnoreDifferences(src *ResourceIgnoreDifference) *argoproj.ResourceIgnoreDifference {
	var dst *argoproj.ResourceIgnoreDifference
	if src != nil {
		dst = &argoproj.ResourceIgnoreDifference{
			All:                 (*argoproj.IgnoreDifferenceCustomization)(src.All),
			ResourceIdentifiers: convertAlphaToBetaResourceIdentifiers(src.ResourceIdentifiers),
		}
	}
	return dst
}

func convertAlphaToBetaResourceIdentifiers(src []ResourceIdentifiers) []argoproj.ResourceIdentifiers {
	var dst []argoproj.ResourceIdentifiers
	for _, s := range src {
		dst = append(dst, argoproj.ResourceIdentifiers{
			Group:         s.Group,
			Kind:          s.Kind,
			Customization: argoproj.IgnoreDifferenceCustomization(s.Customization),
		},
		)
	}
	return dst
}

func convertAlphaToBetaResourceActions(src []ResourceAction) []argoproj.ResourceAction {
	var dst []argoproj.ResourceAction
	for _, s := range src {
		dst = append(dst, argoproj.ResourceAction{
			Group:  s.Group,
			Kind:   s.Kind,
			Action: s.Action,
		},
		)
	}
	return dst
}

func convertAlphaToBetaResourceHealthChecks(src []ResourceHealthCheck) []argoproj.ResourceHealthCheck {
	var dst []argoproj.ResourceHealthCheck
	for _, s := range src {
		dst = append(dst, argoproj.ResourceHealthCheck{
			Group: s.Group,
			Kind:  s.Kind,
			Check: s.Check,
		},
		)
	}
	return dst
}

// Conversion funcs for v1beta1 to v1alpha1.
func convertBetaToAlphaController(src *argoproj.ArgoCDApplicationControllerSpec) *ArgoCDApplicationControllerSpec {
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

func convertBetaToAlphaWebhookServer(src *argoproj.WebhookServerSpec) *WebhookServerSpec {
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

func convertBetaToAlphaApplicationSet(src *argoproj.ArgoCDApplicationSet) *ArgoCDApplicationSet {
	var dst *ArgoCDApplicationSet
	if src != nil {
		dst = &ArgoCDApplicationSet{
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
			Image:            src.Image,
			Version:          src.Version,
			Resources:        src.Resources,
			LogLevel:         src.LogLevel,
			WebhookServer:    *convertBetaToAlphaWebhookServer(&src.WebhookServer),
		}
	}
	return dst
}

func convertBetaToAlphaGrafana(src *argoproj.ArgoCDGrafanaSpec) *ArgoCDGrafanaSpec {
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

func convertBetaToAlphaPrometheus(src *argoproj.ArgoCDPrometheusSpec) *ArgoCDPrometheusSpec {
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

func convertBetaToAlphaSSO(src *argoproj.ArgoCDSSOSpec) *ArgoCDSSOSpec {
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

func convertBetaToAlphaHA(src *argoproj.ArgoCDHASpec) *ArgoCDHASpec {
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

func convertBetaToAlphaTLS(src *argoproj.ArgoCDTLSSpec) *ArgoCDTLSSpec {
	var dst *ArgoCDTLSSpec
	if src != nil {
		dst = &ArgoCDTLSSpec{
			CA:           ArgoCDCASpec(src.CA),
			InitialCerts: src.InitialCerts,
		}
	}
	return dst
}

func convertBetaToAlphaServer(src *argoproj.ArgoCDServerSpec) *ArgoCDServerSpec {
	var dst *ArgoCDServerSpec
	if src != nil {
		dst = &ArgoCDServerSpec{
			Autoscale:        ArgoCDServerAutoscaleSpec(src.Autoscale),
			GRPC:             *convertBetaToAlphaGRPC(&src.GRPC),
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

func convertBetaToAlphaGRPC(src *argoproj.ArgoCDServerGRPCSpec) *ArgoCDServerGRPCSpec {
	var dst *ArgoCDServerGRPCSpec
	if src != nil {
		dst = &ArgoCDServerGRPCSpec{
			Host:    src.Host,
			Ingress: ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

func convertBetaToAlphaKustomizeVersions(src []argoproj.KustomizeVersionSpec) []KustomizeVersionSpec {
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

func convertBetaToAlphaResourceIgnoreDifferences(src *argoproj.ResourceIgnoreDifference) *ResourceIgnoreDifference {
	var dst *ResourceIgnoreDifference
	if src != nil {
		dst = &ResourceIgnoreDifference{
			All:                 (*IgnoreDifferenceCustomization)(src.All),
			ResourceIdentifiers: convertBetaToAlphaResourceIdentifiers(src.ResourceIdentifiers),
		}
	}
	return dst
}

func convertBetaToAlphaResourceIdentifiers(src []argoproj.ResourceIdentifiers) []ResourceIdentifiers {
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

func convertBetaToAlphaResourceActions(src []argoproj.ResourceAction) []ResourceAction {
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

func convertBetaToAlphaResourceHealthChecks(src []argoproj.ResourceHealthCheck) []ResourceHealthCheck {
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
