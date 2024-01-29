package mock

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Redis struct {
	Client    client.Client
	Logger    *util.Logger
	Name      string
	Namespace string
}

func NewRedis(name, namespace string, client client.Client) *Redis {
	return &Redis{
		Client:    client,
		Logger:    util.NewLogger("redis"),
		Name:      name,
		Namespace: namespace,
	}
}

func (r *Redis) TriggerRollout(key string) error {
	return argocdcommon.TriggerDeploymentRollout(r.Name, r.Namespace, key, r.Client)
}

func (r *Redis) UseTLS() bool {
	return true
}

func (r *Redis) GetServerAddress() string {
	return "http://mock-server-address:8080"
}
