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

package argocd

import (
	"context"
	"fmt"

	argoproj "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj"
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
)

// reconcileImportPVC will ensure that the PVC is present for the ArgoCD.
func (r *ReconcileArgoCD) reconcileImportPVC(cr *argoprojv1a1.ArgoCD) error {
	if cr.Spec.Import == nil || len(cr.Spec.Import.Name) <= 0 {
		return nil // No import information present, move along...
	}

	expName := cr.Spec.Import.Name
	expNamespace := cr.Spec.Import.Namespace
	if expNamespace == nil {
		expNamespace = &cr.Namespace
	}

	export := &argoprojv1a1.ArgoCDExport{}
	if err := argoutil.FetchObject(r.client, *expNamespace, expName, export); err != nil {
		log.Error(err, fmt.Sprintf("unable to locate ArgoCDExport %s/%s", *expNamespace, expName))
		return nil // Do not return error
	}

	if export.Status.Phase != argoproj.ArgoCDStatusCompleted {
		log.Info("export not marked complete, skipping import pvc")
		return nil
	}

	pvc := argoutil.NewPersistentVolumeClaimWithName(fmt.Sprintf("%s-import", expName), cr.ObjectMeta)
	if argoutil.IsObjectFound(r.client, cr.Namespace, pvc.Name, pvc) {
		return nil // PVC found, do nothing
	}

	pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	pvc.Spec.Resources = argoutil.DefaultPVCResources()

	// Use existing PersistentVolume if one exists
	var pvs []corev1.PersistentVolume
	labelz := make(map[string]string, 1)
	labelz[argoproj.ArgoCDExportName] = cr.Name

	if err := argoutil.FetchPersistentVolumes(r.client, labelz, &pvs); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("found %d existing persistent volumes", len(pvs)))
	var existingPV *corev1.PersistentVolume
	for _, pv := range pvs {
		log.Info(fmt.Sprintf("found existing pv: %s", pv.Name))
		// Claim the first PV found that matches the following:
		// There is not claim OR
		// The name and namespace for the claim match this PVC
		if pv.Spec.ClaimRef == nil || (pv.Spec.ClaimRef.Name == pvc.Name && pv.Spec.ClaimRef.Namespace == pvc.Namespace) {
			log.Info(fmt.Sprintf("claiming existing pv: %s", pv.Name))
			existingPV = &pv
			pvc.Spec.VolumeName = existingPV.Name
			break
		}
	}

	if err := controllerutil.SetControllerReference(cr, pvc, r.scheme); err != nil {
		return err
	}

	// Create PVC
	log.Info(fmt.Sprintf("creating new pvc: %s", pvc.Name))
	if err := r.client.Create(context.TODO(), pvc); err != nil {
		return err
	}

	log.Info("creating new event")
	if err := argoutil.CreateEvent(r.client, "Exporting", "Created claim for import process.", "PersistentVolumeClaimCreated", cr.ObjectMeta); err != nil {
		return err
	}

	// Update claim to existing PV, if found
	if existingPV != nil {
		log.Info(fmt.Sprintf("updating claim for existing pv: %s", existingPV.Name))
		existingPV.Spec.ClaimRef = &corev1.ObjectReference{
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
		}
		r.client.Update(context.TODO(), existingPV)
	}
	return nil
}
