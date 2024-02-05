package mock

import (
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Appcontroller struct {
	Client    client.Client
	Logger    *util.Logger
	Name      string
	Namespace string
}

func NewAppController(name, namespace string, client client.Client) *Appcontroller {
	return &Appcontroller{
		Client:    client,
		Logger:    util.NewLogger("app-controller"),
		Name:      name,
		Namespace: namespace,
	}
}

func (ac *Appcontroller) TriggerRollout(key string) error {
	return argocdcommon.TriggerStatefulSetRollout(ac.Name, ac.Namespace, key, ac.Client)
}
