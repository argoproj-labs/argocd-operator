package redis

type AppController interface {
	TriggerRollout(string) error
}

type ServerController interface {
	TriggerRollout(string) error
}

type RepoServerController interface {
	TriggerRollout(string) error
}
