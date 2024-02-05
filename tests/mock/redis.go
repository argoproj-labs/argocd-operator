package mock

import (
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Redis struct {
	Client    client.Client
	Logger    *util.Logger
	Name      string
	Namespace string
}

var (
	useTLS             = false
	redisServerAddress = ""
)

func NewRedis(name, namespace string, client client.Client) *Redis {
	return &Redis{
		Client:    client,
		Logger:    util.NewLogger("redis"),
		Name:      name,
		Namespace: namespace,
	}
}

func (r *Redis) SetUseTLS(val bool) {
	useTLS = val
}

func (r *Redis) SetServerAddress(val string) {
	redisServerAddress = val
}

func (r *Redis) UseTLS() bool {
	return useTLS
}

func (r *Redis) GetServerAddress() string {
	return redisServerAddress
}
