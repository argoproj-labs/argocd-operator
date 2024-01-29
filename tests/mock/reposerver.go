package mock

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reposerver struct {
	Client    client.Client
	Logger    *util.Logger
	Name      string
	Namespace string
}

func NewRepoServer(name, namespace string, client client.Client) *Reposerver {
	return &Reposerver{
		Client:    client,
		Logger:    util.NewLogger("repo-server"),
		Name:      name,
		Namespace: namespace,
	}
}

func (r *Reposerver) TriggerRollout(key string) error {
	return argocdcommon.TriggerDeploymentRollout(r.Name, r.Namespace, key, r.Client)
}

func (r *Reposerver) GetServerAddress() string {
	return "http://mock-server-address:8080"
}
