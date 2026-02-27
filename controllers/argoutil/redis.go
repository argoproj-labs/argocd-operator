package argoutil

import (
	corev1 "k8s.io/api/core/v1"

	argocdoperatorv1beta1 "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

const (
	RedisAuthVolumeName = "redis-initial-pass"
	RedisAuthMountPath  = "/app/config/redis-auth/"
)

func MountRedisAuthToArgo(cr *argocdoperatorv1beta1.ArgoCD) (volume corev1.Volume, mount corev1.VolumeMount) {
	volume = corev1.Volume{
		Name: RedisAuthVolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: GetSecretNameWithSuffix(cr, "redis-initial-password"),
				// Mapping the legacy key-name, the operator customers can depend on, to the expected file name for argo-cd
				// Ref.: https://argo-cd.readthedocs.io/en/latest/faq/#using-file-based-redis-credentials-via-redis_creds_dir_path
				Items: []corev1.KeyToPath{
					{
						Key:  "admin.password",
						Path: "auth",
					},
				},
			},
		},
	}

	mount = corev1.VolumeMount{
		Name:      RedisAuthVolumeName,
		MountPath: RedisAuthMountPath,
	}

	return
}

func GetRedisAuthEnv() []corev1.EnvVar {
	return []corev1.EnvVar{{
		Name:  "REDIS_CREDS_DIR_PATH",
		Value: RedisAuthMountPath,
	}}
}
