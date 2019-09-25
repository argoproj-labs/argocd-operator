package argocd

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ReconcileArgoCD) isObjectFound(nsname types.NamespacedName, obj runtime.Object) bool {
	err := r.client.Get(context.TODO(), nsname, obj)
	if err != nil {
		return false
	}
	return true
}
