package redis

import (
	"context"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// getRedisServerAddress will return the Redis service address for the given ArgoCD.
func GetRedisServerAddress(cr *argoproj.ArgoCD) string {
	if cr.Spec.HA.Enabled {
		return GetRedisHAProxyAddress(cr.Namespace)
	}
	return util.FqdnServiceRef(common.ArgoCDDefaultRedisSuffix, cr.Namespace, common.ArgoCDDefaultRedisPort)
}

// getRedisHAProxyAddress will return the Redis HA Proxy service address for the given ArgoCD.
func GetRedisHAProxyAddress(namespace string) string {
	return util.FqdnServiceRef(RedisHAProxyServiceName, namespace, common.ArgoCDDefaultRedisPort)
}

func ShouldUseTLS(client cntrlClient.Client, instanceNamespace string) (bool, error) {
	tlsSecretName := types.NamespacedName{Namespace: instanceNamespace, Name: common.ArgoCDRedisServerTLSSecretName}
	var tlsSecretObj corev1.Secret
	if err := client.Get(context.TODO(), tlsSecretName, &tlsSecretObj); err != nil {
		if !errors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}

	secretOwnerRefs := tlsSecretObj.GetOwnerReferences()
	if len(secretOwnerRefs) > 0 {
		// OpenShift service CA makes the owner reference for the TLS secret to the
		// service, which in turn is owned by the controller. This method performs
		// a lookup of the controller through the intermediate owning service.
		for _, secretOwner := range secretOwnerRefs {
			if argocdcommon.IsOwnerOfInterest(secretOwner) {
				key := cntrlClient.ObjectKey{Name: secretOwner.Name, Namespace: tlsSecretObj.GetNamespace()}
				svc := &corev1.Service{}
				// Get the owning object of the secret
				if err := client.Get(context.TODO(), key, svc); err != nil {
					return false, err
				}

				// If there's an object of kind ArgoCD in the owner's list,
				// this will be our reconciled object.
				serviceOwnerRefs := svc.GetOwnerReferences()
				for _, serviceOwner := range serviceOwnerRefs {
					if serviceOwner.Kind == "ArgoCD" {
						return true, nil
					}
				}
			}
		}
	} else {
		// For secrets without owner (i.e. manually created), we apply some
		// heuristics. This may not be as accurate (e.g. if the user made a
		// typo in the resource's name), but should be good enough for now.
		if _, ok := tlsSecretObj.Annotations[common.ArgoCDArgoprojKeyName]; ok {
			return true, nil
		}
	}
	return false, nil
}
