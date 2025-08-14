package v1alpha1

import (
	"reflect"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	"github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

var conversionLogger = ctrl.Log.WithName("conversion-webhook")

// ConvertTo converts this (v1alpha1) ArgoCD to the Hub version (v1beta1).
func (argocd *ArgoCD) ConvertTo(dstRaw conversion.Hub) error {
	conversionLogger.V(1).Info("v1alpha1 to v1beta1 conversion requested.")
	dst := dstRaw.(*v1beta1.ArgoCD)

	// ObjectMeta conversion
	dst.ObjectMeta = argocd.ObjectMeta

	// Spec conversion

	// sso field
	sso := ConvertAlphaToBetaSSO(argocd.Spec.SSO)

	// in case of conflict, deprecated fields will have more priority during conversion to beta
	// deprecated keycloak configs set in alpha (.spec.sso.image, .spec.sso.version, .spec.sso.verifyTLS, .spec.sso.resources),
	// override .spec.sso.keycloak in beta
	if argocd.Spec.SSO != nil && !reflect.DeepEqual(argocd.Spec.SSO, &ArgoCDSSOSpec{}) {
		if argocd.Spec.SSO.Image != "" || argocd.Spec.SSO.Version != "" || argocd.Spec.SSO.VerifyTLS != nil || argocd.Spec.SSO.Resources != nil {
			if sso.Keycloak == nil {
				sso.Keycloak = &v1beta1.ArgoCDKeycloakSpec{}
			}
			sso.Keycloak.Image = argocd.Spec.SSO.Image
			sso.Keycloak.Version = argocd.Spec.SSO.Version
			sso.Keycloak.VerifyTLS = argocd.Spec.SSO.VerifyTLS
			sso.Keycloak.Resources = argocd.Spec.SSO.Resources

		}
	}

	// deprecated dex configs set in alpha (.spec.dex), override .spec.sso.dex in beta
	if argocd.Spec.Dex != nil && !reflect.DeepEqual(argocd.Spec.Dex, &ArgoCDDexSpec{}) && (argocd.Spec.Dex.Config != "" || argocd.Spec.Dex.OpenShiftOAuth) {
		if sso == nil {
			sso = &v1beta1.ArgoCDSSOSpec{}
		}
		sso.Provider = v1beta1.SSOProviderTypeDex
		sso.Dex = ConvertAlphaToBetaDex(argocd.Spec.Dex)
	}

	dst.Spec.SSO = sso

	// rest of the fields
	dst.Spec.ApplicationSet = ConvertAlphaToBetaApplicationSet(argocd.Spec.ApplicationSet)
	dst.Spec.ExtraConfig = argocd.Spec.ExtraConfig
	dst.Spec.ApplicationInstanceLabelKey = argocd.Spec.ApplicationInstanceLabelKey
	//lint:ignore SA1019 known to be deprecated
	dst.Spec.ConfigManagementPlugins = argocd.Spec.ConfigManagementPlugins //nolint:staticcheck // SA1019: We must test deprecated fields.
	dst.Spec.Controller = *ConvertAlphaToBetaController(&argocd.Spec.Controller)
	dst.Spec.DisableAdmin = argocd.Spec.DisableAdmin
	dst.Spec.ExtraConfig = argocd.Spec.ExtraConfig
	dst.Spec.GATrackingID = argocd.Spec.GATrackingID
	dst.Spec.GAAnonymizeUsers = argocd.Spec.GAAnonymizeUsers
	//lint:ignore SA1019 known to be deprecated
	dst.Spec.Grafana = *ConvertAlphaToBetaGrafana(&argocd.Spec.Grafana) //nolint:staticcheck // SA1019: We must test deprecated fields.
	dst.Spec.HA = *ConvertAlphaToBetaHA(&argocd.Spec.HA)
	dst.Spec.HelpChatURL = argocd.Spec.HelpChatURL
	dst.Spec.HelpChatText = argocd.Spec.HelpChatText
	dst.Spec.Image = argocd.Spec.Image
	dst.Spec.Import = (*v1beta1.ArgoCDImportSpec)(argocd.Spec.Import)
	//lint:ignore SA1019 known to be deprecated
	dst.Spec.InitialRepositories = argocd.Spec.InitialRepositories //nolint:staticcheck // SA1019: We must test deprecated fields.
	dst.Spec.InitialSSHKnownHosts = v1beta1.SSHHostsSpec(argocd.Spec.InitialSSHKnownHosts)
	dst.Spec.KustomizeBuildOptions = argocd.Spec.KustomizeBuildOptions
	dst.Spec.KustomizeVersions = ConvertAlphaToBetaKustomizeVersions(argocd.Spec.KustomizeVersions)
	dst.Spec.OIDCConfig = argocd.Spec.OIDCConfig
	dst.Spec.Monitoring = v1beta1.ArgoCDMonitoringSpec(argocd.Spec.Monitoring)
	dst.Spec.NodePlacement = (*v1beta1.ArgoCDNodePlacementSpec)(argocd.Spec.NodePlacement)
	dst.Spec.Notifications = v1beta1.ArgoCDNotifications(argocd.Spec.Notifications)
	dst.Spec.Prometheus = *ConvertAlphaToBetaPrometheus(&argocd.Spec.Prometheus)
	dst.Spec.RBAC = v1beta1.ArgoCDRBACSpec(argocd.Spec.RBAC)
	dst.Spec.Redis = *ConvertAlphaToBetaRedis(&argocd.Spec.Redis)
	dst.Spec.Repo = *ConvertAlphaToBetaRepo(&argocd.Spec.Repo)
	//lint:ignore SA1019 known to be deprecated
	dst.Spec.RepositoryCredentials = argocd.Spec.RepositoryCredentials //nolint:staticcheck // SA1019: We must test deprecated fields.
	dst.Spec.ResourceHealthChecks = ConvertAlphaToBetaResourceHealthChecks(argocd.Spec.ResourceHealthChecks)
	dst.Spec.ResourceIgnoreDifferences = ConvertAlphaToBetaResourceIgnoreDifferences(argocd.Spec.ResourceIgnoreDifferences)
	dst.Spec.ResourceActions = ConvertAlphaToBetaResourceActions(argocd.Spec.ResourceActions)
	dst.Spec.ResourceExclusions = argocd.Spec.ResourceExclusions
	dst.Spec.ResourceInclusions = argocd.Spec.ResourceInclusions
	dst.Spec.ResourceTrackingMethod = argocd.Spec.ResourceTrackingMethod
	dst.Spec.Server = *ConvertAlphaToBetaServer(&argocd.Spec.Server)
	dst.Spec.SourceNamespaces = argocd.Spec.SourceNamespaces
	dst.Spec.StatusBadgeEnabled = argocd.Spec.StatusBadgeEnabled
	dst.Spec.TLS = *ConvertAlphaToBetaTLS(&argocd.Spec.TLS)
	dst.Spec.UsersAnonymousEnabled = argocd.Spec.UsersAnonymousEnabled
	dst.Spec.Version = argocd.Spec.Version
	dst.Spec.Banner = (*v1beta1.Banner)(argocd.Spec.Banner)
	dst.Spec.DefaultClusterScopedRoleDisabled = argocd.Spec.DefaultClusterScopedRoleDisabled
	dst.Spec.AggregatedClusterRoles = argocd.Spec.AggregatedClusterRoles
	dst.Spec.ArgoCDAgent = ConvertAlphaToBetaArgoCDAgent(argocd.Spec.ArgoCDAgent)
	dst.Spec.NamespaceManagement = ConvertAlphaToBetaNamespaceManagement(argocd.Spec.NamespaceManagement)

	// Status conversion
	dst.Status = v1beta1.ArgoCDStatus(argocd.Status)

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this (v1alpha1) version.
func (argocd *ArgoCD) ConvertFrom(srcRaw conversion.Hub) error {
	conversionLogger.V(1).Info("v1beta1 to v1alpha1 conversion requested.")

	src := srcRaw.(*v1beta1.ArgoCD)

	// ObjectMeta conversion
	argocd.ObjectMeta = src.ObjectMeta

	// Spec conversion

	// sso field
	// ignoring conversions of sso fields from v1beta1 to deprecated v1alpha1 as
	// there is no data loss since the new fields in v1beta1 are also present in v1alpha1 &
	// v1alpha1 is not used in business logic & only exists for presentation
	sso := ConvertBetaToAlphaSSO(src.Spec.SSO)
	argocd.Spec.SSO = sso

	// rest of the fields
	argocd.Spec.ApplicationSet = ConvertBetaToAlphaApplicationSet(src.Spec.ApplicationSet)
	argocd.Spec.ExtraConfig = src.Spec.ExtraConfig
	argocd.Spec.ApplicationInstanceLabelKey = src.Spec.ApplicationInstanceLabelKey
	//lint:ignore SA1019 known to be deprecated
	argocd.Spec.ConfigManagementPlugins = src.Spec.ConfigManagementPlugins //nolint:staticcheck // SA1019: We must test deprecated fields.
	argocd.Spec.Controller = *ConvertBetaToAlphaController(&src.Spec.Controller)
	argocd.Spec.DisableAdmin = src.Spec.DisableAdmin
	argocd.Spec.ExtraConfig = src.Spec.ExtraConfig
	argocd.Spec.GATrackingID = src.Spec.GATrackingID
	argocd.Spec.GAAnonymizeUsers = src.Spec.GAAnonymizeUsers
	//lint:ignore SA1019 known to be deprecated
	argocd.Spec.Grafana = *ConvertBetaToAlphaGrafana(&src.Spec.Grafana) //nolint:staticcheck // SA1019: We must test deprecated fields.
	argocd.Spec.HA = *ConvertBetaToAlphaHA(&src.Spec.HA)
	argocd.Spec.HelpChatURL = src.Spec.HelpChatURL
	argocd.Spec.HelpChatText = src.Spec.HelpChatText
	argocd.Spec.Image = src.Spec.Image
	argocd.Spec.Import = (*ArgoCDImportSpec)(src.Spec.Import)
	//lint:ignore SA1019 known to be deprecated
	argocd.Spec.InitialRepositories = src.Spec.InitialRepositories //nolint:staticcheck // SA1019: We must test deprecated fields.
	argocd.Spec.InitialSSHKnownHosts = SSHHostsSpec(src.Spec.InitialSSHKnownHosts)
	argocd.Spec.KustomizeBuildOptions = src.Spec.KustomizeBuildOptions
	argocd.Spec.KustomizeVersions = ConvertBetaToAlphaKustomizeVersions(src.Spec.KustomizeVersions)
	argocd.Spec.OIDCConfig = src.Spec.OIDCConfig
	argocd.Spec.Monitoring = ArgoCDMonitoringSpec(src.Spec.Monitoring)
	argocd.Spec.NodePlacement = (*ArgoCDNodePlacementSpec)(src.Spec.NodePlacement)
	argocd.Spec.Notifications = ArgoCDNotifications(src.Spec.Notifications)
	argocd.Spec.Prometheus = *ConvertBetaToAlphaPrometheus(&src.Spec.Prometheus)
	argocd.Spec.RBAC = ArgoCDRBACSpec(src.Spec.RBAC)
	argocd.Spec.Redis = *ConvertBetaToAlphaRedis(&src.Spec.Redis)
	argocd.Spec.Repo = *ConvertBetaToAlphaRepo(&src.Spec.Repo)
	//lint:ignore SA1019 known to be deprecated
	argocd.Spec.RepositoryCredentials = src.Spec.RepositoryCredentials //nolint:staticcheck // SA1019: We must test deprecated fields.
	argocd.Spec.ResourceHealthChecks = ConvertBetaToAlphaResourceHealthChecks(src.Spec.ResourceHealthChecks)
	argocd.Spec.ResourceIgnoreDifferences = ConvertBetaToAlphaResourceIgnoreDifferences(src.Spec.ResourceIgnoreDifferences)
	argocd.Spec.ResourceActions = ConvertBetaToAlphaResourceActions(src.Spec.ResourceActions)
	argocd.Spec.ResourceExclusions = src.Spec.ResourceExclusions
	argocd.Spec.ResourceInclusions = src.Spec.ResourceInclusions
	argocd.Spec.ResourceTrackingMethod = src.Spec.ResourceTrackingMethod
	argocd.Spec.Server = *ConvertBetaToAlphaServer(&src.Spec.Server)
	argocd.Spec.SourceNamespaces = src.Spec.SourceNamespaces
	argocd.Spec.StatusBadgeEnabled = src.Spec.StatusBadgeEnabled
	argocd.Spec.TLS = *ConvertBetaToAlphaTLS(&src.Spec.TLS)
	argocd.Spec.UsersAnonymousEnabled = src.Spec.UsersAnonymousEnabled
	argocd.Spec.Version = src.Spec.Version
	argocd.Spec.Banner = (*Banner)(src.Spec.Banner)
	argocd.Spec.DefaultClusterScopedRoleDisabled = src.Spec.DefaultClusterScopedRoleDisabled
	argocd.Spec.AggregatedClusterRoles = src.Spec.AggregatedClusterRoles
	argocd.Spec.ArgoCDAgent = ConvertBetaToAlphaArgoCDAgent(src.Spec.ArgoCDAgent)
	argocd.Spec.NamespaceManagement = ConvertBetaToAlphaNamespaceManagement(src.Spec.NamespaceManagement)

	// Status conversion
	argocd.Status = ArgoCDStatus(src.Status)

	return nil
}

func ConvertAlphaToBetaController(src *ArgoCDApplicationControllerSpec) *v1beta1.ArgoCDApplicationControllerSpec {
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

func ConvertAlphaToBetaRedis(src *ArgoCDRedisSpec) *v1beta1.ArgoCDRedisSpec {
	var dst *v1beta1.ArgoCDRedisSpec
	if src != nil {
		dst = &v1beta1.ArgoCDRedisSpec{
			AutoTLS:                src.AutoTLS,
			DisableTLSVerification: src.DisableTLSVerification,
			Image:                  src.Image,
			Resources:              src.Resources,
			Version:                src.Version,
		}
	}
	return dst
}

func ConvertAlphaToBetaRepo(src *ArgoCDRepoSpec) *v1beta1.ArgoCDRepoSpec {
	var dst *v1beta1.ArgoCDRepoSpec
	if src != nil {
		dst = &v1beta1.ArgoCDRepoSpec{
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

func ConvertAlphaToBetaWebhookServer(src *WebhookServerSpec) *v1beta1.WebhookServerSpec {
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

func ConvertAlphaToBetaApplicationSet(src *ArgoCDApplicationSet) *v1beta1.ArgoCDApplicationSet {
	var dst *v1beta1.ArgoCDApplicationSet
	if src != nil {
		dst = &v1beta1.ArgoCDApplicationSet{
			Env:              src.Env,
			ExtraCommandArgs: src.ExtraCommandArgs,
			Image:            src.Image,
			Version:          src.Version,
			Resources:        src.Resources,
			LogLevel:         src.LogLevel,
			WebhookServer:    *ConvertAlphaToBetaWebhookServer(&src.WebhookServer),
			LogFormat:        src.LogFormat,
		}
	}
	return dst
}

func ConvertAlphaToBetaGrafana(src *ArgoCDGrafanaSpec) *v1beta1.ArgoCDGrafanaSpec {
	var dst *v1beta1.ArgoCDGrafanaSpec
	if src != nil {
		dst = &v1beta1.ArgoCDGrafanaSpec{
			Enabled:   src.Enabled,
			Host:      src.Host,
			Image:     src.Image,
			Ingress:   v1beta1.ArgoCDIngressSpec(src.Ingress),
			Resources: src.Resources,
			Route:     v1beta1.ArgoCDRouteSpec(src.Route),
			Size:      src.Size,
			Version:   src.Version,
		}
	}
	return dst
}

func ConvertAlphaToBetaPrometheus(src *ArgoCDPrometheusSpec) *v1beta1.ArgoCDPrometheusSpec {
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

func ConvertAlphaToBetaSSO(src *ArgoCDSSOSpec) *v1beta1.ArgoCDSSOSpec {
	var dst *v1beta1.ArgoCDSSOSpec
	if src != nil {
		dst = &v1beta1.ArgoCDSSOSpec{
			Provider: v1beta1.SSOProviderType(src.Provider),
			Dex:      ConvertAlphaToBetaDex(src.Dex),
			Keycloak: (*v1beta1.ArgoCDKeycloakSpec)(src.Keycloak),
		}
	}
	return dst
}

func ConvertAlphaToBetaDex(src *ArgoCDDexSpec) *v1beta1.ArgoCDDexSpec {
	var dst *v1beta1.ArgoCDDexSpec
	if src != nil {
		dst = &v1beta1.ArgoCDDexSpec{
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

func ConvertAlphaToBetaHA(src *ArgoCDHASpec) *v1beta1.ArgoCDHASpec {
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

func ConvertAlphaToBetaTLS(src *ArgoCDTLSSpec) *v1beta1.ArgoCDTLSSpec {
	var dst *v1beta1.ArgoCDTLSSpec
	if src != nil {
		dst = &v1beta1.ArgoCDTLSSpec{
			CA:           v1beta1.ArgoCDCASpec(src.CA),
			InitialCerts: src.InitialCerts,
		}
	}
	return dst
}

func ConvertAlphaToBetaServer(src *ArgoCDServerSpec) *v1beta1.ArgoCDServerSpec {
	var dst *v1beta1.ArgoCDServerSpec
	if src != nil {
		dst = &v1beta1.ArgoCDServerSpec{
			Autoscale:        v1beta1.ArgoCDServerAutoscaleSpec(src.Autoscale),
			GRPC:             *ConvertAlphaToBetaGRPC(&src.GRPC),
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

func ConvertAlphaToBetaGRPC(src *ArgoCDServerGRPCSpec) *v1beta1.ArgoCDServerGRPCSpec {
	var dst *v1beta1.ArgoCDServerGRPCSpec
	if src != nil {
		dst = &v1beta1.ArgoCDServerGRPCSpec{
			Host:    src.Host,
			Ingress: v1beta1.ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

func ConvertAlphaToBetaKustomizeVersions(src []KustomizeVersionSpec) []v1beta1.KustomizeVersionSpec {
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

func ConvertAlphaToBetaResourceIgnoreDifferences(src *ResourceIgnoreDifference) *v1beta1.ResourceIgnoreDifference {
	var dst *v1beta1.ResourceIgnoreDifference
	if src != nil {
		dst = &v1beta1.ResourceIgnoreDifference{
			All:                 (*v1beta1.IgnoreDifferenceCustomization)(src.All),
			ResourceIdentifiers: ConvertAlphaToBetaResourceIdentifiers(src.ResourceIdentifiers),
		}
	}
	return dst
}

func ConvertAlphaToBetaResourceIdentifiers(src []ResourceIdentifiers) []v1beta1.ResourceIdentifiers {
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

func ConvertAlphaToBetaResourceActions(src []ResourceAction) []v1beta1.ResourceAction {
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

func ConvertAlphaToBetaResourceHealthChecks(src []ResourceHealthCheck) []v1beta1.ResourceHealthCheck {
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

func ConvertBetaToAlphaController(src *v1beta1.ArgoCDApplicationControllerSpec) *ArgoCDApplicationControllerSpec {
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

func ConvertBetaToAlphaWebhookServer(src *v1beta1.WebhookServerSpec) *WebhookServerSpec {
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

func ConvertBetaToAlphaApplicationSet(src *v1beta1.ArgoCDApplicationSet) *ArgoCDApplicationSet {
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
			LogFormat:        src.LogFormat,
		}
	}
	return dst
}

func ConvertBetaToAlphaGrafana(src *v1beta1.ArgoCDGrafanaSpec) *ArgoCDGrafanaSpec {
	var dst *ArgoCDGrafanaSpec
	if src != nil {
		dst = &ArgoCDGrafanaSpec{
			Enabled:   src.Enabled,
			Host:      src.Host,
			Image:     src.Image,
			Ingress:   ArgoCDIngressSpec(src.Ingress),
			Resources: src.Resources,
			Route:     ArgoCDRouteSpec(src.Route),
			Size:      src.Size,
			Version:   src.Version,
		}
	}
	return dst
}

func ConvertBetaToAlphaPrometheus(src *v1beta1.ArgoCDPrometheusSpec) *ArgoCDPrometheusSpec {
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

func ConvertBetaToAlphaSSO(src *v1beta1.ArgoCDSSOSpec) *ArgoCDSSOSpec {
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

func ConvertBetaToAlphaDex(src *v1beta1.ArgoCDDexSpec) *ArgoCDDexSpec {
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

func ConvertBetaToAlphaHA(src *v1beta1.ArgoCDHASpec) *ArgoCDHASpec {
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

func ConvertBetaToAlphaTLS(src *v1beta1.ArgoCDTLSSpec) *ArgoCDTLSSpec {
	var dst *ArgoCDTLSSpec
	if src != nil {
		dst = &ArgoCDTLSSpec{
			CA:           ArgoCDCASpec(src.CA),
			InitialCerts: src.InitialCerts,
		}
	}
	return dst
}

func ConvertBetaToAlphaServer(src *v1beta1.ArgoCDServerSpec) *ArgoCDServerSpec {
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

func ConvertBetaToAlphaGRPC(src *v1beta1.ArgoCDServerGRPCSpec) *ArgoCDServerGRPCSpec {
	var dst *ArgoCDServerGRPCSpec
	if src != nil {
		dst = &ArgoCDServerGRPCSpec{
			Host:    src.Host,
			Ingress: ArgoCDIngressSpec(src.Ingress),
		}
	}
	return dst
}

func ConvertBetaToAlphaKustomizeVersions(src []v1beta1.KustomizeVersionSpec) []KustomizeVersionSpec {
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

func ConvertBetaToAlphaResourceIgnoreDifferences(src *v1beta1.ResourceIgnoreDifference) *ResourceIgnoreDifference {
	var dst *ResourceIgnoreDifference
	if src != nil {
		dst = &ResourceIgnoreDifference{
			All:                 (*IgnoreDifferenceCustomization)(src.All),
			ResourceIdentifiers: ConvertBetaToAlphaResourceIdentifiers(src.ResourceIdentifiers),
		}
	}
	return dst
}

func ConvertBetaToAlphaResourceIdentifiers(src []v1beta1.ResourceIdentifiers) []ResourceIdentifiers {
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

func ConvertBetaToAlphaResourceActions(src []v1beta1.ResourceAction) []ResourceAction {
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

func ConvertBetaToAlphaResourceHealthChecks(src []v1beta1.ResourceHealthCheck) []ResourceHealthCheck {
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

func ConvertBetaToAlphaRedis(src *v1beta1.ArgoCDRedisSpec) *ArgoCDRedisSpec {
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

func ConvertBetaToAlphaRepo(src *v1beta1.ArgoCDRepoSpec) *ArgoCDRepoSpec {
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

func ConvertAlphaToBetaArgoCDAgent(src *ArgoCDAgentSpec) *v1beta1.ArgoCDAgentSpec {
	var dst *v1beta1.ArgoCDAgentSpec
	if src != nil {
		dst = &v1beta1.ArgoCDAgentSpec{
			Principal: ConvertAlphaToBetaPrincipal(src.Principal),
		}
	}
	return dst
}

func ConvertBetaToAlphaNamespaceManagement(src []v1beta1.ManagedNamespaces) []ManagedNamespaces {
	var dst []ManagedNamespaces
	for _, s := range src {
		dst = append(dst, ManagedNamespaces{
			Name:           s.Name,
			AllowManagedBy: s.AllowManagedBy,
		})
	}
	return dst
}

func ConvertAlphaToBetaPrincipal(src *PrincipalSpec) *v1beta1.PrincipalSpec {
	var dst *v1beta1.PrincipalSpec
	if src != nil {
		dst = &v1beta1.PrincipalSpec{
			Enabled:           src.Enabled,
			AllowedNamespaces: src.AllowedNamespaces,
			JWTAllowGenerate:  src.JWTAllowGenerate,
			Auth:              src.Auth,
			LogLevel:          src.LogLevel,
			Image:             src.Image,
		}
	}
	return dst
}

func ConvertBetaToAlphaArgoCDAgent(src *v1beta1.ArgoCDAgentSpec) *ArgoCDAgentSpec {
	var dst *ArgoCDAgentSpec
	if src != nil {
		dst = &ArgoCDAgentSpec{
			Principal: ConvertBetaToAlphaPrincipal(src.Principal),
		}
	}
	return dst
}

func ConvertBetaToAlphaPrincipal(src *v1beta1.PrincipalSpec) *PrincipalSpec {
	var dst *PrincipalSpec
	if src != nil {
		dst = &PrincipalSpec{
			Enabled:           src.Enabled,
			AllowedNamespaces: src.AllowedNamespaces,
			JWTAllowGenerate:  src.JWTAllowGenerate,
			Auth:              src.Auth,
			LogLevel:          src.LogLevel,
			Image:             src.Image,
		}
	}
	return dst
}

func ConvertAlphaToBetaNamespaceManagement(src []ManagedNamespaces) []v1beta1.ManagedNamespaces {
	var dst []v1beta1.ManagedNamespaces
	for _, s := range src {
		dst = append(dst, v1beta1.ManagedNamespaces{
			Name:           s.Name,
			AllowManagedBy: s.AllowManagedBy,
		},
		)
	}
	return dst
}
