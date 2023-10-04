package reposerver

import (
	"crypto/sha256"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/appcontroller"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (rsr *RepoServerReconciler) reconcileTLSSecret() error {
	var sha256sum string
	rsr.Logger.Info("reconciling TLS secrets")

	secretRequest := workloads.SecretRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RepoServerTLSSecretName,
			Namespace: rsr.Instance.Namespace,
			Labels:    resourceLabels,
		},

		Client:    rsr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	desiredSecret, err := workloads.RequestSecret(secretRequest)
	if err != nil {
		rsr.Logger.Error(err, "reconcileSecret: failed to request secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
		return err
	}

	namespace, err := cluster.GetNamespace(rsr.Instance.Namespace, rsr.Client)
	if err != nil {
		rsr.Logger.Error(err, "reconcileSecret: failed to retrieve namespace", "name", rsr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := rsr.deleteTLSSecret(desiredSecret.Namespace); err != nil {
			rsr.Logger.Error(err, "reconcileSecret: failed to delete secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
		}
		return err
	}

	existingSecret, err := workloads.GetSecret(desiredSecret.Name, desiredSecret.Namespace, rsr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			rsr.Logger.Error(err, "reconcileSecret: failed to retrieve secret", "name", existingSecret.Name, "namespace", existingSecret.Namespace)
			return err
		}
	} else if existingSecret.Type != corev1.SecretTypeTLS {
		return nil
	} else {
		crt, crtOk := existingSecret.Data[corev1.TLSCertKey]
		key, keyOk := existingSecret.Data[corev1.TLSPrivateKeyKey]
		if crtOk && keyOk {
			var sumBytes []byte
			sumBytes = append(sumBytes, crt...)
			sumBytes = append(sumBytes, key...)
			sha256sum = fmt.Sprintf("%x", sha256.Sum256(sumBytes))
		}
	}

	if rsr.Instance.Status.RepoTLSChecksum != sha256sum {
		rsr.Instance.Status.RepoTLSChecksum = sha256sum
		err = workloads.UpdateSecret(desiredSecret, rsr.Client)
		if err != nil {
			rsr.Logger.Error(err, "reconcileSecret: failed to update secret", "name", desiredSecret.Name, "namespace", desiredSecret.Namespace)
			return err
		}

		// Trigger rollout of API components
		components := []string{common.Server, RepoServerControllerComponent, appcontroller.ArgoCDApplicationControllerComponent}
		for _, component := range components {
			depl, err := workloads.CreateDeploymentWithSuffix(component, component, rsr.Instance)
			if err != nil {
				return err
			}
			err = argocdcommon.TriggerRollout(rsr.Client, depl, common.ArgoCDRepoTLSCertChangedKey)
			if err != nil {
				return err
			}
		}

		rsr.Logger.V(0).Info("reconcileSecret: TLS secret updated", "name", RepoServerTLSSecretName, "namespace", desiredSecret.Namespace)
		return nil
	}

	return nil
}

func (rsr *RepoServerReconciler) deleteTLSSecret(namespace string) error {
	if err := workloads.DeleteSecret(RepoServerTLSSecretName, namespace, rsr.Client); err != nil {
		rsr.Logger.Error(err, "DeleteSecret: failed to delete secret", "name", RepoServerTLSSecretName, "namespace", namespace)
		return err
	}
	rsr.Logger.V(0).Info("DeleteSecret: secret deleted", "name", RepoServerTLSSecretName, "namespace", namespace)
	return nil
}
