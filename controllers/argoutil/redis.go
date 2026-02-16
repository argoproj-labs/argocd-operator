package argoutil

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"text/template"

	corev1 "k8s.io/api/core/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

const (
	RedisAuthVolumeName = "redis-initial-pass"
	RedisAuthMountPath  = "/app/config/redis-auth/"
)

func MountRedisAuthToArgo(cr *argoproj.ArgoCD) (volume corev1.Volume, mount corev1.VolumeMount) {
	volume = corev1.Volume{
		Name: RedisAuthVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: GetSecretNameWithSuffix(cr, "redis-initial-password"),
				Items: []corev1.KeyToPath{
					{
						// Mapping the legacy key-name, the operator customers can depend on, to the expected file name for argo-cd
						// Ref.: https://argo-cd.readthedocs.io/en/latest/faq/#using-file-based-redis-credentials-via-redis_creds_dir_path
						Key:  "admin.password",
						Path: "auth",
					},
					{
						// ACL file used by redis-server
						Key:  "users.acl",
						Path: "users.acl",
					},
				},
			},
		},
	}

	mount = corev1.VolumeMount{
		Name:      RedisAuthVolumeName,
		MountPath: RedisAuthMountPath,
	}

	return
}

func GetRedisAuthEnv() []corev1.EnvVar {
	return []corev1.EnvVar{{
		Name:  "REDIS_CREDS_DIR_PATH",
		Value: RedisAuthMountPath,
	}}
}

func GetRedisHAReplicas() *int32 {
	replicas := common.ArgoCDDefaultRedisHAReplicas
	// TODO: Allow override of this value through CR?
	return &replicas
}

// getRedisConfigPath will return the path for the Redis configuration templates.
func getRedisConfigPath() string {
	path := os.Getenv("REDIS_CONFIG_PATH")
	if len(path) > 0 {
		return path
	}
	return common.ArgoCDDefaultRedisConfigPath
}

// GetRedisInitScript will load the redis configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisConf(useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/redis.conf.tpl", getRedisConfigPath())
	params := map[string]string{
		"UseTLS": strconv.FormatBool(useTLSForRedis),
	}
	conf, err := loadTemplateFile(path, params)
	if err != nil {
		log.Error(err, "unable to load redis configuration")
		return ""
	}
	return conf
}

// GetRedisContainerImage will return the container image for the Redis server.
func GetRedisContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.Redis.Image
	if img == "" {
		img = common.ArgoCDDefaultRedisImage
		defaultImg = true
	}
	tag := cr.Spec.Redis.Version
	if tag == "" {
		tag = common.ArgoCDDefaultRedisVersion
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDRedisImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return CombineImageTag(img, tag)
}

// GetRedisHAContainerImage will return the container image for the Redis server in HA mode.
func GetRedisHAContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.Redis.Image
	if img == "" {
		img = common.ArgoCDDefaultRedisImage
		defaultImg = true
	}
	tag := cr.Spec.Redis.Version
	if tag == "" {
		tag = common.ArgoCDDefaultRedisVersionHA
		defaultTag = true
	}
	if e := os.Getenv(common.ArgoCDRedisHAImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return CombineImageTag(img, tag)
}

// getRedisHAProxyAddress will return the Redis HA Proxy service address for the given ArgoCD.
func getRedisHAProxyAddress(cr *argoproj.ArgoCD) string {
	return FqdnServiceRef("redis-ha-haproxy", common.ArgoCDDefaultRedisPort, cr)
}

// GetRedisHAProxyContainerImage will return the container image for the Redis HA Proxy.
func GetRedisHAProxyContainerImage(cr *argoproj.ArgoCD) string {
	defaultImg, defaultTag := false, false
	img := cr.Spec.HA.RedisProxyImage
	if len(img) <= 0 {
		img = common.ArgoCDDefaultRedisHAProxyImage
		defaultImg = true
	}

	tag := cr.Spec.HA.RedisProxyVersion
	if len(tag) <= 0 {
		tag = common.ArgoCDDefaultRedisHAProxyVersion
		defaultTag = true
	}

	if e := os.Getenv(common.ArgoCDRedisHAProxyImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}

	return CombineImageTag(img, tag)
}

// GetRedisInitScript will load the redis init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisInitScript(cr *argoproj.ArgoCD, useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": NameWithSuffix(cr.ObjectMeta, "redis-ha"),
		"UseTLS":      strconv.FormatBool(useTLSForRedis),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis init-script")
		return ""
	}
	return script
}

// getRedisHAProxySConfig will load the Redis HA Proxy configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisHAProxyConfig(cr *argoproj.ArgoCD, useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/haproxy.cfg.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": NameWithSuffix(cr.ObjectMeta, "redis-ha"),
		"UseTLS":      strconv.FormatBool(useTLSForRedis),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy configuration")
		return ""
	}
	return script
}

// GetRedisHAProxyScript will load the Redis HA Proxy init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisHAProxyScript(cr *argoproj.ArgoCD) string {
	path := fmt.Sprintf("%s/haproxy_init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": NameWithSuffix(cr.ObjectMeta, "redis-ha"),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy init script")
		return ""
	}
	return script
}

// GetRedisResources will return the ResourceRequirements for the Redis container.
func GetRedisResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Redis.Resources != nil {
		resources = *cr.Spec.Redis.Resources
	}

	return resources
}

// GetRedisHAResources will return the ResourceRequirements for the Redis HA.
func GetRedisHAResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.HA.Resources != nil {
		resources = *cr.Spec.HA.Resources
	}

	return resources
}

// GetRedisSentinelConf will load the redis sentinel configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisSentinelConf(useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/sentinel.conf.tpl", getRedisConfigPath())
	params := map[string]string{
		"UseTLS": strconv.FormatBool(useTLSForRedis),
	}
	conf, err := loadTemplateFile(path, params)
	if err != nil {
		log.Error(err, "unable to load redis sentinel configuration")
		return ""
	}
	return conf
}

// GetRedisLivenessScript will load the redis liveness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisLivenessScript(useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/redis_liveness.sh.tpl", getRedisConfigPath())
	params := map[string]string{
		"UseTLS": strconv.FormatBool(useTLSForRedis),
	}
	conf, err := loadTemplateFile(path, params)
	if err != nil {
		log.Error(err, "unable to load redis liveness script")
		return ""
	}
	return conf
}

// GetRedisReadinessScript will load the redis readiness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetRedisReadinessScript(useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/redis_readiness.sh.tpl", getRedisConfigPath())
	params := map[string]string{
		"UseTLS": strconv.FormatBool(useTLSForRedis),
	}
	conf, err := loadTemplateFile(path, params)
	if err != nil {
		log.Error(err, "unable to load redis readiness script")
		return ""
	}
	return conf
}

// GetSentinelLivenessScript will load the redis liveness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func GetSentinelLivenessScript(useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/sentinel_liveness.sh.tpl", getRedisConfigPath())
	params := map[string]string{
		"UseTLS": strconv.FormatBool(useTLSForRedis),
	}
	conf, err := loadTemplateFile(path, params)
	if err != nil {
		log.Error(err, "unable to load sentinel liveness script")
		return ""
	}
	return conf
}

// GetRedisServerAddress will return the Redis service address for the given ArgoCD.
func GetRedisServerAddress(cr *argoproj.ArgoCD) string {
	if cr.Spec.Redis.Remote != nil && *cr.Spec.Redis.Remote != "" {
		return *cr.Spec.Redis.Remote
	}

	// If principal is enabled, then Argo CD server/repo server should be configured to use redis proxy from principal (argo cd agent)
	if cr.Spec.ArgoCDAgent != nil && cr.Spec.ArgoCDAgent.Principal != nil && cr.Spec.ArgoCDAgent.Principal.IsEnabled() {
		return GenerateAgentPrincipalRedisProxyServiceName(cr.Name) + "." + cr.Namespace + ".svc.cluster.local:6379"
	}

	if cr.Spec.HA.Enabled {
		return getRedisHAProxyAddress(cr)
	}

	return FqdnServiceRef(common.ArgoCDDefaultRedisSuffix, common.ArgoCDDefaultRedisPort, cr)
}

// loadTemplateFile will parse a template with the given path and execute it with the given params.
func loadTemplateFile(path string, params map[string]string) (string, error) {
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		log.Error(err, "unable to parse template")
		return "", fmt.Errorf("unable to parse template. error: %w", err)
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, params)
	if err != nil {
		log.Error(err, "unable to execute template")
		return "", fmt.Errorf("unable to execute template. error: %w", err)
	}
	return buf.String(), nil
}

func GenerateAgentPrincipalRedisProxyServiceName(crName string) string {
	return fmt.Sprintf("%s-agent-%s", crName, "principal-redisproxy")
}
