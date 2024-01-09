package redis

// keys
const (
	haproxyCfgKey             = "haproxy.cfg"
	haproxyScriptKey          = "haproxy_init.sh"
	initScriptKey             = "init.sh"
	redisConfKey              = "redis.conf"
	sentinelConfKey           = "sentinel.Conf"
	UseTLSKey                 = "UseTLS"
	ServiceNameKey            = "ServiceName"
	livenessScriptKey         = "redis_liveness.sh"
	readinessScriptKey        = "redis_readiness.sh"
	sentinelLivenessScriptKey = "sentinel_liveness.sh"
)

// filenames
const (
	haproxyCfgTpl         = "haproxy.cfg.tpl"
	haproxyInitTpl        = "haproxy_init.sh.tpl"
	initShTpl             = "init.sh.tpl"
	redisConfTpl          = "redis.conf.tpl"
	sentinelConfTpl       = "sentinel.conf.tpl"
	livenessShTpl         = "redis_liveness.sh.tpl"
	readinessShTpl        = "redis_readiness.sh.tpl"
	sentinelLivenessShTpl = "sentinel_liveness.sh.tpl"
)
