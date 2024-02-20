package argocd

import (
	"fmt"
	"reflect"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"gopkg.in/yaml.v2"
)

const (
	healthKey     = "health"
	ignoreDIffKey = "ignoreDifferences"
	actionsKey    = "actions"
	allKey        = "all"
)

// getApplicationInstanceLabelKey returns the application instance label key for the given ArgoCD.
func (r *ArgoCDReconciler) getApplicationInstanceLabelKey() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.ApplicationInstanceLabelKey, common.ArgoCDDefaultApplicationInstanceLabelKey).(string)
}

// getCAConfigMapName returns the CA ConfigMap name for the given ArgoCD.
func (r *ArgoCDReconciler) getCAConfigMapName() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.TLS.CA.ConfigMapName, argoutil.GenerateResourceName(r.Instance.Name, common.CASuffix)).(string)
}

// TO DO: move to appset component
// getSCMRootCAConfigMapName returns the SCMRootCA ConfigMap name for the given ArgoCD ApplicationSet Controller.
func (r *ArgoCDReconciler) getSCMRootCAConfigMapName() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.ApplicationSet.SCMRootCAConfigMap, "").(string)
}

// getConfigManagementPlugins returns the config management plugins for the given ArgoCD.
func (r *ArgoCDReconciler) getConfigManagementPlugins() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.ConfigManagementPlugins, common.ArgoCDDefaultConfigManagementPlugins).(string)
}

// getGATrackingID returns the google analytics tracking ID for the given Argo CD.
func (r *ArgoCDReconciler) getGATrackingID() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.GATrackingID, common.ArgoCDDefaultGATrackingID).(string)
}

// getHelpChatURL returns the help chat URL for the given Argo CD.
func (r *ArgoCDReconciler) getHelpChatURL() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.HelpChatURL, common.ArgoCDDefaultHelpChatURL).(string)
}

// getHelpChatText returns the help chat text for the given Argo CD.
func (r *ArgoCDReconciler) getHelpChatText() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.HelpChatText, common.ArgoCDDefaultHelpChatText).(string)
}

// getKustomizeBuildOptions returns the kuztomize build options for the given ArgoCD.
func (r *ArgoCDReconciler) getKustomizeBuildOptions() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.KustomizeBuildOptions, common.ArgoCDDefaultKustomizeBuildOptions).(string)
}

// getOIDCConfig returns the OIDC configuration for the given  instance.
func (r *ArgoCDReconciler) getOIDCConfig() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.OIDCConfig, common.ArgoCDDefaultOIDCConfig).(string)
}

// getRBACPolicy will return the RBAC policy for the given ArgoCD instance.
func (r *ArgoCDReconciler) getRBACPolicy() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.RBAC.Policy, common.ArgoCDDefaultRBACPolicy).(string)
}

// getRBACPolicyMatcherMode will return the RBAC policy matcher mode for the given ArgoCD instance.
func (r *ArgoCDReconciler) getRBACPolicyMatcherMode() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.RBAC.PolicyMatcherMode, "").(string)
}

// getRBACDefaultPolicy will return the RBAC default policy for the given ArgoCD instance.
func (r *ArgoCDReconciler) getRBACDefaultPolicy() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.RBAC.DefaultPolicy, common.ArgoCDDefaultRBACPolicy).(string)
}

// getRBACScopes will return the RBAC scopes for the given ArgoCD instance.
func (r *ArgoCDReconciler) getRBACScopes() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.RBAC.Scopes, common.ArgoCDDefaultRBACScopes).(string)
}

// getResourceExclusions will return the resource exclusions for the given ArgoCD instance.
func (r *ArgoCDReconciler) getResourceExclusions() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.ResourceExclusions, common.ArgoCDDefaultResourceExclusions).(string)
}

// getResourceInclusions will return the resource inclusions for the given ArgoCD instance.
func (r *ArgoCDReconciler) getResourceInclusions() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.ResourceInclusions, common.ArgoCDDefaultResourceInclusions).(string)
}

// getInitialRepositories will return the initial repositories for the given ArgoCD instance.
func (r *ArgoCDReconciler) getInitialRepositories() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.InitialRepositories, common.ArgoCDDefaultRepositories).(string)
}

// getRepositoryCredentials will return the repository credentials for the given ArgoCD instance.
func (r *ArgoCDReconciler) getRepositoryCredentials() string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.RepositoryCredentials, common.ArgoCDDefaultRepositoryCredentials).(string)
}

// getInitialTLSCerts will return the TLS certs for the given ArgoCD instance.
func (r *ArgoCDReconciler) getInitialTLSCerts() map[string]string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.TLS.InitialCerts, make(map[string]string)).(map[string]string)
}

// getSSHKnownHosts will return the SSH Known Hosts data for the given ArgoCD instance.
func (r *ArgoCDReconciler) getInitialSSHKnownHosts() string {
	skh := common.ArgoCDDefaultSSHKnownHosts
	if r.Instance.Spec.InitialSSHKnownHosts.ExcludeDefaultHosts {
		skh = ""
	}
	if len(r.Instance.Spec.InitialSSHKnownHosts.Keys) > 0 {
		skh += r.Instance.Spec.InitialSSHKnownHosts.Keys
	}
	return skh
}

func (r *ArgoCDReconciler) getDisableAdmin() string {
	return fmt.Sprintf("%t", !r.Instance.Spec.DisableAdmin)
}

func (r *ArgoCDReconciler) getGAAnonymizeUsers() string {
	return fmt.Sprintf("%t", r.Instance.Spec.GAAnonymizeUsers)
}

func (r *ArgoCDReconciler) getStatusBadgeEnabled() string {
	return fmt.Sprintf("%t", r.Instance.Spec.StatusBadgeEnabled)
}

func (r *ArgoCDReconciler) getUsersAnonymousEnabled() string {
	return fmt.Sprintf("%t", r.Instance.Spec.UsersAnonymousEnabled)
}

// getResourceTrackingMethod will return the resource tracking method for the given ArgoCD instance
func (r *ArgoCDReconciler) getResourceTrackingMethod() string {
	rtm := argoproj.ParseResourceTrackingMethod(r.Instance.Spec.ResourceTrackingMethod)
	if rtm == argoproj.ResourceTrackingMethodInvalid {
		r.Logger.Debug(fmt.Sprintf("found invalid resource tracking method '%s'; defaulting to 'label' method", r.Instance.Spec.ResourceTrackingMethod))
	} else if r.Instance.Spec.ResourceTrackingMethod != "" {
		r.Logger.Debug(fmt.Sprintf("found resource tracking method '%s'", r.Instance.Spec.ResourceTrackingMethod))
	} else {
		r.Logger.Debug("using default resource tracking method 'label'")
	}
	return rtm.String()
}

func (r *ArgoCDReconciler) getKustomizeVersions() map[string]string {
	versions := make(map[string]string)
	for _, kv := range r.Instance.Spec.KustomizeVersions {
		versions[common.ArgoCDKeyKustomizeVersion+kv.Version] = kv.Path
	}
	return versions
}

func (r *ArgoCDReconciler) getBanner() map[string]string {
	banner := make(map[string]string)
	if r.Instance.Spec.Banner != nil {
		banner[common.ArgoCDKeyBannerContent] = argocdcommon.GetValueOrDefault(r.Instance.Spec.Banner.Content, "").(string)
		banner[common.ArgoCDKeyBannerURL] = argocdcommon.GetValueOrDefault(r.Instance.Spec.Banner.URL, "").(string)
	}
	return banner
}

func (r *ArgoCDReconciler) getExtraConfig() map[string]string {
	return argocdcommon.GetValueOrDefault(r.Instance.Spec.ExtraConfig, make(map[string]string)).(map[string]string)
}

// getResourceHealthChecks loads health customizations to `resource.customizations.health` from argocd-cm ConfigMap
func (r *ArgoCDReconciler) getResourceHealthChecks() map[string]string {
	healthCheck := make(map[string]string)

	if r.Instance.Spec.ResourceHealthChecks != nil {
		rhc := r.Instance.Spec.ResourceHealthChecks
		for _, hc := range rhc {
			subkey := util.ConstructString(util.DotSep, common.ArgoCDKeyResourceCustomizations, healthKey, util.ConstructString(util.UnderscoreSep, hc.Group, hc.Kind))
			subvalue := hc.Check
			healthCheck[subkey] = subvalue
		}
	}

	return healthCheck
}

// getResourceActions loads custom actions to `resource.customizations.actions` from argocd-cm ConfigMap
func (r *ArgoCDReconciler) getResourceActions() map[string]string {
	actions := make(map[string]string)

	if r.Instance.Spec.ResourceActions != nil {
		ra := r.Instance.Spec.ResourceActions
		for _, a := range ra {
			subkey := util.ConstructString(util.DotSep, common.ArgoCDKeyResourceCustomizations, actionsKey, util.ConstructString(util.UnderscoreSep, a.Group, a.Kind))
			subvalue := a.Action
			actions[subkey] = subvalue
		}
	}

	return actions
}

// getResourceIgnoreDifferences loads ignore differences customizations to `resource.customizations.ignoreDifferences` from argocd-cm ConfigMap
func (r *ArgoCDReconciler) getResourceIgnoreDifferences() map[string]string {
	ignoreDiff := make(map[string]string)

	if r.Instance.Spec.ResourceIgnoreDifferences != nil {
		rid := r.Instance.Spec.ResourceIgnoreDifferences

		if !reflect.DeepEqual(rid.All, &argoproj.IgnoreDifferenceCustomization{}) {
			subkey := util.ConstructString(util.DotSep, common.ArgoCDKeyResourceCustomizations, ignoreDIffKey, allKey)
			bytes, err := yaml.Marshal(rid.All)
			if err != nil {
				r.Logger.Error(err, "getResourceIgnoreDifferences")
				return ignoreDiff
			}
			subvalue := string(bytes)
			ignoreDiff[subkey] = subvalue
		}

		for _, id := range rid.ResourceIdentifiers {
			subkey := util.ConstructString(util.DotSep, common.ArgoCDKeyResourceCustomizations, ignoreDIffKey, util.ConstructString(util.UnderscoreSep, id.Group, id.Kind))
			bytes, err := yaml.Marshal(id.Customization)
			if err != nil {
				r.Logger.Error(err, "getResourceIgnoreDifferences")
				return ignoreDiff
			}
			subvalue := string(bytes)
			ignoreDiff[subkey] = subvalue
		}
	}

	return ignoreDiff
}
