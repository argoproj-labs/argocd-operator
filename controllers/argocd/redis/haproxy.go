package redis

import (
	"fmt"
	"strconv"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
)

const (
	haproxyCfgTpl  = "haproxy.cfg.tpl"
	haproxyInitTpl = "haproxy_init.sh.tpl"
)

// GetHAProxyAddress will return the Redis HA Proxy service address for the given ArgoCD instance
func (rr *RedisReconciler) GetHAProxyAddress() string {
	return argoutil.FQDNwithPort(HAProxyResourceName, rr.Instance.Namespace, common.DefaultRedisPort)
}

// getHAProxyContainerImage will return the container image for the Redis HA Proxy.
func (rr *RedisReconciler) getHAProxyContainerImage() string {
	fn := func(cr *argoproj.ArgoCD) (string, string) {
		return cr.Spec.HA.RedisProxyImage, cr.Spec.HA.RedisProxyVersion
	}
	return argocdcommon.GetContainerImage(fn, rr.Instance, common.RedisHAProxyImageEnvVar, common.DefaultRedisHAProxyImage, common.DefaultRedisHAProxyVersion)
}

// getHAProxyConfig will load the Redis HA Proxy configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func (rr *RedisReconciler) getHAProxyConfig() string {
	path := fmt.Sprintf("%s/%s", getConfigPath(), haproxyCfgTpl)
	params := map[string]string{
		ServiceNameKey: HAResourceName,
		UseTLSKey:      strconv.FormatBool(rr.TLSEnabled),
	}

	script, err := util.LoadTemplateFile(path, params)
	if err != nil {
		rr.Logger.Error(err, "GetHAProxyConfig: failed to load haproxy configuration")
		return ""
	}
	return script
}

// getHAProxyScript will load the Redis HA Proxy init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func (rr *RedisReconciler) getHAProxyScript() string {
	path := fmt.Sprintf("%s/%s", getConfigPath(), haproxyInitTpl)
	params := map[string]string{
		ServiceNameKey: HAResourceName,
	}

	script, err := util.LoadTemplateFile(path, params)
	if err != nil {
		rr.Logger.Error(err, "GetHAProxyScript: failed to load haproxy init script")
		return ""
	}
	return script
}
