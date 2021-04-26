package argocd

import (
	"context"
	"fmt"
	"strings"

	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
)

func (r *ReconcileArgoCD) clusterResourceMapper(o handler.MapObject) []reconcile.Request {
	crbAnnotations := o.Meta.GetAnnotations()
	namespacedArgoCDObject := client.ObjectKey{}

	for k, v := range crbAnnotations {
		if k == common.AnnotationName {
			namespacedArgoCDObject.Name = v
		} else if k == common.AnnotationNamespace {
			namespacedArgoCDObject.Namespace = v
		}
	}

	var result = []reconcile.Request{}
	if namespacedArgoCDObject.Name != "" && namespacedArgoCDObject.Namespace != "" {
		result = []reconcile.Request{
			{NamespacedName: namespacedArgoCDObject},
		}
	}
	return result
}

// OpenShift service CA makes the owner reference for the TLS secret to the
// service, which in turn is owned by the controller. This method performs
// a lookup of the controller through the intermediate owning service.
// This will currently only work on OpenShift.
func (r *ReconcileArgoCD) tlsSecretMapper(o handler.MapObject) []reconcile.Request {
	var result = []reconcile.Request{}
	if o.Meta.GetName() != common.ArgoCDRepoServerTLSSecretName {
		return result
	}
	namespacedArgoCDObject := client.ObjectKey{}
	secretOwnerRefs := o.Meta.GetOwnerReferences()
	for _, secretOwner := range secretOwnerRefs {
		if secretOwner.Kind == "Service" && strings.HasSuffix(secretOwner.Name, "-repo-server") {
			key := client.ObjectKey{Name: secretOwner.Name, Namespace: o.Meta.GetNamespace()}
			svc := &corev1.Service{}
			err := r.client.Get(context.TODO(), key, svc)
			if err != nil {
				log.Error(err, fmt.Sprintf("could not get owner of secret %s", o.Meta.GetName()))
				return result
			}
			serviceOwnerRefs := svc.GetOwnerReferences()
			for _, serviceOwner := range serviceOwnerRefs {
				if serviceOwner.Kind == "ArgoCD" {
					namespacedArgoCDObject.Name = serviceOwner.Name
					namespacedArgoCDObject.Namespace = svc.ObjectMeta.Namespace
					result = []reconcile.Request{
						{NamespacedName: namespacedArgoCDObject},
					}
				}
			}
		}
	}
	return result
}
