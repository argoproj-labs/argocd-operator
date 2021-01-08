package argocd

import (
	"sync"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
)

var (
	mutex sync.RWMutex
	hooks = []Hook{}
)

// Hook changes resources as they are created or updated by the
// reconciler.
type Hook func(*argoprojv1alpha1.ArgoCD, interface{}) error

// Register adds a modifier for updating resources during reconciliation.
func Register(h Hook) {
	mutex.Lock()
	defer mutex.Unlock()
	hooks = append(hooks, h)
}

func ApplyReconcilerHook(cr *argoprojv1alpha1.ArgoCD, r *rbacv1.ClusterRole) error {
	for _, v := range hooks {
		err := v(cr, r)
		if err != nil {
			return err
		}
	}
	return nil
}
