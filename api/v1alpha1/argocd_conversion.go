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
	conversionLogger.WithValues("instance", src.Name, "instance-namespace", src.Namespace).V(1).Info("processing v1alpha1 to v1beta1 conversion")
	dst := dstRaw.(*v1beta1.ArgoCD)

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
	dst.Spec.Import = (*v1beta1.ArgoCDImportSpec)(src.Spec.Import)
	dst.Spec.InitialRepositories = src.Spec.InitialRepositories
	dst.Spec.InitialSSHKnownHosts = v1beta1.SSHHostsSpec(src.Spec.InitialSSHKnownHosts)
	dst.Spec.KustomizeBuildOptions = src.Spec.KustomizeBuildOptions
	dst.Spec.KustomizeVersions = convertAlphaToBetaKustomizeVersions(src.Spec.KustomizeVersions)
	dst.Spec.OIDCConfig = src.Spec.OIDCConfig
	dst.Spec.Monitoring = v1beta1.ArgoCDMonitoringSpec(src.Spec.Monitoring)
	dst.Spec.NodePlacement = (*v1beta1.ArgoCDNodePlacementSpec)(src.Spec.NodePlacement)
	dst.Spec.Notifications = v1beta1.ArgoCDNotifications(src.Spec.Notifications)
	dst.Spec.Prometheus = *convertAlphaToBetaPrometheus(&src.Spec.Prometheus)
	dst.Spec.RBAC = v1beta1.ArgoCDRBACSpec(src.Spec.RBAC)
	dst.Spec.Redis = v1beta1.ArgoCDRedisSpec(src.Spec.Redis)
	dst.Spec.Repo = v1beta1.ArgoCDRepoSpec(src.Spec.Repo)
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
	dst.Spec.Banner = (*v1beta1.Banner)(src.Spec.Banner)

	// Status conversion
	dst.Status = v1beta1.ArgoCDStatus(src.Status)

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this (v1alpha1) version.
func (dst *ArgoCD) ConvertFrom(srcRaw conversion.Hub) error {
	conversionLogger.WithValues("instance", dst.Name, "instance-namespace", dst.Namespace).V(1).Info("processing v1beta1 to v1alpha1 conversion")

	src := srcRaw.(*v1beta1.ArgoCD)

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
func convertAlphaToBetaController(src *ArgoCDApplicationControllerSpec) *v1beta1.ArgoCDApplicationControllerSpec {
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

func convertAlphaToBetaWebhookServer(src *WebhookServerSpec) *v1beta1.WebhookServerSpec {
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

func convertAlphaToBetaApplicationSet(src *ArgoCDApplicationSet) *v1beta1.ArgoCDApplicationSet {
	var dst *v1beta1.ArgoCDApplicationSet
	if src != nil {
		dst = &v1beta1.ArgoCDApplicationSet{
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

func convertAlphaToBetaGrafana(src *ArgoCDGrafanaSpec) *v1beta1.ArgoCDGrafanaSpec {
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

func convertAlphaToBetaPrometheus(src *ArgoCDPrometheusSpec) *v1beta1.ArgoCDPrometheusSpec {
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

func convertAlphaToBetaSSO(src *ArgoCDSSOSpec) *v1beta1.ArgoCDSSOSpec {
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

func convertAlphaToBetaHA(src *ArgoCDHASpec) *v1beta1.ArgoCDHASpec {
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

func convertAlphaToBetaTLS(src *ArgoCDTLSSpec) *v1beta1.ArgoCDTLSSpec {
	var dst *v1beta1.ArgoCDTLSSpec
	if src != nil {
		dst = &v1beta1.ArgoCDTLSSpec{
			CA:           v1beta1.ArgoCDCASpec(src.CA),
			InitialCerts: src.InitialCerts,
		}
	}
	return dst
}

func convertAlphaToBetaServer(src *ArgoCDServerSpec) *v1beta1.ArgoCDServerSpec {
	var dst *v1beta1.ArgoCDServerSpec
	if src != nil {
		dst = &v1beta1.ArgoCDServerSpec{
			Autoscale:        v1beta1.ArgoCDServerAutoscaleSpec(src.Autoscale),
			GRPC:             *convertAlphaToBetaGRPC(&src.GRPC),
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

func convertAlphaToBetaGRPC(src *ArgoCDServerGRPCSpec) *v1beta1.ArgoCDServerGRPCSpec {
	var dst *v1beta1.ArgoCDServerGRPCSpec
	if src != nil {
		dst = &v1beta1.ArgoCDServerGRPCSpec{
			Host:    src.Host,
			Ingress: v1beta1.ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

func convertAlphaToBetaKustomizeVersions(src []KustomizeVersionSpec) []v1beta1.KustomizeVersionSpec {
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

func convertAlphaToBetaResourceIgnoreDifferences(src *ResourceIgnoreDifference) *v1beta1.ResourceIgnoreDifference {
	var dst *v1beta1.ResourceIgnoreDifference
	if src != nil {
		dst = &v1beta1.ResourceIgnoreDifference{
			All:                 (*v1beta1.IgnoreDifferenceCustomization)(src.All),
			ResourceIdentifiers: convertAlphaToBetaResourceIdentifiers(src.ResourceIdentifiers),
		}
	}
	return dst
}

func convertAlphaToBetaResourceIdentifiers(src []ResourceIdentifiers) []v1beta1.ResourceIdentifiers {
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

func convertAlphaToBetaResourceActions(src []ResourceAction) []v1beta1.ResourceAction {
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

func convertAlphaToBetaResourceHealthChecks(src []ResourceHealthCheck) []v1beta1.ResourceHealthCheck {
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
func convertBetaToAlphaController(src *v1beta1.ArgoCDApplicationControllerSpec) *ArgoCDApplicationControllerSpec {
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

func convertBetaToAlphaWebhookServer(src *v1beta1.WebhookServerSpec) *WebhookServerSpec {
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

func convertBetaToAlphaApplicationSet(src *v1beta1.ArgoCDApplicationSet) *ArgoCDApplicationSet {
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

func convertBetaToAlphaGrafana(src *v1beta1.ArgoCDGrafanaSpec) *ArgoCDGrafanaSpec {
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

func convertBetaToAlphaPrometheus(src *v1beta1.ArgoCDPrometheusSpec) *ArgoCDPrometheusSpec {
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

func convertBetaToAlphaSSO(src *v1beta1.ArgoCDSSOSpec) *ArgoCDSSOSpec {
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

func convertBetaToAlphaHA(src *v1beta1.ArgoCDHASpec) *ArgoCDHASpec {
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

func convertBetaToAlphaTLS(src *v1beta1.ArgoCDTLSSpec) *ArgoCDTLSSpec {
	var dst *ArgoCDTLSSpec
	if src != nil {
		dst = &ArgoCDTLSSpec{
			CA:           ArgoCDCASpec(src.CA),
			InitialCerts: src.InitialCerts,
		}
	}
	return dst
}

func convertBetaToAlphaServer(src *v1beta1.ArgoCDServerSpec) *ArgoCDServerSpec {
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

func convertBetaToAlphaGRPC(src *v1beta1.ArgoCDServerGRPCSpec) *ArgoCDServerGRPCSpec {
	var dst *ArgoCDServerGRPCSpec
	if src != nil {
		dst = &ArgoCDServerGRPCSpec{
			Host:    src.Host,
			Ingress: ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

func convertBetaToAlphaKustomizeVersions(src []v1beta1.KustomizeVersionSpec) []KustomizeVersionSpec {
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

func convertBetaToAlphaResourceIgnoreDifferences(src *v1beta1.ResourceIgnoreDifference) *ResourceIgnoreDifference {
	var dst *ResourceIgnoreDifference
	if src != nil {
		dst = &ResourceIgnoreDifference{
			All:                 (*IgnoreDifferenceCustomization)(src.All),
			ResourceIdentifiers: convertBetaToAlphaResourceIdentifiers(src.ResourceIdentifiers),
		}
	}
	return dst
}

func convertBetaToAlphaResourceIdentifiers(src []v1beta1.ResourceIdentifiers) []ResourceIdentifiers {
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

func convertBetaToAlphaResourceActions(src []v1beta1.ResourceAction) []ResourceAction {
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

func convertBetaToAlphaResourceHealthChecks(src []v1beta1.ResourceHealthCheck) []ResourceHealthCheck {
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
