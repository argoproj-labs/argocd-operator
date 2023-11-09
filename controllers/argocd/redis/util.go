package redis

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

// GetRedisServerAddress will return the Argo CD repo server address.
func GetRedisServerAddress(cr *argoproj.ArgoCD) string {
	name := cr.Name
	namespace := cr.Namespace
	if cr.Spec.HA.Enabled {
		return util.FqdnServiceRef(util.NameWithSuffix(name, ArgoCDRedisHAControllerComponent), namespace, common.ArgoCDDefaultRedisPort)
	}
	return util.FqdnServiceRef(util.NameWithSuffix(name, ArgoCDRedisControllerComponent), namespace, common.ArgoCDDefaultRedisPort)
}


func IsRedisTLSVerificationDisabled(cr *argoproj.ArgoCD) bool {
	return cr.Spec.Redis.DisableTLSVerification
}


