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
	batchv1 "k8s.io/api/batch/v1"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// getArgoExportCommand will return the command for the ArgoCD export process.
func getArgoExportCommand(cr *argoprojv1a1.ArgoCDExport) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "/bin/bash")
	cmd = append(cmd, "-c")
	cmd = append(cmd, "argocd-util export > /backups/argocd-backup.yaml")
	return cmd
}

func getArgoStorageVolume(name string, cr *argoprojv1a1.ArgoCDExport) corev1.Volume {
	volume := corev1.Volume{
		Name: name,
	}

	// Handle Local storage volume
	if cr.Spec.Storage != nil && cr.Spec.Storage.Local != nil {
		volume.VolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: cr.Name,
			},
		}
	}

	return volume
}

// newJob returns a new Job instance for the given ArgoCDExport.
func newJob(cr *argoprojv1a1.ArgoCDExport) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.DefaultLabels(cr.Name),
		},
	}
}

// newCronJob returns a new CronJob instance for the given ArgoCDExport.
func newCronJob(cr *argoprojv1a1.ArgoCDExport) *batchv1b1.CronJob {
	return &batchv1b1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.DefaultLabels(cr.Name),
		},
	}
}

func newExportPodSpec(cr *argoprojv1a1.ArgoCDExport) corev1.PodSpec {
	pod := corev1.PodSpec{}

	pod.Containers = []corev1.Container{{
		Command:         getArgoExportCommand(cr),
		Image:           fmt.Sprintf("%s:%s", argoproj.ArgoCDDefaultArgoImage, argoproj.ArgoCDDefaultArgoVersion),
		ImagePullPolicy: corev1.PullAlways,
		Name:            "argocd-export",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "backup-storage",
				MountPath: "/backups",
			},
		},
	}}

	pod.RestartPolicy = corev1.RestartPolicyOnFailure
	pod.ServiceAccountName = "argocd-application-controller"
	pod.Volumes = []corev1.Volume{
		getArgoStorageVolume("backup-storage", cr),
	}

	return pod
}

func newPodTemplateSpec(cr *argoprojv1a1.ArgoCDExport) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    argoutil.DefaultLabels(cr.Name),
		},
		Spec: newExportPodSpec(cr),
	}
}

// reconcileCronJob will ensure that the CronJob for the ArgoCDExport is present.
func (r *ReconcileArgoCDExport) reconcileCronJob(cr *argoprojv1a1.ArgoCDExport) error {
	if cr.Spec.Storage == nil || cr.Spec.Storage.Local == nil {
		return nil // Do nothing if storage or local options not set
	}

	cj := newCronJob(cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, cj.Name, cj) {
		if *cr.Spec.Schedule != cj.Spec.Schedule {
			cj.Spec.Schedule = *cr.Spec.Schedule
			return r.client.Update(context.TODO(), cj)
		}
		return nil
	}

	cj.Spec.Schedule = *cr.Spec.Schedule

	job := newJob(cr)
	job.Spec.Template = newPodTemplateSpec(cr)

	cj.Spec.JobTemplate.Spec = job.Spec

	if err := controllerutil.SetControllerReference(cr, cj, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), cj)
}

// reconcileJob will ensure that the Job for the ArgoCDExport is present.
func (r *ReconcileArgoCDExport) reconcileJob(cr *argoprojv1a1.ArgoCDExport) error {
	if cr.Spec.Storage == nil || cr.Spec.Storage.Local == nil {
		return nil // Do nothing if storage or local options not set
	}

	job := newJob(cr)
	if argoutil.IsObjectFound(r.client, cr.Namespace, job.Name, job) {
		if job.Status.Succeeded > 0 && cr.Status.Phase != argoproj.ArgoCDStatusCompleted {
			// Mark status Phase as Complete
			cr.Status.Phase = argoproj.ArgoCDStatusCompleted
			r.client.Status().Update(context.TODO(), cr)

			// Delete PVC for export
		}
		return nil
	}

	job.Spec.Template = newPodTemplateSpec(cr)

	if err := controllerutil.SetControllerReference(cr, job, r.scheme); err != nil {
		return err
	}
	return r.client.Create(context.TODO(), job)
}
