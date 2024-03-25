package server

import (
	"fmt"
	"os"
	"strings"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func getCustomRoleName() string {
	return os.Getenv(ArgoCDServerClusterRoleEnvVar)
}

func (sr *ServerReconciler) getManagedNsRBAC() ([]types.NamespacedName, []types.NamespacedName, error) {
	roles := []types.NamespacedName{}
	rbs := []types.NamespacedName{}

	compReq, err := argocdcommon.GetComponentLabelRequirement(component)
	if err != nil {
		return nil, nil, err
	}

	rbacReq, err := argocdcommon.GetRbacTypeLabelRequirement(common.ArgoCDRBACTypeResourceMananagement)
	if err != nil {
		return nil, nil, err
	}

	ls := argocdcommon.GetLabelSelector(*compReq, *rbacReq)

	for ns := range sr.ManagedNamespaces {
		nsRoles, nsRbs := argocdcommon.GetRBACToBeDeleted(ns, ls, sr.Client, sr.Logger)
		roles = append(roles, nsRoles...)
		rbs = append(rbs, nsRbs...)
	}

	return roles, rbs, nil
}

func (sr *ServerReconciler) getSourceNsRBAC() ([]types.NamespacedName, []types.NamespacedName, error) {
	roles := []types.NamespacedName{}
	rbs := []types.NamespacedName{}

	compReq, err := argocdcommon.GetComponentLabelRequirement(component)
	if err != nil {
		return nil, nil, err
	}

	rbacReq, err := argocdcommon.GetRbacTypeLabelRequirement(common.ArgoCDRBACTypeAppManagement)
	if err != nil {
		return nil, nil, err
	}

	ls := argocdcommon.GetLabelSelector(*compReq, *rbacReq)

	for ns := range sr.SourceNamespaces {
		nsRoles, nsRbs := argocdcommon.GetRBACToBeDeleted(ns, ls, sr.Client, sr.Logger)
		roles = append(roles, nsRoles...)
		rbs = append(rbs, nsRbs...)
	}

	return roles, rbs, nil
}

func (sr *ServerReconciler) getAppsetSourceNsRBAC() ([]types.NamespacedName, []types.NamespacedName, error) {
	roles := []types.NamespacedName{}
	rbs := []types.NamespacedName{}

	compReq, err := argocdcommon.GetComponentLabelRequirement(component)
	if err != nil {
		return nil, nil, err
	}

	rbacReq, err := argocdcommon.GetRbacTypeLabelRequirement(common.ArgoCDRBACTypeAppSetManagement)
	if err != nil {
		return nil, nil, err
	}

	ls := argocdcommon.GetLabelSelector(*compReq, *rbacReq)

	for ns := range sr.AppsetSourceNamespaces {
		nsRoles, nsRbs := argocdcommon.GetRBACToBeDeleted(ns, ls, sr.Client, sr.Logger)
		roles = append(roles, nsRoles...)
		rbs = append(rbs, nsRbs...)
	}

	return roles, rbs, nil
}

// getHost will return the host for the given ArgoCD.
func (sr *ServerReconciler) getHost() string {
	host := sr.Instance.Name
	if len(sr.Instance.Spec.Server.Host) > 0 {
		tmpHost, err := argocdcommon.ShortenHostname(sr.Instance.Spec.Server.Host)
		if err != nil {
			sr.Logger.Error(err, "getHost: failed to shorten hostname")
		} else {
			host = tmpHost
		}
	}
	return host
}

// getPathOrDefault will return the Ingress Path for the Argo CD component.
func getPathOrDefault(path string) string {
	result := common.ArgoCDDefaultIngressPath
	if len(path) > 0 {
		result = path
	}
	return result
}

// getGRPCHost will return the GRPC host for the given ArgoCD.
func (sr *ServerReconciler) getGRPCHost() string {
	host := argoutil.NameWithSuffix(sr.Instance.Name, "grpc")
	if len(sr.Instance.Spec.Server.GRPC.Host) > 0 {
		host = sr.Instance.Spec.Server.GRPC.Host
	}
	return host
}

func (sr *ServerReconciler) getServiceType() corev1.ServiceType {
	svcType := corev1.ServiceTypeClusterIP
	// override service type if set in ArgoCD CR
	if len(sr.Instance.Spec.Server.Service.Type) > 0 {
		svcType = sr.Instance.Spec.Server.Service.Type
	}

	return svcType
}

// getCmd will return the command for the ArgoCD server component.
func (sr *ServerReconciler) getCmd() []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "argocd-server")

	if sr.Instance.Spec.Server.Insecure {
		cmd = append(cmd, "--insecure")
	}

	cmd = append(cmd, "--staticassets")
	cmd = append(cmd, "/shared/app")

	cmd = append(cmd, "--dex-server")
	cmd = append(cmd, sr.Dex.GetServerAddress())

	// reposerver flags
	if sr.RepoServer.UseTLS() {
		cmd = append(cmd, "--repo-server-strict-tls")
	}

	cmd = append(cmd, "--repo-server")
	cmd = append(cmd, sr.RepoServer.GetServerAddress())

	// redis flags
	cmd = append(cmd, "--redis")
	cmd = append(cmd, sr.Redis.GetServerAddress())

	if sr.Redis.UseTLS() {
		cmd = append(cmd, "--redis-use-tls")
		if sr.Redis.TLSVerificationDisabled() {
			cmd = append(cmd, "--redis-insecure-skip-tls-verify")
		} else {
			cmd = append(cmd, "--redis-ca-certificate", "/app/config/server/tls/redis/tls.crt")
		}
	}

	// set log level & format
	cmd = append(cmd, "--loglevel")
	cmd = append(cmd, argoutil.GetLogLevel(sr.Instance.Spec.Server.LogLevel))

	cmd = append(cmd, "--logformat")
	cmd = append(cmd, argoutil.GetLogFormat(sr.Instance.Spec.Server.LogFormat))

	// set source namespaces
	if sr.Instance.Spec.SourceNamespaces != nil && len(sr.Instance.Spec.SourceNamespaces) > 0 {
		cmd = append(cmd, "--application-namespaces", fmt.Sprint(strings.Join(sr.Instance.Spec.SourceNamespaces, ",")))
	}

	// extra args should always be added at the end
	extraArgs := sr.Instance.Spec.Server.ExtraCommandArgs
	err := argocdcommon.IsMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}
	cmd = append(cmd, extraArgs...)

	return cmd
}
