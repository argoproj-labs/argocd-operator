package mock

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Server struct {
	Client    client.Client
	Logger    *util.Logger
	Name      string
	Namespace string
}

func NewServer(name, namespace string, client client.Client) *Server {
	return &Server{
		Client:    client,
		Logger:    util.NewLogger("server"),
		Name:      name,
		Namespace: namespace,
	}
}

func (s *Server) TriggerRollout(key string) error {
	return argocdcommon.TriggerDeploymentRollout(s.Name, s.Namespace, key, s.Client)
}
