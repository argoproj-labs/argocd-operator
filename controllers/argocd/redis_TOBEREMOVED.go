package argocd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"time"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcileArgoCD) redisShouldUseTLS(cr *argoproj.ArgoCD) bool {
	var tlsSecretObj corev1.Secret
	tlsSecretName := types.NamespacedName{Namespace: cr.Namespace, Name: common.ArgoCDRedisServerTLSSecretName}
	err := r.Client.Get(context.TODO(), tlsSecretName, &tlsSecretObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "error looking up redis tls secret")
		}
		return false
	}

	secretOwnerRefs := tlsSecretObj.GetOwnerReferences()
	if len(secretOwnerRefs) > 0 {
		// OpenShift service CA makes the owner reference for the TLS secret to the
		// service, which in turn is owned by the controller. This method performs
		// a lookup of the controller through the intermediate owning service.
		for _, secretOwner := range secretOwnerRefs {
			if isOwnerOfInterest(secretOwner) {
				key := client.ObjectKey{Name: secretOwner.Name, Namespace: tlsSecretObj.GetNamespace()}
				svc := &corev1.Service{}

				// Get the owning object of the secret
				err := r.Client.Get(context.TODO(), key, svc)
				if err != nil {
					log.Error(err, fmt.Sprintf("could not get owner of secret %s", tlsSecretObj.GetName()))
					return false
				}

				// If there's an object of kind ArgoCD in the owner's list,
				// this will be our reconciled object.
				serviceOwnerRefs := svc.GetOwnerReferences()
				for _, serviceOwner := range serviceOwnerRefs {
					if serviceOwner.Kind == "ArgoCD" {
						return true
					}
				}
			}
		}
	} else {
		// For secrets without owner (i.e. manually created), we apply some
		// heuristics. This may not be as accurate (e.g. if the user made a
		// typo in the resource's name), but should be good enough for now.
		if _, ok := tlsSecretObj.Annotations[common.AnnotationName]; ok {
			return true
		}
	}
	return false
}

// getRedisHAProxyAddress will return the Redis HA Proxy service address for the given ArgoCD.
func getRedisHAProxyAddress(cr *argoproj.ArgoCD) string {
	return fqdnServiceRef("redis-ha-haproxy", common.ArgoCDDefaultRedisPort, cr)
}

// getRedisServerAddress will return the Redis service address for the given ArgoCD.
func getRedisServerAddress(cr *argoproj.ArgoCD) string {
	if cr.Spec.Redis.Remote != nil && *cr.Spec.Redis.Remote != "" {
		return *cr.Spec.Redis.Remote
	}
	if cr.Spec.HA.Enabled {
		return getRedisHAProxyAddress(cr)
	}
	return fqdnServiceRef(common.ArgoCDDefaultRedisSuffix, common.ArgoCDDefaultRedisPort, cr)
}

// getRedisContainerImage will return the container image for the Redis server.
func getRedisContainerImage(cr *argoproj.ArgoCD) string {
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
	return argoutil.CombineImageTag(img, tag)
}

// getRedisHAContainerImage will return the container image for the Redis server in HA mode.
func getRedisHAContainerImage(cr *argoproj.ArgoCD) string {
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
	return argoutil.CombineImageTag(img, tag)
}

// getRedisHAProxyContainerImage will return the container image for the Redis HA Proxy.
func getRedisHAProxyContainerImage(cr *argoproj.ArgoCD) string {
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

	return argoutil.CombineImageTag(img, tag)
}

// getRedisConfigPath will return the path for the Redis configuration templates.
func getRedisConfigPath() string {
	path := os.Getenv("REDIS_CONFIG_PATH")
	if len(path) > 0 {
		return path
	}
	return common.ArgoCDDefaultRedisConfigPath
}

// getRedisHAProxySConfig will load the Redis HA Proxy configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisHAProxyConfig(cr *argoproj.ArgoCD, useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/haproxy.cfg.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
		"UseTLS":      strconv.FormatBool(useTLSForRedis),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy configuration")
		return ""
	}
	return script
}

// getRedisHAProxyScript will load the Redis HA Proxy init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisHAProxyScript(cr *argoproj.ArgoCD) string {
	path := fmt.Sprintf("%s/haproxy_init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis haproxy init script")
		return ""
	}
	return script
}

// getRedisResources will return the ResourceRequirements for the Redis container.
func getRedisResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.Redis.Resources != nil {
		resources = *cr.Spec.Redis.Resources
	}

	return resources
}

// getRedisHAResources will return the ResourceRequirements for the Redis HA.
func getRedisHAResources(cr *argoproj.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.HA.Resources != nil {
		resources = *cr.Spec.HA.Resources
	}

	return resources
}

// getRedisInitScript will load the redis configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisConf(useTLSForRedis bool) string {
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

// getRedisInitScript will load the redis init script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisInitScript(cr *argoproj.ArgoCD, useTLSForRedis bool) string {
	path := fmt.Sprintf("%s/init.sh.tpl", getRedisConfigPath())
	vars := map[string]string{
		"ServiceName": nameWithSuffix("redis-ha", cr),
		"UseTLS":      strconv.FormatBool(useTLSForRedis),
	}

	script, err := loadTemplateFile(path, vars)
	if err != nil {
		log.Error(err, "unable to load redis init-script")
		return ""
	}
	return script
}

// getRedisSentinelConf will load the redis sentinel configuration from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisSentinelConf(useTLSForRedis bool) string {
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

// getRedisLivenessScript will load the redis liveness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisLivenessScript(useTLSForRedis bool) string {
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

// getRedisReadinessScript will load the redis readiness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getRedisReadinessScript(useTLSForRedis bool) string {
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

// getSentinelLivenessScript will load the redis liveness script from a template on disk for the given ArgoCD.
// If an error occurs, an empty string value will be returned.
func getSentinelLivenessScript(useTLSForRedis bool) string {
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

func isRedisTLSVerificationDisabled(cr *argoproj.ArgoCD) bool {
	return cr.Spec.Redis.DisableTLSVerification
}

// reconcileRedisHAConfigMap will ensure that the Redis HA ConfigMap is present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisHAConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if !cr.Spec.HA.Enabled {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			return r.Client.Delete(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	cm.Data = map[string]string{
		"haproxy.cfg":     getRedisHAProxyConfig(cr, useTLSForRedis),
		"haproxy_init.sh": getRedisHAProxyScript(cr),
		"init.sh":         getRedisInitScript(cr, useTLSForRedis),
		"redis.conf":      getRedisConf(useTLSForRedis),
		"sentinel.conf":   getRedisSentinelConf(useTLSForRedis),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRedisHAConfigMap will ensure that the Redis HA Health ConfigMap is present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisHAHealthConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAHealthConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if !cr.Spec.HA.Enabled {
			// ConfigMap exists but HA enabled flag has been set to false, delete the ConfigMap
			return r.Client.Delete(context.TODO(), cm)
		}
		return nil // ConfigMap found with nothing changed, move along...
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	cm.Data = map[string]string{
		"redis_liveness.sh":    getRedisLivenessScript(useTLSForRedis),
		"redis_readiness.sh":   getRedisReadinessScript(useTLSForRedis),
		"sentinel_liveness.sh": getSentinelLivenessScript(useTLSForRedis),
	}

	if err := controllerutil.SetControllerReference(cr, cm, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), cm)
}

// reconcileRedisConfiguration will ensure that all of the Redis ConfigMaps are present for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileRedisConfiguration(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	if err := r.reconcileRedisHAConfigMap(cr, useTLSForRedis); err != nil {
		return err
	}
	if err := r.reconcileRedisHAHealthConfigMap(cr, useTLSForRedis); err != nil {
		return err
	}
	return nil
}

func getArgoRedisArgs(useTLS bool) []string {
	args := make([]string, 0)

	args = append(args, "--save", "")
	args = append(args, "--appendonly", "no")

	if useTLS {
		args = append(args, "--tls-port", "6379")
		args = append(args, "--port", "0")

		args = append(args, "--tls-cert-file", "/app/config/redis/tls/tls.crt")
		args = append(args, "--tls-key-file", "/app/config/redis/tls/tls.key")
		args = append(args, "--tls-auth-clients", "no")
	}

	return args
}

// reconcileRedisHAAnnounceServices will ensure that the announce Services are present for Redis when running in HA mode.
func (r *ReconcileArgoCD) reconcileRedisHAAnnounceServices(cr *argoproj.ArgoCD) error {
	for i := int32(0); i < common.ArgoCDDefaultRedisHAReplicas; i++ {
		svc := newServiceWithSuffix(fmt.Sprintf("redis-ha-announce-%d", i), "redis", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
				return r.Client.Delete(context.TODO(), svc)
			}
			return nil // Service found, do nothing
		}

		if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
			return nil //return as Ha is not enabled do nothing
		}

		svc.ObjectMeta.Annotations = map[string]string{
			common.ArgoCDKeyTolerateUnreadyEndpounts: "true",
		}

		svc.Spec.PublishNotReadyAddresses = true

		svc.Spec.Selector = map[string]string{
			common.ArgoCDKeyName:               nameWithSuffix("redis-ha", cr),
			common.ArgoCDKeyStatefulSetPodName: nameWithSuffix(fmt.Sprintf("redis-ha-server-%d", i), cr),
		}

		svc.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "server",
				Port:       common.ArgoCDDefaultRedisPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("redis"),
			}, {
				Name:       "sentinel",
				Port:       common.ArgoCDDefaultRedisSentinelPort,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromString("sentinel"),
			},
		}

		if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
			return err
		}

		if err := r.Client.Create(context.TODO(), svc); err != nil {
			return err
		}
	}
	return nil
}

// reconcileRedisHAMasterService will ensure that the "master" Service is present for Redis when running in HA mode.
func (r *ReconcileArgoCD) reconcileRedisHAMasterService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("redis-ha", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
		return nil //return as Ha is not enabled do nothing
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("redis-ha", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "server",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		}, {
			Name:       "sentinel",
			Port:       common.ArgoCDDefaultRedisSentinelPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("sentinel"),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAProxyService will ensure that the HA Proxy Service is present for Redis when running in HA mode.
func (r *ReconcileArgoCD) reconcileRedisHAProxyService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("redis-ha-haproxy", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {

		if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
			return r.Client.Delete(context.TODO(), svc)
		}

		if ensureAutoTLSAnnotation(svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS()) {
			return r.Client.Update(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if !cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
		return nil //return as Ha is not enabled do nothing
	}

	ensureAutoTLSAnnotation(svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS())

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("redis-ha-haproxy", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "haproxy",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromString("redis"),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisHAServices will ensure that all required Services are present for Redis when running in HA mode.
func (r *ReconcileArgoCD) reconcileRedisHAServices(cr *argoproj.ArgoCD) error {

	if err := r.reconcileRedisHAAnnounceServices(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisHAMasterService(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisHAProxyService(cr); err != nil {
		return err
	}
	return nil
}

// reconcileRedisService will ensure that the Service for Redis is present.
func (r *ReconcileArgoCD) reconcileRedisService(cr *argoproj.ArgoCD) error {
	svc := newServiceWithSuffix("redis", "redis", cr)

	if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
		if !cr.Spec.Redis.IsEnabled() {
			return r.Client.Delete(context.TODO(), svc)
		}
		if ensureAutoTLSAnnotation(svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS()) {
			return r.Client.Update(context.TODO(), svc)
		}
		if cr.Spec.HA.Enabled {
			return r.Client.Delete(context.TODO(), svc)
		}
		return nil // Service found, do nothing
	}

	if cr.Spec.HA.Enabled || !cr.Spec.Redis.IsEnabled() {
		return nil //return as Ha is enabled do nothing
	}

	ensureAutoTLSAnnotation(svc, common.ArgoCDRedisServerTLSSecretName, cr.Spec.Redis.WantsAutoTLS())

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("redis", cr),
	}

	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "tcp-redis",
			Port:       common.ArgoCDDefaultRedisPort,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(common.ArgoCDDefaultRedisPort),
		},
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

// reconcileRedisTLSSecret checks whether the argocd-operator-redis-tls secret
// has changed since our last reconciliation loop. It does so by comparing the
// checksum of tls.crt and tls.key in the status of the ArgoCD CR against the
// values calculated from the live state in the cluster.
func (r *ReconcileArgoCD) reconcileRedisTLSSecret(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	var tlsSecretObj corev1.Secret
	var sha256sum string

	log.Info("reconciling redis-server TLS secret")

	tlsSecretName := types.NamespacedName{Namespace: cr.Namespace, Name: common.ArgoCDRedisServerTLSSecretName}
	err := r.Client.Get(context.TODO(), tlsSecretName, &tlsSecretObj)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else if tlsSecretObj.Type != corev1.SecretTypeTLS {
		// We only process secrets of type kubernetes.io/tls
		return nil
	} else {
		// We do the checksum over a concatenated byte stream of cert + key
		crt, crtOk := tlsSecretObj.Data[corev1.TLSCertKey]
		key, keyOk := tlsSecretObj.Data[corev1.TLSPrivateKeyKey]
		if crtOk && keyOk {
			var sumBytes []byte
			sumBytes = append(sumBytes, crt...)
			sumBytes = append(sumBytes, key...)
			sha256sum = fmt.Sprintf("%x", sha256.Sum256(sumBytes))
		}
	}

	// The content of the TLS secret has changed since we last looked if the
	// calculated checksum doesn't match the one stored in the status.
	if cr.Status.RedisTLSChecksum != sha256sum {
		// We store the value early to prevent a possible restart loop, for the
		// cost of a possibly missed restart when we cannot update the status
		// field of the resource.
		cr.Status.RedisTLSChecksum = sha256sum
		err = r.Client.Status().Update(context.TODO(), cr)
		if err != nil {
			return err
		}

		// Trigger rollout of redis
		if cr.Spec.HA.Enabled {
			err = r.recreateRedisHAConfigMap(cr, useTLSForRedis)
			if err != nil {
				return err
			}
			err = r.recreateRedisHAHealthConfigMap(cr, useTLSForRedis)
			if err != nil {
				return err
			}
			haProxyDepl := newDeploymentWithSuffix("redis-ha-haproxy", "redis", cr)
			err = r.triggerRollout(haProxyDepl, "redis.tls.cert.changed")
			if err != nil {
				return err
			}
			// If we use triggerRollout on the redis stateful set, kubernetes will attempt to restart the  pods
			// one at a time, and the first one to restart (which will be using tls) will hang as it tries to
			// communicate with the existing pods (which are not using tls) to establish which is the master.
			// So instead we delete the stateful set, which will delete all the pods.
			redisSts := newStatefulSetWithSuffix("redis-ha-server", "redis", cr)
			if argoutil.IsObjectFound(r.Client, redisSts.Namespace, redisSts.Name, redisSts) {
				err = r.Client.Delete(context.TODO(), redisSts)
				if err != nil {
					return err
				}
			}
		} else {
			redisDepl := newDeploymentWithSuffix("redis", "redis", cr)
			err = r.triggerRollout(redisDepl, "redis.tls.cert.changed")
			if err != nil {
				return err
			}
		}

		// Trigger rollout of API server
		apiDepl := newDeploymentWithSuffix("server", "server", cr)
		err = r.triggerRollout(apiDepl, "redis.tls.cert.changed")
		if err != nil {
			return err
		}

		// Trigger rollout of repository server
		repoDepl := newDeploymentWithSuffix("repo-server", "repo-server", cr)
		err = r.triggerRollout(repoDepl, "redis.tls.cert.changed")
		if err != nil {
			return err
		}

		// Trigger rollout of application controller
		controllerSts := newStatefulSetWithSuffix("application-controller", "application-controller", cr)
		err = r.triggerRollout(controllerSts, "redis.tls.cert.changed")
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *ReconcileArgoCD) recreateRedisHAConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.Client.Delete(context.TODO(), cm); err != nil {
			return err
		}
	}
	return r.reconcileRedisHAConfigMap(cr, useTLSForRedis)
}

func (r *ReconcileArgoCD) recreateRedisHAHealthConfigMap(cr *argoproj.ArgoCD, useTLSForRedis bool) error {
	cm := newConfigMapWithName(common.ArgoCDRedisHAHealthConfigMapName, cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, cm.Name, cm) {
		if err := r.Client.Delete(context.TODO(), cm); err != nil {
			return err
		}
	}
	return r.reconcileRedisHAHealthConfigMap(cr, useTLSForRedis)
}

// reconcileStatusRedis will ensure that the Redis status is updated for the given ArgoCD.
func (r *ReconcileArgoCD) reconcileStatusRedis(cr *argoproj.ArgoCD) error {
	status := "Unknown"

	if !cr.Spec.HA.Enabled {
		deploy := newDeploymentWithSuffix("redis", "redis", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, deploy.Name, deploy) {
			status = "Pending"

			if deploy.Spec.Replicas != nil {
				if deploy.Status.ReadyReplicas == *deploy.Spec.Replicas {
					status = "Running"
				}
			}
		}
	} else {
		ss := newStatefulSetWithSuffix("redis-ha-server", "redis-ha-server", cr)
		if argoutil.IsObjectFound(r.Client, cr.Namespace, ss.Name, ss) {
			status = "Pending"

			if ss.Status.ReadyReplicas == *ss.Spec.Replicas {
				status = "Running"
			}
		}
		// TODO: Add check for HA proxy deployment here as well?
	}

	if cr.Status.Redis != status {
		cr.Status.Redis = status
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}

func policyRuleForRedis(client client.Client) []v1.PolicyRule {
	rules := []v1.PolicyRule{}

	// Need additional policy rules if we are running on openshift, else the stateful set won't have the right
	// permissions to start
	rules = appendOpenShiftNonRootSCC(rules, client)

	return rules
}

func policyRuleForRedisHa(client client.Client) []v1.PolicyRule {

	rules := []v1.PolicyRule{
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"endpoints",
			},
			Verbs: []string{
				"get",
			},
		},
	}

	// Need additional policy rules if we are running on openshift, else the stateful set won't have the right
	// permissions to start
	rules = appendOpenShiftNonRootSCC(rules, client)

	return rules
}

func getRedisHAReplicas(cr *argoproj.ArgoCD) *int32 {
	replicas := common.ArgoCDDefaultRedisHAReplicas
	// TODO: Allow override of this value through CR?
	return &replicas
}

func (r *ReconcileArgoCD) reconcileRedisStatefulSet(cr *argoproj.ArgoCD) error {
	ss := newStatefulSetWithSuffix("redis-ha-server", "redis", cr)

	ss.Spec.PodManagementPolicy = appsv1.OrderedReadyPodManagement
	ss.Spec.Replicas = getRedisHAReplicas(cr)
	ss.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			common.ArgoCDKeyName: nameWithSuffix("redis-ha", cr),
		},
	}

	ss.Spec.ServiceName = nameWithSuffix("redis-ha", cr)

	ss.Spec.Template.ObjectMeta = metav1.ObjectMeta{
		Annotations: map[string]string{
			"checksum/init-config": "7128bfbb51eafaffe3c33b1b463e15f0cf6514cec570f9d9c4f2396f28c724ac", // TODO: Should this be hard-coded?
		},
		Labels: map[string]string{
			common.ArgoCDKeyName: nameWithSuffix("redis-ha", cr),
		},
	}

	ss.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						common.ArgoCDKeyName: nameWithSuffix("redis-ha", cr),
					},
				},
				TopologyKey: common.ArgoCDKeyHostname,
			}},
		},
	}

	f := false
	ss.Spec.Template.Spec.AutomountServiceAccountToken = &f

	ss.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Args: []string{
				"/data/conf/redis.conf",
			},
			Command: []string{
				"redis-server",
			},
			Image:           getRedisHAContainerImage(cr),
			ImagePullPolicy: corev1.PullIfNotPresent,
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"sh",
							"-c",
							"/health/redis_liveness.sh",
						},
					},
				},
				FailureThreshold:    int32(5),
				InitialDelaySeconds: int32(30),
				PeriodSeconds:       int32(15),
				SuccessThreshold:    int32(1),
				TimeoutSeconds:      int32(15),
			},
			Name: "redis",
			Ports: []corev1.ContainerPort{{
				ContainerPort: common.ArgoCDDefaultRedisPort,
				Name:          "redis",
			}},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"sh",
							"-c",
							"/health/redis_readiness.sh",
						},
					},
				},
				FailureThreshold:    int32(5),
				InitialDelaySeconds: int32(30),
				PeriodSeconds:       int32(15),
				SuccessThreshold:    int32(1),
				TimeoutSeconds:      int32(15),
			},
			Resources: getRedisHAResources(cr),
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: boolPtr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						"ALL",
					},
				},
				RunAsNonRoot: boolPtr(true),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/data",
					Name:      "data",
				},
				{
					MountPath: "/health",
					Name:      "health",
				},
				{
					Name:      common.ArgoCDRedisServerTLSSecretName,
					MountPath: "/app/config/redis/tls",
				},
			},
		},
		{
			Args: []string{
				"/data/conf/sentinel.conf",
			},
			Command: []string{
				"redis-sentinel",
			},
			Image:           getRedisHAContainerImage(cr),
			ImagePullPolicy: corev1.PullIfNotPresent,
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"sh",
							"-c",
							"/health/sentinel_liveness.sh",
						},
					},
				},
				FailureThreshold:    int32(5),
				InitialDelaySeconds: int32(30),
				PeriodSeconds:       int32(15),
				SuccessThreshold:    int32(1),
				TimeoutSeconds:      int32(15),
			},
			Name: "sentinel",
			Ports: []corev1.ContainerPort{{
				ContainerPort: common.ArgoCDDefaultRedisSentinelPort,
				Name:          "sentinel",
			}},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{
							"sh",
							"-c",
							"/health/sentinel_liveness.sh",
						},
					},
				},
				FailureThreshold:    int32(5),
				InitialDelaySeconds: int32(30),
				PeriodSeconds:       int32(15),
				SuccessThreshold:    int32(1),
				TimeoutSeconds:      int32(15),
			},
			Resources: getRedisHAResources(cr),
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: boolPtr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{
						"ALL",
					},
				},
				RunAsNonRoot: boolPtr(true),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					MountPath: "/data",
					Name:      "data",
				},
				{
					MountPath: "/health",
					Name:      "health",
				},
				{
					Name:      common.ArgoCDRedisServerTLSSecretName,
					MountPath: "/app/config/redis/tls",
				},
			},
		},
	}

	ss.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Args: []string{
			"/readonly-config/init.sh",
		},
		Command: []string{
			"sh",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "SENTINEL_ID_0",
				Value: "3c0d9c0320bb34888c2df5757c718ce6ca992ce6", // TODO: Should this be hard-coded?
			},
			{
				Name:  "SENTINEL_ID_1",
				Value: "40000915ab58c3fa8fd888fb8b24711944e6cbb4", // TODO: Should this be hard-coded?
			},
			{
				Name:  "SENTINEL_ID_2",
				Value: "2bbec7894d954a8af3bb54d13eaec53cb024e2ca", // TODO: Should this be hard-coded?
			},
		},
		Image:           getRedisHAContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		Resources:       getRedisHAResources(cr),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/readonly-config",
				Name:      "config",
				ReadOnly:  true,
			},
			{
				MountPath: "/data",
				Name:      "data",
			},
			{
				Name:      common.ArgoCDRedisServerTLSSecretName,
				MountPath: "/app/config/redis/tls",
			},
		},
	}}

	var fsGroup int64 = 1000
	var runAsNonRoot bool = true
	var runAsUser int64 = 1000

	ss.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		FSGroup:      &fsGroup,
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
	}
	AddSeccompProfileForOpenShift(r.Client, &ss.Spec.Template.Spec)

	ss.Spec.Template.Spec.ServiceAccountName = nameWithSuffix("argocd-redis-ha", cr)

	var terminationGracePeriodSeconds int64 = 60
	ss.Spec.Template.Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds

	var defaultMode int32 = 493
	ss.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDRedisHAConfigMapName,
					},
				},
			},
		},
		{
			Name: "health",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &defaultMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDRedisHAHealthConfigMapName,
					},
				},
			},
		},
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
	}

	ss.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
	}

	if err := applyReconcilerHook(cr, ss, ""); err != nil {
		return err
	}

	existing := newStatefulSetWithSuffix("redis-ha-server", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if !(cr.Spec.HA.Enabled && cr.Spec.Redis.IsEnabled()) {
			// StatefulSet exists but either HA or component enabled flag has been set to false, delete the StatefulSet
			return r.Client.Delete(context.TODO(), existing)
		}

		desiredImage := getRedisHAContainerImage(cr)
		changed := false
		updateNodePlacementStateful(existing, ss, &changed)
		for i, container := range existing.Spec.Template.Spec.Containers {
			if container.Image != desiredImage {
				existing.Spec.Template.Spec.Containers[i].Image = getRedisHAContainerImage(cr)
				existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
				changed = true
			}

			if !reflect.DeepEqual(ss.Spec.Template.Spec.Containers[i].Resources, existing.Spec.Template.Spec.Containers[i].Resources) {
				existing.Spec.Template.Spec.Containers[i].Resources = ss.Spec.Template.Spec.Containers[i].Resources
				changed = true
			}
		}

		if !reflect.DeepEqual(ss.Spec.Template.Spec.InitContainers[0].Resources, existing.Spec.Template.Spec.InitContainers[0].Resources) {
			existing.Spec.Template.Spec.InitContainers[0].Resources = ss.Spec.Template.Spec.InitContainers[0].Resources
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}

		return nil // StatefulSet found, do nothing
	}

	if !cr.Spec.Redis.IsEnabled() {
		log.Info("Redis disabled. Skipping starting Redis.") // Redis not enabled, do nothing.
		return nil
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	if err := controllerutil.SetControllerReference(cr, ss, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), ss)
}

// reconcileRedisDeployment will ensure the Deployment resource is present for the ArgoCD Redis component.
func (r *ReconcileArgoCD) reconcileRedisDeployment(cr *argoproj.ArgoCD, useTLS bool) error {
	deploy := newDeploymentWithSuffix("redis", "redis", cr)

	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Args:            getArgoRedisArgs(useTLS),
		Image:           getRedisContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "redis",
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRedisPort,
			},
		},
		Resources: getRedisResources(cr),
		Env:       proxyEnvVars(),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
			RunAsUser:    int64Ptr(999),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      common.ArgoCDRedisServerTLSSecretName,
				MountPath: "/app/config/redis/tls",
			},
		},
	}}

	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-redis")
	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
	}

	if err := applyReconcilerHook(cr, deploy, ""); err != nil {
		return err
	}

	existing := newDeploymentWithSuffix("redis", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if !cr.Spec.Redis.IsEnabled() {
			// Deployment exists but component enabled flag has been set to false, delete the Deployment
			log.Info("Redis exists but should be disabled. Deleting existing redis.")
			return r.Client.Delete(context.TODO(), deploy)
		}
		if cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to true, delete the Deployment
			return r.Client.Delete(context.TODO(), deploy)
		}
		changed := false
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRedisContainerImage(cr)
		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed)

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Args, existing.Spec.Template.Spec.Containers[0].Args) {
			existing.Spec.Template.Spec.Containers[0].Args = deploy.Spec.Template.Spec.Containers[0].Args
			changed = true
		}

		if !reflect.DeepEqual(existing.Spec.Template.Spec.Containers[0].Env,
			deploy.Spec.Template.Spec.Containers[0].Env) {
			existing.Spec.Template.Spec.Containers[0].Env = deploy.Spec.Template.Spec.Containers[0].Env
			changed = true
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found with nothing to do, move along...
	}

	if !cr.Spec.Redis.IsEnabled() {
		log.Info("Redis disabled. Skipping starting redis.")
		return nil
	}

	if cr.Spec.HA.Enabled {
		return nil // HA enabled, do nothing.
	}
	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}

// reconcileRedisHAProxyDeployment will ensure the Deployment resource is present for the Redis HA Proxy component.
func (r *ReconcileArgoCD) reconcileRedisHAProxyDeployment(cr *argoproj.ArgoCD) error {
	deploy := newDeploymentWithSuffix("redis-ha-haproxy", "redis", cr)

	deploy.Spec.Template.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								common.ArgoCDKeyName: nameWithSuffix("redis-ha-haproxy", cr),
							},
						},
						TopologyKey: common.ArgoCDKeyFailureDomainZone,
					},
					Weight: int32(100),
				},
			},
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.ArgoCDKeyName: nameWithSuffix("redis-ha-haproxy", cr),
						},
					},
					TopologyKey: common.ArgoCDKeyHostname,
				},
			},
		},
	}

	deploy.Spec.Template.Spec.Containers = []corev1.Container{{
		Image:           getRedisHAProxyContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "haproxy",
		Env:             proxyEnvVars(),
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8888),
				},
			},
			InitialDelaySeconds: int32(5),
			PeriodSeconds:       int32(3),
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: common.ArgoCDDefaultRedisPort,
				Name:          "redis",
			},
		},
		Resources: getRedisHAResources(cr),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			RunAsNonRoot: boolPtr(true),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data",
				MountPath: "/usr/local/etc/haproxy",
			},
			{
				Name:      "shared-socket",
				MountPath: "/run/haproxy",
			},
			{
				Name:      common.ArgoCDRedisServerTLSSecretName,
				MountPath: "/app/config/redis/tls",
			},
		},
	}}

	deploy.Spec.Template.Spec.InitContainers = []corev1.Container{{
		Args: []string{
			"/readonly/haproxy_init.sh",
		},
		Command: []string{
			"sh",
		},
		Image:           getRedisHAProxyContainerImage(cr),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Name:            "config-init",
		Env:             proxyEnvVars(),
		Resources:       getRedisHAResources(cr),
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "config-volume",
				MountPath: "/readonly",
				ReadOnly:  true,
			},
			{
				Name:      "data",
				MountPath: "/data",
			},
		},
	}}

	deploy.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: common.ArgoCDRedisHAConfigMapName,
					},
				},
			},
		},
		{
			Name: "shared-socket",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: common.ArgoCDRedisServerTLSSecretName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: common.ArgoCDRedisServerTLSSecretName,
					Optional:   boolPtr(true),
				},
			},
		},
	}

	deploy.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
		RunAsUser:    int64Ptr(1000),
		FSGroup:      int64Ptr(1000),
	}
	AddSeccompProfileForOpenShift(r.Client, &deploy.Spec.Template.Spec)

	deploy.Spec.Template.Spec.ServiceAccountName = fmt.Sprintf("%s-%s", cr.Name, "argocd-redis-ha")

	version, err := getClusterVersion(r.Client)
	if err != nil {
		log.Error(err, "error getting cluster version")
	}
	if err := applyReconcilerHook(cr, deploy, version); err != nil {
		return err
	}

	existing := newDeploymentWithSuffix("redis-ha-haproxy", "redis", cr)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {
		if !cr.Spec.HA.Enabled {
			// Deployment exists but HA enabled flag has been set to false, delete the Deployment
			return r.Client.Delete(context.TODO(), existing)
		}
		changed := false
		actualImage := existing.Spec.Template.Spec.Containers[0].Image
		desiredImage := getRedisHAProxyContainerImage(cr)

		if actualImage != desiredImage {
			existing.Spec.Template.Spec.Containers[0].Image = desiredImage
			existing.Spec.Template.ObjectMeta.Labels["image.upgraded"] = time.Now().UTC().Format("01022006-150406-MST")
			changed = true
		}
		updateNodePlacement(existing, deploy, &changed)

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.Containers[0].Resources, existing.Spec.Template.Spec.Containers[0].Resources) {
			existing.Spec.Template.Spec.Containers[0].Resources = deploy.Spec.Template.Spec.Containers[0].Resources
			changed = true
		}

		if !reflect.DeepEqual(deploy.Spec.Template.Spec.InitContainers[0].Resources, existing.Spec.Template.Spec.InitContainers[0].Resources) {
			existing.Spec.Template.Spec.InitContainers[0].Resources = deploy.Spec.Template.Spec.InitContainers[0].Resources
			changed = true
		}

		if changed {
			return r.Client.Update(context.TODO(), existing)
		}
		return nil // Deployment found, do nothing
	}

	if !cr.Spec.HA.Enabled {
		return nil // HA not enabled, do nothing.
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)
}
