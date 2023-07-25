package mutation

import (
	"sync"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	mutex       sync.RWMutex
	mutateFuncs = []MutateFunc{}
)

// MutateFunc defines the function signature for any mutation functions that need to be executed by this package
type MutateFunc func(cr *v1alpha1.ArgoCD, resource interface{}, client *ctrlClient.Client) error

// Register adds a modifier for updating resources during reconciliation.
func Register(m ...MutateFunc) {
	mutex.Lock()
	defer mutex.Unlock()
	mutateFuncs = append(mutateFuncs, m...)
}

func ApplyReconcilerMutation(cr *v1alpha1.ArgoCD, resource interface{}, client *ctrlClient.Client) error {
	mutex.Lock()
	defer mutex.Unlock()
	for _, mutateFunc := range mutateFuncs {
		if err := mutateFunc(cr, resource, client); err != nil {
			return err
		}
	}
	return nil
}
