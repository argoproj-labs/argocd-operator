package dex

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

// GetServerAddress will return the Redis service address for the given ArgoCD instance
func (rr *DexReconciler) GetServerAddress() string {
	return argoutil.FQDNwithPort(resourceName, rr.Instance.Namespace, common.ArgoCDDefaultDexHTTPPort)
}
