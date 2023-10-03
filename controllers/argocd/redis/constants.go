package redis

const (
	// Values
	RedisControllerComponent = "redis"
	RedisHAProxyServiceName  = "redis-ha-haproxy"

	// Commands
	Redis                      = "--redis"
	RedisUseTLS                = "--redis-use-tls"
	RedisInsecureSkipTLSVerify = "--redis-insecure-skip-tls-verify"
	RedisCACertificate         = "--redis-ca-certificate"
)
