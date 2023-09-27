package argocd

import (
	"sync"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

var (
	mutex sync.RWMutex
	hooks = []Hook{}
)

// Hook changes resources as they are created or updated by the reconciler.
type Hook func(*argoproj.ArgoCD, interface{}, string) error

// Register adds a modifier for updating resources during reconciliation.
func Register(h ...Hook) {
	mutex.Lock()
	defer mutex.Unlock()
	hooks = append(hooks, h...)
}

// nolint:unparam
func applyReconcilerHook(cr *argoproj.ArgoCD, i interface{}, hint string) error {
	mutex.Lock()
	defer mutex.Unlock()
	for _, v := range hooks {
		if err := v(cr, i, hint); err != nil {
			return err
		}
	}
	return nil
}
