package reposerver

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/cluster"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (rsr *RepoServerReconciler) reconcileTLSSecret() error {
	var sha256sum string
	rsr.Logger.Info("reconciling TLS secrets")

	namespace, err := cluster.GetNamespace(rsr.Instance.Namespace, rsr.Client)
	if err != nil {
		rsr.Logger.Error(err, "reconcileSecret: failed to retrieve namespace", "name", rsr.Instance.Namespace)
		return err
	}
	if namespace.DeletionTimestamp != nil {
		if err := rsr.deleteTLSSecret(rsr.Instance.Namespace); err != nil {
			rsr.Logger.Error(err, "reconcileSecret: failed to delete secret", "name", common.RepoServerTLSSecretName, "namespace", rsr.Instance.Namespace)
		}
		return err
	}

	existingSecret, err := workloads.GetSecret(common.RepoServerTLSSecretName, rsr.Instance.Namespace, rsr.Client)
	if err != nil {
		if !errors.IsNotFound(err) {
			rsr.Logger.Error(err, "reconcileSecret: failed to retrieve secret", "name", common.RepoServerTLSSecretName, "namespace", rsr.Instance.Namespace)
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
		err = rsr.Client.Status().Update(context.TODO(), rsr.Instance)
		// err = workloads.UpdateSecret(desiredSecret, rsr.Client)
		if err != nil {
			rsr.Logger.Error(err, "reconcileSecret: failed to update status", "name", common.RepoServerTLSSecretName, "namespace", rsr.Instance.Namespace)
			return err
		}

		// Trigger rollout of API components
		err = rsr.TriggerRepoServerDeploymentRollout()
		if err != nil {
			return err
		}

		err = rsr.ServerController.TriggerServerDeploymentRollout()
		if err != nil {
			return err
		}

		err = rsr.AppController.TriggerAppControllerStatefulSetRollout()
		if err != nil {
			return err
		}

		rsr.Logger.V(0).Info("reconcileSecret: argocd client status updated", "name", common.RepoServerTLSSecretName, "namespace", rsr.Instance.Namespace)
		return nil
	}

	return nil
}

func (rsr *RepoServerReconciler) deleteTLSSecret(namespace string) error {
	if err := workloads.DeleteSecret(common.RepoServerTLSSecretName, namespace, rsr.Client); err != nil {
		rsr.Logger.Error(err, "DeleteSecret: failed to delete secret", "name", common.RepoServerTLSSecretName, "namespace", namespace)
		return err
	}
	rsr.Logger.V(0).Info("DeleteSecret: secret deleted", "name", common.RepoServerTLSSecretName, "namespace", namespace)
	return nil
}
