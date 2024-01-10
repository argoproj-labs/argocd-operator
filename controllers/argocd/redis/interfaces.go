package redis

type AppController interface {
	TriggerRollout() error
}

type Server interface {
	TriggerRollout() error
}

type RepoServer interface {
	TriggerRollout() error
}
