package server

import (
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

type AppController interface {
	TriggerRollout(string) error
}

type RedisController interface {
	UseTLS() bool
	GetServerAddress() string
	TLSVerificationDisabled() bool
}

type RepoServerController interface {
	UseTLS() bool
	GetServerAddress() string
}

type DexController interface {
	GetServerAddress() string
}

type SSOController interface {
	GetProvider(*argoproj.ArgoCD) argoproj.SSOProviderType
	DexController
}
