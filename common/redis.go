package common

// names
const (
	RedisController = "redis-controller"

	// RedisComponentName is the Redis control plane component
	RedisComponent = "redis"

	HAProxyName = "haproxy"

	// ArgoCDRedisHAConfigMapName is the upstream ArgoCD Redis HA ConfigMap name.
	ArgoCDRedisHAConfigMapName = "argocd-redis-ha-configmap"

	// ArgoCDRedisHAHealthConfigMapName is the upstream ArgoCD Redis HA Health ConfigMap name.
	ArgoCDRedisHAHealthConfigMapName = "argocd-redis-ha-health-configmap"

	// ArgoCDRedisProbesConfigMapName is the upstream ArgoCD Redis Probes ConfigMap name.
	ArgoCDRedisProbesConfigMapName = "argocd-redis-ha-probes"

	// ArgoCDRedisServerTLSSecretName is the name of the TLS secret for the redis-server
	ArgoCDRedisServerTLSSecretName = "argocd-operator-redis-tls"
)

// suffixes
const (
	//RedisSuffix is the default suffix to use for Redis resources.
	RedisSuffix = "redis"

	RedisHASuffix = "redis-ha"

	RedisHAProxySuffix = "redis-ha-haproxy"

	RedisHAServerSuffix = "redis-ha-server"

	RedisHAAnnouceSuffix = "redis-ha-announce"
)

// defaults
const (

	// DefaultRedisConfigPath is the default Redis configuration directory when not specified.
	DefaultRedisConfigPath = "/var/lib/redis"

	// DefaultRedisHAReplicas is the defaul number of replicas for Redis when rinning in HA mode.
	DefaultRedisHAReplicas = int32(3)

	// DefaultRedisHAProxyImage is the default Redis HAProxy image to use when not specified.
	DefaultRedisHAProxyImage = "haproxy"

	// DefaultRedisHAProxyVersion is the default Redis HAProxy image tag to use when not specified.
	DefaultRedisHAProxyVersion = "sha256:7392fbbbb53e9e063ca94891da6656e6062f9d021c0e514888a91535b9f73231" // 2.0.25-alpine

	// ArgoCDDefaultRedisImage is the Redis container image to use when not specified.
	DefaultRedisImage = "redis"

	// DefaultRedisPort is the default listen port for Redis.
	DefaultRedisPort = 6379

	// DefaultRedisSentinelPort is the default listen port for Redis sentinel.
	DefaultRedisSentinelPort = 26379

	// DefaultRedisVersion is the Redis container image tag to use when not specified.
	DefaultRedisVersion = "sha256:8061ca607db2a0c80010aeb5fc9bed0253448bc68711eaa14253a392f6c48280" // 6.2.4-alpine

	// DefaultRedisVersionHA is the Redis container image tag to use when not specified in HA mode.
	DefaultRedisVersionHA = "sha256:8061ca607db2a0c80010aeb5fc9bed0253448bc68711eaa14253a392f6c48280" // 6.2.4-alpine
)

// env vars
const (
	// RedisHAProxyImageEnvVar is the environment variable used to get the image
	// to used for the Redis HA Proxy container.
	RedisHAProxyImageEnvVar = "ARGOCD_REDIS_HA_PROXY_IMAGE"

	// RedisHAImageEnvVar is the environment variable used to get the image
	// to used for the the Redis container in HA mode.
	RedisHAImageEnvVar = "ARGOCD_REDIS_HA_IMAGE"

	// RedisImageEnvVar is the environment variable used to get the image
	// to used for the Redis container.
	RedisImageEnvVar = "ARGOCD_REDIS_IMAGE"

	// RedisConfigPathEnvVar is the environment variiable used to get the redis configuration templates
	RedisConfigPathEnvVar = "REDIS_CONFIG_PATH"
)

// keys
const (
	RedisTLSCertChangedKey = "redis.tls.cert.changed"
)

// commands
const (
	RedisCmd                      = "--redis"
	RedisUseTLSCmd                = "--redis-use-tls"
	RedisInsecureSkipTLSVerifyCmd = "--redis-insecure-skip-tls-verify"
	RedisCACertificate            = "--redis-ca-certificate"
)
