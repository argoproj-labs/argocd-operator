// Copyright 2019 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argocdexport

import (
	"context"

	"github.com/sethvargo/go-password/password"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
)

// generateBackupKey will generate and return the backup key for the export process.
func generateBackupKey() ([]byte, error) {
	pass, err := password.Generate(
		common.ArgoCDDefaultBackupKeyLength,
		common.ArgoCDDefaultBackupKeyNumDigits,
		common.ArgoCDDefaultBackupKeyNumSymbols,
		false, false)

	return []byte(pass), err
}

// reconcileExport will ensure that the resources for the export process are present for the ArgoCDExport.
func (r *ArgoCDExportReconciler) reconcileExport(cr *argoprojv1alpha1.ArgoCDExport) error {
	log.Info("reconciling export secret")
	if err := r.reconcileExportSecret(cr); err != nil {
		return err
	}

	if cr.Spec.Schedule != nil && len(*cr.Spec.Schedule) > 0 {
		log.Info("reconciling export cronjob")
		if err := r.reconcileCronJob(cr); err != nil {
			return err
		}
	} else {
		log.Info("reconciling export job")
		if err := r.reconcileJob(cr); err != nil {
			return err
		}
	}

	return nil
}

// FetchStorageSecretName will return the name of the Secret to use for the export process.
func FetchStorageSecretName(export *argoprojv1alpha1.ArgoCDExport) string {
	name := argoutil.NameWithSuffix(export.ObjectMeta.Name, "export")
	if export.Spec.Storage != nil && len(export.Spec.Storage.SecretName) > 0 {
		name = export.Spec.Storage.SecretName
	}
	return name
}

// reconcileExportSecret will ensure that the Secret used for the export process is present.
func (r *ArgoCDExportReconciler) reconcileExportSecret(cr *argoprojv1alpha1.ArgoCDExport) error {
	name := FetchStorageSecretName(cr)
	// Dummy CR to retrieve secret
	a := &argoproj.ArgoCD{}
	a.ObjectMeta = cr.ObjectMeta
	secret := argoutil.NewSecretWithName(a, name)
	if argoutil.IsObjectFound(r.Client, cr.Namespace, name, secret) {
		backupKey := secret.Data[common.ArgoCDKeyBackupKey]
		if len(backupKey) <= 0 {
			backupKey, err := generateBackupKey()
			if err != nil {
				return err
			}
			secret.Data[common.ArgoCDKeyBackupKey] = backupKey
			return r.Client.Update(context.TODO(), secret)
		}

		return nil // TODO: Handle case where backup key changes, should trigger a new export?
	}

	backupKey, err := generateBackupKey()
	if err != nil {
		return err
	}

	secret.Data = map[string][]byte{
		common.ArgoCDKeyBackupKey: backupKey,
	}

	if err := controllerutil.SetControllerReference(cr, secret, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), secret)
}

// validateExport will ensure that the given ArgoCDExport is valid.
func (r *ArgoCDExportReconciler) validateExport(cr *argoprojv1alpha1.ArgoCDExport) error {
	if len(cr.Status.Phase) <= 0 {
		cr.Status.Phase = "Pending"
		return r.Client.Status().Update(context.TODO(), cr)
	}
	return nil
}
