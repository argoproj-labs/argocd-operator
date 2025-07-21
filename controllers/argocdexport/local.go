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
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

// reconcileLocalStorage will ensure the PersistentVolumeClaim is present for the ArgoCDExport.
func (r *ReconcileArgoCDExport) reconcileLocalStorage(cr *argoproj.ArgoCDExport) error {
	if cr.Spec.Storage == nil || strings.ToLower(cr.Spec.Storage.Backend) != common.ArgoCDExportStorageBackendLocal {
		return nil // Do nothing if storage or local options not set
	}

	log.Info("reconciling local pvc")
	if err := r.reconcilePVC(cr); err != nil {
		return err
	}
	return nil
}

// reconcilePVC will ensure that the PVC for the ArgoCDExport is present.
func (r *ReconcileArgoCDExport) reconcilePVC(cr *argoproj.ArgoCDExport) error {
	if cr.Status.Phase == common.ArgoCDStatusCompleted {
		return nil // Nothing to see here, move along...
	}

	pvc := argoutil.NewPersistentVolumeClaim(cr.ObjectMeta)
	pvcExists, err := argoutil.IsObjectFound(r.Client, cr.Namespace, pvc.Name, pvc)
	if err != nil {
		return err
	}
	if pvcExists {
		return nil // PVC exists, move along...
	}

	// Allow override of PVC spec
	if cr.Spec.Storage.PVC != nil {
		pvc.Spec = *cr.Spec.Storage.PVC
	} else {
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		pvc.Spec.Resources = argoutil.DefaultPVCResources()
	}

	if err := controllerutil.SetControllerReference(cr, pvc, r.Scheme); err != nil {
		return err
	}

	// Create PVC
	argoutil.LogResourceCreation(log, pvc)
	if err := r.Client.Create(context.TODO(), pvc); err != nil {
		return err
	}

	// Create event
	return argoutil.CreateEvent(r.Client, "Normal", "Exporting", "Created claim for export process.", "PersistentVolumeClaimCreated", cr.ObjectMeta, cr.TypeMeta)
}
