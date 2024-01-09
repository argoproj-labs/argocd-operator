package server

import (
	"fmt"
	"os"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

func getCustomRoleName() string {
	return os.Getenv(ArgoCDServerClusterRoleEnvVar)
}

func getRoleNameForSourceNamespace(argocdName, targetNamespace string) string {
	return fmt.Sprintf("%s_%s", argocdName, targetNamespace)
}

func getRoleBindingNameForSourceNamespace(argocdName, targetNamespace string) string {
	return fmt.Sprintf("%s_%s", argocdName, targetNamespace)
}

func getServiceAccountName(argoCDName string) string {
	return util.NameWithSuffix(argoCDName, ArgoCDServerSuffix)
}

func getRoleName(argoCDName string) string {
	return util.NameWithSuffix(argoCDName, ArgoCDServerSuffix)
}

func getRoleBindingName(argoCDName string) string {
	return util.NameWithSuffix(argoCDName, ArgoCDServerSuffix)
}

func getClusterRoleName(argoCDName, namespace string) string {
	return util.GenerateUniqueResourceName(argoCDName, argoCDName, ArgoCDServerSuffix)
}

func getClusterRoleBindingName(argoCDName, namespace string) string {
	return util.GenerateUniqueResourceName(argoCDName, argoCDName, ArgoCDServerSuffix)
}

func getDeploymentName(argoCDName string) string {
	return util.NameWithSuffix(argoCDName, ServerSuffix)
}

func getServiceName(argoCDName string) string {
	return util.NameWithSuffix(argoCDName, ServerSuffix)
}

func getHPAName(argoCDName string) string {
	return util.NameWithSuffix(argoCDName, ServerSuffix)
}

func getRouteName(argoCDName string) string {
	return util.NameWithSuffix(argoCDName, ServerSuffix)
}

func getIngressName(argoCDName string) string {
	return util.NameWithSuffix(argoCDName, "server")
}

func getGRPCIngressName(argoCDName string) string {
	return util.NameWithSuffix(argoCDName, "grpc")
}

// getHost will return the host for the given ArgoCD.
func getHost(cr *argoproj.ArgoCD) string {
	host := cr.Name
	if len(cr.Spec.Server.Host) > 0 {
		host = cr.Spec.Server.Host
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
func getGRPCHost(cr *argoproj.ArgoCD) string {
	host := util.NameWithSuffix(cr.Name, "grpc")
	if len(cr.Spec.Server.GRPC.Host) > 0 {
		host = cr.Spec.Server.GRPC.Host
	}
	return host
}
