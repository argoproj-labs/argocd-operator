package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

// GetRepoServerAddress will return the Argo CD repo server address.
func GetRepoServerAddress(name string, namespace string) string {
	return argoutil.FqdnServiceRef(argoutil.NameWithSuffix(name, ArgoCDRepoServerControllerComponent), namespace, common.ArgoCDDefaultRepoServerPort)
}
