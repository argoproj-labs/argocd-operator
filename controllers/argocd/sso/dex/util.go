package dex

import (
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

// GetDexServerAddress will return the Dex server address.
func GetDexServerAddress(name string, namespace string) string {
	return fmt.Sprintf("https://%s", argoutil.FqdnServiceRef(argoutil.NameWithSuffix(name, ArgoCDDexControllerComponent), namespace, common.ArgoCDDefaultDexHTTPPort))
}
