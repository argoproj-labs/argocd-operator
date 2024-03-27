package argocdcommon

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

func GetClusterSecrets(ns string, cl client.Client) (*corev1.SecretList, error) {
	clusterSecretReq, err := GetLabelRequirements(common.ArgoCDSecretTypeLabel, selection.Equals, []string{common.ArgoCDSecretTypeCluster})
	if err != nil {
		return nil, errors.Wrap(err, "GetClusterSecrets: failed to generate requirement")
	}

	clusterSecretLs := GetLabelSelector(*clusterSecretReq)
	clusterSecrets, err := workloads.ListSecrets(ns, cl, []client.ListOption{
		&client.ListOptions{
			LabelSelector: clusterSecretLs,
			Namespace:     ns,
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "GetClusterSecrets: failed to generate labelSelector")
	}

	return clusterSecrets, nil
}

// TLSSecretChecksum retrieves a specified TLS secret and calculates its checksum value and returns it. If a secret is determined to be of type other than TLS it returns an empty string
func TLSSecretChecksum(secretRef types.NamespacedName, client cntrlClient.Client) (string, error) {
	var sha256sum string

	tlsSecret, err := workloads.GetSecret(secretRef.Name, secretRef.Namespace, client)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", err
	}

	if tlsSecret.Type != corev1.SecretTypeTLS {
		// We only process secrets of type kubernetes.io/tls
		return "", nil
	}

	crt, crtOk := tlsSecret.Data[corev1.TLSCertKey]
	key, keyOk := tlsSecret.Data[corev1.TLSPrivateKeyKey]
	if crtOk && keyOk {
		var sumBytes []byte
		sumBytes = append(sumBytes, crt...)
		sumBytes = append(sumBytes, key...)
		sha256sum = fmt.Sprintf("%x", sha256.Sum256(sumBytes))
	}
	return sha256sum, nil
}

// FindSecretOwnerInstance finds the Argo CD instance that directly or indirectly owns the given secret. It looks up a given secret, checks if it is owned by an Argo CD service or not. If yes, finds the Argo CD instance that owns the service and returns a reference to that instance. If not, it looks for a reference to the owning instance set in the secret's annotations, and returns the reference from there
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
			if IsOwnerOfInterest(secretOwner) {
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

// isSecretOfInterest returns true if the name of the given secret matches one of the
// well-known tls secrets used to secure communication amongst the Argo CD components.
func IsSecretOfInterest(o client.Object) bool {
	if strings.HasSuffix(o.GetName(), common.RepoServerTLSSuffix) {
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
func IsOwnerOfInterest(owner metav1.OwnerReference) bool {
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
