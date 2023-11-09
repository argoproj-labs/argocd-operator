package dex

import (
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

// GetDexServerAddress will return the Dex server address.
func GetDexServerAddress(name string, namespace string) string {
	return fmt.Sprintf("https://%s", util.FqdnServiceRef(util.NameWithSuffix(name, ArgoCDDexControllerComponent), namespace, common.ArgoCDDefaultDexHTTPPort))
}

