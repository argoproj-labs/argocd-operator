package argocd

import (
	"sync"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	rbacv1 "k8s.io/api/rbac/v1"
)

var (
	modifiersMu sync.RWMutex
	modifiers   = []Modifier{}
)

// Modifier can change resources as they are created or updated by the
// reconciler.
type Modifier interface {
	Role(*argoprojv1alpha1.ArgoCD, *rbacv1.Role) error
}

// Register adds a modifier for updating resources during reconciliation.
func Register(m Modifier) {
	modifiersMu.Lock()
	defer modifiersMu.Unlock()
	modifiers = append(modifiers, m)
}

func applyRoleModifiers(cr *argoprojv1alpha1.ArgoCD, r *rbacv1.Role) error {
	for _, v := range modifiers {
		if err := v.Role(cr, r); err != nil {
			return err
		}
	}
	return nil
}
