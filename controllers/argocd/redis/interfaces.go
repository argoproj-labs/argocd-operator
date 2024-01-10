package redis

type AppController interface {
	TriggerDeploymentRollout() error
}

type Server interface {
	TriggerDeploymentRollout() error
}

type RepoServer interface {
	TriggerDeploymentRollout() error
}
