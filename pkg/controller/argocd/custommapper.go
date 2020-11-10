package argocd

import (
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/client"
)



func (r *ReconcileArgoCD) clusterRoleBindingMapper(o handler.MapObject)[]reconcile.Request {
	crbAnnotations := o.Meta.GetAnnotations()
	namespacedArgoCDObject := client.ObjectKey{}

	for k,v := range crbAnnotations {
		if k == "argocds.argoproj.io/name" {
			namespacedArgoCDObject.Name = v
		}else if k == "argocds.argoproj.io/namespace"{
			namespacedArgoCDObject.Namespace = v
		}
	}

	
	result := []reconcile.Request{
		reconcile.Request{namespacedArgoCDObject},
	}
	return result
}