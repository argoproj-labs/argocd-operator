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
	"fmt"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/common"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// reconcileLocalStorage will ensure the PersistentVolumeClaim is present for the ArgoCDExport.
func (r *ReconcileArgoCDExport) reconcileLocalStorage(cr *argoprojv1a1.ArgoCDExport) error {
	if cr.Spec.Storage == nil || cr.Spec.Storage.Local == nil {
		return nil // Do nothing if storage or local options not set
	}

	log.Info("reconciling local pvc")
	if err := r.reconcilePVC(cr); err != nil {
		return err
	}
	return nil
}

// reconcilePVC will ensure that the PVC for the ArgoCDExport is present.
func (r *ReconcileArgoCDExport) reconcilePVC(cr *argoprojv1a1.ArgoCDExport) error {
	if cr.Status.Phase == common.ArgoCDStatusCompleted {
		return nil // Nothing to see here, move along...
	}

	pvc := argoutil.NewPersistentVolumeClaim(cr.ObjectMeta)
	if argoutil.IsObjectFound(r.client, cr.Namespace, pvc.Name, pvc) {
		return nil // PVC exists, move along...
	}

	// Allow override of PVC spec
	if cr.Spec.Storage.Local.PVC != nil {
		pvc.Spec = *cr.Spec.Storage.Local.PVC
	} else {
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		pvc.Spec.Resources = argoutil.DefaultPVCResources()
	}

	if err := controllerutil.SetControllerReference(cr, pvc, r.scheme); err != nil {
		return err
	}

	// Create PVC
	log.Info(fmt.Sprintf("creating new pvc: %s", pvc.Name))
	if err := r.client.Create(context.TODO(), pvc); err != nil {
		return err
	}

	// Create event
	log.Info("creating new event")
	return argoutil.CreateEvent(r.client, "Exporting", "Created claim for export process.", "PersistentVolumeClaimCreated", cr.ObjectMeta)
}
