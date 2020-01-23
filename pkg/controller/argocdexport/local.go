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

	argoproj "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj"
	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// getPersistentVolumes will return the list of PersistentVolumes that match the given labels for the given ArgoCDExport.
func (r *ReconcileArgoCDExport) getPersistentVolumes(cr *argoprojv1a1.ArgoCDExport) ([]corev1.PersistentVolume, error) {
	lbls := make(map[string]string, 1)
	lbls[argoproj.ArgoCDExportName] = cr.Name

	opts := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(lbls),
	}

	list := &corev1.PersistentVolumeList{}
	if err := r.client.List(context.TODO(), list, opts); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// newPV returns a new PersistentVolume instance for the given ArgoCDExport.
func newPV(cr *argoprojv1a1.ArgoCDExport) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.DefaultLabels(cr.Name),
		},
	}
}

// newPVWithName creates a new PersistentVolume with the given name and CR.
func newPVWithName(name string, cr *argoprojv1a1.ArgoCDExport) *corev1.PersistentVolume {
	pv := newPV(cr)
	pv.ObjectMeta.Name = name

	lbls := pv.ObjectMeta.Labels
	lbls[argoproj.ArgoCDKeyName] = name
	pv.ObjectMeta.Labels = lbls

	return pv
}

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

// reconcilePV will ensure that the PersistentVolume with the given name is configured properly for the given ArgoCDExport.
func (r *ReconcileArgoCDExport) reconcilePV(name string, cr *argoprojv1a1.ArgoCDExport) error {
	pv := newPVWithName(name, cr)
	if !argoutil.IsObjectFound(r.client, "", pv.Name, pv) { // Note: PersistentVolumes have no namespace!
		return nil // PV not found, move along...
	}

	log.Info("reconciling pv")
	changed := false

	// Ensure labels are set
	if pv.ObjectMeta.Labels[argoproj.ArgoCDExportName] != cr.Name {
		log.Info("updating labels for pv")
		lbls := make(map[string]string, 1)
		lbls[argoproj.ArgoCDExportName] = cr.Name
		pv.Labels = argoutil.AppendStringMap(pv.Labels, lbls)
		changed = true
	}

	// Ensure reclaim policy is set to retain the PV.
	if pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
		log.Info("updating reclaim policy for pv")
		pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
		changed = true
	}

	// Ensure claim is removed when status is Completed
	if cr.Status.Phase == argoproj.ArgoCDStatusCompleted {
		log.Info("removing claim for pv")
		pv.Spec.ClaimRef = nil
		changed = true
	}

	if changed {
		return r.client.Update(context.TODO(), pv)
	}
	return nil
}

// reconcileExistingPVC will maintain the desired state for an existing PersistentVolumeClaim for the given ArgoCDExport.
func (r *ReconcileArgoCDExport) reconcileExistingPVC(pvc *corev1.PersistentVolumeClaim, cr *argoprojv1a1.ArgoCDExport) error {
	if err := r.reconcilePV(pvc.Spec.VolumeName, cr); err != nil {
		return err
	}
	if cr.Status.Phase == argoproj.ArgoCDStatusCompleted {
		log.Info("export completed, removing pvc")
		if len(pvc.Finalizers) > 0 {
			log.Info("removing finalizers from pvc")
			pvc.Finalizers = []string{}
			return r.client.Update(context.TODO(), pvc)
		}
		return r.client.Delete(context.TODO(), pvc)
	}
	return nil
}

// reconcileExistingPVC will maintain the desired state for a new PersistentVolumeClaim for the given ArgoCDExport.
func (r *ReconcileArgoCDExport) reconcileNewPVC(pvc *corev1.PersistentVolumeClaim, cr *argoprojv1a1.ArgoCDExport) error {
	if cr.Status.Phase == argoproj.ArgoCDStatusCompleted {
		return nil // Nothing to see here, move along...
	}

	// Allow override of PVC spec
	if cr.Spec.Storage.Local.PVC != nil {
		pvc.Spec = *cr.Spec.Storage.Local.PVC
	} else {
		pvc.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		pvc.Spec.Resources = argoutil.DefaultPVCResources()
	}

	// Use existing PersistentVolume if one exists
	pvs, err := r.getPersistentVolumes(cr)
	if err != nil {
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
	err = argoutil.CreateEvent(r.client, "Exporting", "Created claim for export process.", "PersistentVolumeClaimCreated", cr.ObjectMeta)
	if err != nil {
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

// reconcilePVC will ensure that the PVC for the ArgoCDExport is present.
func (r *ReconcileArgoCDExport) reconcilePVC(cr *argoprojv1a1.ArgoCDExport) error {
	pvc := argoutil.NewPersistentVolumeClaim(cr.ObjectMeta)
	if argoutil.IsObjectFound(r.client, cr.Namespace, pvc.Name, pvc) {
		return r.reconcileExistingPVC(pvc, cr)
	}
	return r.reconcileNewPVC(pvc, cr)
}
