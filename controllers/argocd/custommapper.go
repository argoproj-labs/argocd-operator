package argocd

import (
	"context"
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v3/util/glob"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ReconcileArgoCD) clusterResourceMapper(ctx context.Context, o client.Object) []reconcile.Request {
	crbAnnotations := o.GetAnnotations()
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

// isSecretOfInterest returns true if the name of the given secret matches one of the
// well-known tls secrets used to secure communication amongst the Argo CD components.
func isSecretOfInterest(o client.Object) bool {
	if strings.HasSuffix(o.GetName(), "-repo-server-tls") {
		return true
	}
	if o.GetName() == common.ArgoCDRedisServerTLSSecretName {
		return true
	}
	return false
}

// isOwnerOfInterest returns true if the given owner is one of the Argo CD services that
// may have been made the owner of the tls secret created by the OpenShift service CA, used
// to secure communication amongst the Argo CD components.
func isOwnerOfInterest(owner v1.OwnerReference) bool {
	if owner.Kind != "Service" {
		return false
	}
	if strings.HasSuffix(owner.Name, "-repo-server") {
		return true
	}
	if strings.HasSuffix(owner.Name, "-redis") {
		return true
	}
	return false
}

// isUserManagedSecret checks if the given secret is referenced in the ArgoCD CR for configuring the Argo CD instance.
// User-managed secrets are referenced by the ArgoCD CR but are not owned by Operator itself (i.e. managed by the user).
// Returns the namespaced name of the ArgoCD instance if found and a boolean indicating whether the secret is user-managed.
func (r *ReconcileArgoCD) isUserManagedSecret(ctx context.Context, o client.Object) (client.ObjectKey, bool) {
	namespacedName := client.ObjectKey{}
	var ok bool

	// List ArgoCD instances in the same namespace as the secret.
	argocds := &argoproj.ArgoCDList{}
	err := r.Client.List(ctx, argocds, &client.ListOptions{Namespace: o.GetNamespace()})
	if err != nil {
		return namespacedName, false
	}
	// Return false if no ArgoCD instance or more than one is detected in the namespace.
	if len(argocds.Items) != 1 {
		return namespacedName, false
	}
	argocd := argocds.Items[0]
	namespacedName.Name = argocd.Name
	namespacedName.Namespace = argocd.Namespace

	// Check if the secret is referenced in the ArgoCD CR.
	if argocd.Spec.Server.Route.UseExternalCertificate() && argocd.Spec.Server.Route.TLS.ExternalCertificate.Name == o.GetName() {
		ok = true
	} else if argocd.Spec.Prometheus.Route.UseExternalCertificate() && argocd.Spec.Prometheus.Route.TLS.ExternalCertificate.Name == o.GetName() {
		ok = true
	} else if argocd.Spec.ApplicationSet != nil && argocd.Spec.ApplicationSet.WebhookServer.Route.UseExternalCertificate() && argocd.Spec.ApplicationSet.WebhookServer.Route.TLS.ExternalCertificate.Name == o.GetName() {
		ok = true
	}

	return namespacedName, ok
}

// tlsSecretMapper maps a watch event on a secret of type TLS back to the
// ArgoCD object that we want to reconcile.
func (r *ReconcileArgoCD) tlsSecretMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	// Check if secret is user-managed, meaning it is referenced in the ArgoCD CR for configuration.
	if namespacedName, ok := r.isUserManagedSecret(ctx, o); ok {
		return []reconcile.Request{{NamespacedName: namespacedName}}
	}

	if !isSecretOfInterest(o) {
		return result
	}
	namespacedArgoCDObject := client.ObjectKey{}

	secretOwnerRefs := o.GetOwnerReferences()
	if len(secretOwnerRefs) > 0 {
		// OpenShift service CA makes the owner reference for the TLS secret to the
		// service, which in turn is owned by the controller. This method performs
		// a lookup of the controller through the intermediate owning service.
		for _, secretOwner := range secretOwnerRefs {
			if isOwnerOfInterest(secretOwner) {
				key := client.ObjectKey{Name: secretOwner.Name, Namespace: o.GetNamespace()}
				svc := &corev1.Service{}

				// Get the owning object of the secret
				err := r.Client.Get(context.TODO(), key, svc)
				if err != nil {
					log.Error(err, fmt.Sprintf("could not get owner of secret %s", o.GetName()))
					return result
				}

				// If there's an object of kind ArgoCD in the owner's list,
				// this will be our reconciled object.
				serviceOwnerRefs := svc.GetOwnerReferences()
				for _, serviceOwner := range serviceOwnerRefs {
					if serviceOwner.Kind == "ArgoCD" {
						namespacedArgoCDObject.Name = serviceOwner.Name
						namespacedArgoCDObject.Namespace = svc.ObjectMeta.Namespace
						result = []reconcile.Request{
							{NamespacedName: namespacedArgoCDObject},
						}
						return result
					}
				}
			}
		}
	} else {
		// For secrets without owner (i.e. manually created), we apply some
		// heuristics. This may not be as accurate (e.g. if the user made a
		// typo in the resource's name), but should be good enough for now.
		secret, ok := o.(*corev1.Secret)
		if !ok {
			return result
		}
		if owner, ok := secret.Annotations[common.AnnotationName]; ok {
			namespacedArgoCDObject.Name = owner
			namespacedArgoCDObject.Namespace = o.GetNamespace()
			result = []reconcile.Request{
				{NamespacedName: namespacedArgoCDObject},
			}
		}
	}

	return result
}

// namespaceResourceMapper maps a watch event on a namespace, back to the
// ArgoCD object that we want to reconcile.
func (r *ReconcileArgoCD) namespaceResourceMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	argocds := &argoproj.ArgoCDList{}
	labels := o.GetLabels()
	namespaceName := o.GetName()
	if v, ok := labels[common.ArgoCDManagedByLabel]; ok {
		if err := r.Client.List(context.TODO(), argocds, &client.ListOptions{Namespace: v}); err != nil {
			return result
		}
		if len(argocds.Items) != 1 {
			return result
		}
		argocd := argocds.Items[0]
		namespacedName := client.ObjectKey{
			Name:      argocd.Name,
			Namespace: argocd.Namespace,
		}
		result = []reconcile.Request{
			{NamespacedName: namespacedName},
		}
	} else {
		// If the namespace does not have the expected managed-by label,
		// iterate through each ArgoCD instance to identify if the observed namespace
		// matches any configured sourceNamespace pattern. If a match is found,
		// generate a reconcile request for the instances.
		if err := r.Client.List(ctx, argocds, &client.ListOptions{}); err != nil {
			return result
		}
		for _, argocd := range argocds.Items {
			if glob.MatchStringInList(argocd.Spec.SourceNamespaces, namespaceName, glob.GLOB) {
				namespacedName := client.ObjectKey{
					Name:      argocd.Name,
					Namespace: argocd.Namespace,
				}
				result = append(result, reconcile.Request{NamespacedName: namespacedName})
			}
		}
	}

	return result
}

// clusterSecretResourceMapper maps a watch event on a namespace, back to the
// ArgoCD object that we want to reconcile.
func (r *ReconcileArgoCD) clusterSecretResourceMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	labels := o.GetLabels()
	if v, ok := labels[common.ArgoCDSecretTypeLabel]; ok && v == "cluster" {
		argocds := &argoproj.ArgoCDList{}
		if err := r.Client.List(context.TODO(), argocds, &client.ListOptions{Namespace: o.GetNamespace()}); err != nil {
			return result
		}

		if len(argocds.Items) != 1 {
			return result
		}

		argocd := argocds.Items[0]
		namespacedName := client.ObjectKey{
			Name:      argocd.Name,
			Namespace: argocd.Namespace,
		}
		result = []reconcile.Request{
			{NamespacedName: namespacedName},
		}
	}

	return result
}

// applicationSetSCMTLSConfigMapMapper maps a watch event on a configmap with name "argocd-appset-gitlab-scm-tls-certs-cm",
// back to the ArgoCD object that we want to reconcile.
func (r *ReconcileArgoCD) applicationSetSCMTLSConfigMapMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result = []reconcile.Request{}

	if o.GetName() == common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName {
		argocds := &argoproj.ArgoCDList{}
		if err := r.Client.List(context.TODO(), argocds, &client.ListOptions{Namespace: o.GetNamespace()}); err != nil {
			return result
		}

		if len(argocds.Items) != 1 {
			return result
		}

		argocd := argocds.Items[0]
		namespacedName := client.ObjectKey{
			Name:      argocd.Name,
			Namespace: argocd.Namespace,
		}
		result = []reconcile.Request{
			{NamespacedName: namespacedName},
		}
	}

	return result
}

// namespaceResourceMapper maps a watch event on a namespaceManagement, back to the
// ArgoCD object that we want to reconcile.
func (r *ReconcileArgoCD) nmMapper(ctx context.Context, o client.Object) []reconcile.Request {
	var result []reconcile.Request

	// List ALL ArgoCD CRs in the cluster
	argocdList := &argoproj.ArgoCDList{}
	if err := r.List(ctx, argocdList); err != nil {
		return result
	}

	// Reconcile each ArgoCD instance
	for _, argocd := range argocdList.Items {
		result = append(result, reconcile.Request{
			NamespacedName: client.ObjectKey{
				Name:      argocd.Name,
				Namespace: argocd.Namespace,
			},
		})
	}

	return result
}
