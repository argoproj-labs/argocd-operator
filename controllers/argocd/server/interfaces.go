package server

type AppController interface {
	TriggerRollout(string) error
}

type RedisController interface {
	UseTLS() bool
	GetServerAddress() string
	TLSVerificationDisabled() bool
}

type RepoServerController interface {
	UseTLS() bool
	GetServerAddress() string
}

// TODO: use sso pkg?
type DexController interface {
	GetServerAddress() string
}
