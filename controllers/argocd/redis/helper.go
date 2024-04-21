package redis

import (
	"fmt"
	"strconv"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

const (
	initShTpl             = "init.sh.tpl"
	redisConfTpl          = "redis.conf.tpl"
	sentinelConfTpl       = "sentinel.conf.tpl"
	livenessShTpl         = "redis_liveness.sh.tpl"
	readinessShTpl        = "redis_readiness.sh.tpl"
	sentinelLivenessShTpl = "sentinel_liveness.sh.tpl"

	TLSPath     = "/app/config/redis/tls"
	TLSCertPath = "/app/config/redis/tls/tls.crt"
	TLSKeyPath  = "/app/config/redis/tls/tls.key"
)

func (rr *RedisReconciler) TLSVerificationDisabled() bool {
	return rr.Instance.Spec.Redis.DisableTLSVerification
}

// UseTLS determines whether Redis component should communicate with TLS or not
func (rr *RedisReconciler) UseTLS() bool {
	rr.TLSEnabled = argocdcommon.UseTLS(common.ArgoCDRedisServerTLSSecretName, rr.Instance.Namespace, rr.Client, rr.Logger)
	// returning for interface compliance
	return rr.TLSEnabled
}

// GetServerAddress will return the Redis service address for the given ArgoCD instance
func (rr *RedisReconciler) GetServerAddress() string {
	rr.varSetter()
	if rr.Instance.Spec.Redis.Remote != nil && *rr.Instance.Spec.Redis.Remote != "" {
		return *rr.Instance.Spec.Redis.Remote
	}
	if rr.Instance.Spec.HA.Enabled {
		return rr.GetHAProxyAddress()
	}
	return argoutil.FQDNwithPort(resourceName, rr.Instance.Namespace, common.DefaultRedisPort)
}

// getContainerImage will return the container image for the Redis server.
func (rr *RedisReconciler) getContainerImage() string {
	fn := func(cr *argoproj.ArgoCD) (string, string) {
		return cr.Spec.Redis.Image, cr.Spec.Redis.Version
	}
	return argocdcommon.GetContainerImage(fn, rr.Instance, common.RedisImageEnvVar, common.DefaultRedisImage, common.DefaultRedisVersion)
}

// getHAContainerImage will return the container image for the Redis server in HA mode.
func (rr *RedisReconciler) getHAContainerImage() string {
	fn := func(cr *argoproj.ArgoCD) (string, string) {
		return cr.Spec.Redis.Image, cr.Spec.Redis.Version
	}
	return argocdcommon.GetContainerImage(fn, rr.Instance, common.RedisHAImageEnvVar, common.ArgoCDDefaultRedisImage, common.ArgoCDDefaultRedisVersionHA)
}

// getHAResources will return the ResourceRequirements for the Redis container in HA mode
func (rr *RedisReconciler) getHAResources() corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if rr.Instance.Spec.HA.Resources != nil {
		resources = *rr.Instance.Spec.HA.Resources
	}
	return resources
}

// getResources will return the ResourceRequirements for the Redis container.
func (rr *RedisReconciler) getResources() corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if rr.Instance.Spec.Redis.Resources != nil {
		resources = *rr.Instance.Spec.Redis.Resources
	}
	return resources
}

func getHAReplicas() *int32 {
	replicas := common.DefaultRedisHAReplicas
	// TODO: Allow override of this value through CR?
	return &replicas
}

// getConf will load the redis configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func (rr *RedisReconciler) getConf() string {
	path := fmt.Sprintf("%s/%s", getConfigPath(), redisConfTpl)
	params := map[string]string{
		UseTLSKey: strconv.FormatBool(rr.TLSEnabled),
	}

	conf, err := util.LoadTemplateFile(path, params)
	if err != nil {
		rr.Logger.Error(err, "getConf: failed to load redis configuration")
		return ""
	}
	return conf
}

// getInitScript will load the redis init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func (rr *RedisReconciler) getInitScript() string {
	path := fmt.Sprintf("%s/%s", getConfigPath(), initShTpl)
	params := map[string]string{
		ServiceNameKey: HAResourceName,
		UseTLSKey:      strconv.FormatBool(rr.TLSEnabled),
	}

	script, err := util.LoadTemplateFile(path, params)
	if err != nil {
		rr.Logger.Error(err, "getInitScript: failed to load redis init script")
		return ""
	}
	return script
}

// getLivenessScript will load the redis liveness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func (rr *RedisReconciler) getLivenessScript() string {
	path := fmt.Sprintf("%s/%s", getConfigPath(), livenessShTpl)
	params := map[string]string{
		UseTLSKey: strconv.FormatBool(rr.TLSEnabled),
	}
	script, err := util.LoadTemplateFile(path, params)
	if err != nil {
		rr.Logger.Error(err, "getLivenessScript: failed to load redis liveness script")
		return ""
	}
	return script
}

// getReadinessScript will load the redis readiness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func (rr *RedisReconciler) getReadinessScript() string {
	path := fmt.Sprintf("%s/%s", getConfigPath(), readinessShTpl)
	params := map[string]string{
		UseTLSKey: strconv.FormatBool(rr.TLSEnabled),
	}
	script, err := util.LoadTemplateFile(path, params)
	if err != nil {
		rr.Logger.Error(err, "getLivenessScript: failed to load redis readiness script")
		return ""
	}
	return script
}

// getSentinelConf will load the redis sentinel configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func (rr *RedisReconciler) getSentinelConf() string {
	path := fmt.Sprintf("%s/%s", getConfigPath(), sentinelConfTpl)
	params := map[string]string{
		UseTLSKey: strconv.FormatBool(rr.TLSEnabled),
	}

	conf, err := util.LoadTemplateFile(path, params)
	if err != nil {
		rr.Logger.Error(err, "getSentinelConf: failed to load redis sentinel configuration")
		return ""
	}
	return conf
}

// getSentinelLivenessScript will load the redis liveness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func (rr *RedisReconciler) getSentinelLivenessScript() string {
	path := fmt.Sprintf("%s/%s", getConfigPath(), sentinelLivenessShTpl)
	params := map[string]string{
		UseTLSKey: strconv.FormatBool(rr.TLSEnabled),
	}

	script, err := util.LoadTemplateFile(path, params)
	if err != nil {
		rr.Logger.Error(err, "getSentinelConf: failed to load sentinel liveness script")
		return ""
	}
	return script
}

// getConfigPath will return the path for the Redis configuration templates.
func getConfigPath() string {
	path := common.DefaultRedisConfigPath
	if _, val := util.CaseInsensitiveGetenv(common.RedisConfigPathEnvVar); val != "" {
		path = val
	}
	return path
}

// getCmd will return the list of cmd args to be supplied to the redis container
func (rr *RedisReconciler) getCmd() []string {
	args := make([]string, 0)

	args = append(args, "--save", "")
	args = append(args, "--appendonly", "no")

	if rr.TLSEnabled {
		args = append(args, "--tls-port", strconv.Itoa(common.DefaultRedisPort))
		args = append(args, "--port", "0")

		args = append(args, "--tls-cert-file", TLSCertPath)
		args = append(args, "--tls-key-file", TLSKeyPath)
		args = append(args, "--tls-auth-clients", "no")
	}

	return args
}
