package mutation

import (
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

var (
	mutex       sync.RWMutex
	mutateFuncs = []MutateFunc{}
)

// MutateFunc defines the function signature for any mutation functions that need to be executed by this package
type MutateFunc func(*argoproj.ArgoCD, interface{}, client.Client, ...interface{}) error

// Register adds a modifier for updating resources during reconciliation.
func Register(m ...MutateFunc) {
	mutex.Lock()
	defer mutex.Unlock()
	mutateFuncs = append(mutateFuncs, m...)
}

func ApplyReconcilerMutation(cr *argoproj.ArgoCD, resource interface{}, client client.Client, args ...interface{}) error {
	mutex.Lock()
	defer mutex.Unlock()
	for _, mutateFunc := range mutateFuncs {
		if err := mutateFunc(cr, resource, client, args...); err != nil {
			return err
		}
	}
	return nil
}
