package argocdcommon

import (
	"strings"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// FindSecretOwnerInstance finds the Argo CD instance that indirectly owns the given secret. It looks up a given secret, checks if it is owned by an Argo CD service or not. If yes, finds the Argo CD instance that owns the service and returns a reference to that instance
func FindSecretOwnerInstance(secretRef types.NamespacedName, client cntrlClient.Client) (types.NamespacedName, error) {
	owner := types.NamespacedName{}

	secret, err := workloads.GetSecret(secretRef.Name, secretRef.Namespace, client)
	if err != nil {
		return types.NamespacedName{}, err
	}

	secretOwnerRefs := secret.GetOwnerReferences()
	if len(secretOwnerRefs) > 0 {
		// OpenShift service CA sets the owner reference for the TLS secret to be a
		// service, which in turn is owned by an Argo CD instance. This method performs
		// a lookup of the instance through the intermediate owning service.
		for _, secretOwner := range secretOwnerRefs {
			if isOwnerOfInterest(secretOwner) {
				owningSvc, err := networking.GetService(secretOwner.Name, secret.Namespace, client)
				if err != nil {
					return types.NamespacedName{}, err
				}

				svcOwnerRefs := owningSvc.GetOwnerReferences()
				for _, svcOwner := range svcOwnerRefs {
					if svcOwner.Kind == common.ArgoCDKind {
						owner.Name = svcOwner.Name
						owner.Namespace = secret.Namespace
						break
					}
				}
			}
		}
	} else {
		// For secrets without owner (i.e. manually created), we apply some
		// heuristics. This may not be as accurate (e.g. if the user made a
		// typo in the resource's name), but should be good enough for now.
		if _, ok := secret.Annotations[common.ArgoCDArgoprojKeyName]; ok {
			owner.Name = secret.Annotations[common.ArgoCDArgoprojKeyName]
			owner.Namespace = secret.Annotations[common.ArgoCDArgoprojKeyNamespace]
		}
	}
	return owner, nil
}

// isOwnerOfInterest returns true if the given owner is one of the Argo CD services that
// may have been made the owner of the tls secret created by the OpenShift service CA, used
// to secure communication amongst the Argo CD components.
func isOwnerOfInterest(owner metav1.OwnerReference) bool {
	if owner.Kind != common.ServiceKind {
		return false
	}
	if strings.HasSuffix(owner.Name, common.RepoServerSuffix) {
		return true
	}
	if strings.HasSuffix(owner.Name, common.RedisSuffix) {
		return true
	}
	return false
}
