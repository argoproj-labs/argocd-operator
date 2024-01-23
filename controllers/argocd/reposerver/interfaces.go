package reposerver

type AppController interface {
	TriggerRollout(string) error
}

type ServerController interface {
	TriggerRollout(string) error
}

type RedisController interface {
	UseTLS() bool
	GetServerAddress() string
}
