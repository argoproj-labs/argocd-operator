package server

import (
	"os"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

func getCustomRoleName() string {
	return os.Getenv(ArgoCDServerClusterRoleEnvVar)
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
	host := argoutil.NameWithSuffix(cr.Name, "grpc")
	if len(cr.Spec.Server.GRPC.Host) > 0 {
		host = cr.Spec.Server.GRPC.Host
	}
	return host
}
