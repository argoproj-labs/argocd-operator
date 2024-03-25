package appcontroller

type RedisController interface {
	UseTLS() bool
	GetServerAddress() string
}

type RepoServerController interface {
	GetServerAddress() string
	TLSVerificationRequested() bool
}
