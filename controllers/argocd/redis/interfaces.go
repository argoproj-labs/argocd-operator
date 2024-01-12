package redis

type AppController interface {
	TriggerRollout(string) error
}

type Server interface {
	TriggerRollout(string) error
}

type RepoServer interface {
	TriggerRollout(string) error
}
