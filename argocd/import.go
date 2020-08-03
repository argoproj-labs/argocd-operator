// Copyright 2019 Argo CD Operator Developers
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
	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/resources"
	corev1 "k8s.io/api/core/v1"
)

func (r *ArgoClusterReconciler) getArgoImportBackend(cr *v1alpha1.ArgoCD) string {
	backend := common.ArgoCDExportStorageBackendLocal
	namespace := cr.ObjectMeta.Namespace
	if cr.Spec.Import != nil && cr.Spec.Import.Namespace != nil && len(*cr.Spec.Import.Namespace) > 0 {
		namespace = *cr.Spec.Import.Namespace
	}

	export := &v1alpha1.ArgoCDExport{}
	if resources.IsObjectFound(r.Client, namespace, cr.Spec.Import.Name, export) {
		if export.Spec.Storage != nil && len(export.Spec.Storage.Backend) > 0 {
			backend = export.Spec.Storage.Backend
		}
	}
	return backend
}

// getArgoImportCommand will return the command for the ArgoCD import process.
func (r *ArgoClusterReconciler) getArgoImportCommand(cr *v1alpha1.ArgoCD) []string {
	cmd := make([]string, 0)
	cmd = append(cmd, "uid_entrypoint.sh")
	cmd = append(cmd, "argocd-operator-util")
	cmd = append(cmd, "import")
	cmd = append(cmd, r.getArgoImportBackend(cr))
	return cmd
}

func getArgoImportContainerEnv(cr *v1alpha1.ArgoCDExport) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0)

	switch cr.Spec.Storage.Backend {
	case common.ArgoCDExportStorageBackendAWS:
		env = append(env, corev1.EnvVar{
			Name: "AWS_ACCESS_KEY_ID",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: getStorageSecretName(cr),
					},
					Key: "aws.access.key.id",
				},
			},
		})

		env = append(env, corev1.EnvVar{
			Name: "AWS_SECRET_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: getStorageSecretName(cr),
					},
					Key: "aws.secret.access.key",
				},
			},
		})
	}

	return env
}

// getArgoImportContainerImage will return the container image for the Argo CD import process.
func getArgoImportContainerImage(cr *v1alpha1.ArgoCDExport) string {
	img := common.ArgoCDDefaultExportJobImage
	if len(cr.Spec.Image) > 0 {
		img = cr.Spec.Image
	}

	tag := common.ArgoCDDefaultExportJobVersion
	if len(cr.Spec.Version) > 0 {
		tag = cr.Spec.Version
	}

	return common.CombineImageTag(img, tag)
}

// getArgoImportVolumeMounts will return the VolumneMounts for the given ArgoCDExport.
func getArgoImportVolumeMounts(cr *v1alpha1.ArgoCDExport) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0)

	mounts = append(mounts, corev1.VolumeMount{
		Name:      "backup-storage",
		MountPath: "/backups",
	})

	mounts = append(mounts, corev1.VolumeMount{
		Name:      "secret-storage",
		MountPath: "/secrets",
	})

	return mounts
}

// getArgoImportVolumes will return the Volumes for the given ArgoCDExport.
func getArgoImportVolumes(cr *v1alpha1.ArgoCDExport) []corev1.Volume {
	volumes := make([]corev1.Volume, 0)

	if cr.Spec.Storage != nil && cr.Spec.Storage.Backend == common.ArgoCDExportStorageBackendLocal {
		volumes = append(volumes, corev1.Volume{
			Name: "backup-storage",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: cr.Name,
				},
			},
		})
	} else {
		volumes = append(volumes, corev1.Volume{
			Name: "backup-storage",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	volumes = append(volumes, corev1.Volume{
		Name: "secret-storage",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: getArgoExportSecretName(cr),
			},
		},
	})

	return volumes
}
