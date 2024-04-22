package reposerver

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/argoproj-labs/argocd-operator/tests/mock"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestReconcileDeployment_create(t *testing.T) {
	mockRedisName := "test-argocd-redis"

	tests := []struct {
		name               string
		reconciler         *RepoServerReconciler
		expectedError      bool
		expectedDeployment *appsv1.Deployment
	}{
		{
			name: "Deployment does not exist",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedError:      false,
			expectedDeployment: getDesiredDeployment(),
		},
		{
			name: "Deployment exists",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestDeployment(getDesiredDeployment(),
					func(d *appsv1.Deployment) {
						d.Name = "test-argocd-repo-server"
					},
				),
			),
			expectedError:      false,
			expectedDeployment: getDesiredDeployment(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.varSetter()

			mockRedis := mock.NewRedis(mockRedisName, test.TestNamespace, tt.reconciler.Client)
			mockRedis.SetServerAddress("http://mock-server-address")
			mockRedis.SetUseTLS(true)
			tt.reconciler.Redis = mockRedis

			err := tt.reconciler.reconcileDeployment()
			assert.NoError(t, err)

			_, err = workloads.GetDeployment("test-argocd-repo-server", test.TestNamespace, tt.reconciler.Client)

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

		})
	}
}

func TestDeleteDeployment(t *testing.T) {
	tests := []struct {
		name            string
		reconciler      *RepoServerReconciler
		deploymentExist bool
		expectedError   bool
	}{
		{
			name: "Deployment exists",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestDeployment(nil),
			),
			deploymentExist: true,
			expectedError:   false,
		},
		{
			name: "Deployment does not exist",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			deploymentExist: false,
			expectedError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := tt.reconciler.deleteDeployment(test.TestName, test.TestNamespace)

			if tt.deploymentExist {
				_, err := workloads.GetDeployment(test.TestName, test.TestNamespace, tt.reconciler.Client)
				assert.True(t, apierrors.IsNotFound(err))
			}

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}
		})
	}
}

func getDesiredDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-argocd-repo-server",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":       "test-argocd-repo-server",
				"app.kubernetes.io/part-of":    "argocd",
				"app.kubernetes.io/instance":   "test-argocd",
				"app.kubernetes.io/managed-by": "argocd-operator",
				"app.kubernetes.io/component":  "repo-server",
			},
			Annotations: map[string]string{
				"argocds.argoproj.io/name":      "test-argocd",
				"argocds.argoproj.io/namespace": "test-ns",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": "test-argocd-repo-server",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": "test-argocd-repo-server",
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "ssh-known-hosts",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "argocd-ssh-known-hosts-cm",
									},
								},
							},
						},
						{
							Name: "tls-certs",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "argocd-tls-certs-cm",
									},
								},
							},
						},
						{
							Name: "gpg-keys",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "argocd-gpg-keys-cm",
									},
								},
							},
						},
						{
							Name: "gpg-keyring",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "tmp",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "argocd-repo-server-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "argocd-repo-server-tls",
									Optional:   util.BoolPtr(true),
								},
							},
						},
						{
							Name: "argocd-operator-redis-tls",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "argocd-operator-redis-tls",
									Optional:   util.BoolPtr(true),
								},
							},
						},
						{
							Name: "var-files",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "plugins",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:            "copyutil",
							Image:           "quay.io/argoproj/argocd@sha256:8576d347f30fa4c56a0129d1c0a0f5ed1e75662f0499f1ed7e917c405fd909dc",
							Command:         []string{"cp", "-n", "/usr/local/bin/argocd", "/var/run/argocd/argocd-cmp-server"},
							ImagePullPolicy: "Always",
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: util.BoolPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{
										"ALL",
									},
								},
								RunAsNonRoot: util.BoolPtr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "var-files",
									MountPath: "var/run/argocd",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "argocd-repo-server",
							Image:           "quay.io/argoproj/argocd@sha256:8576d347f30fa4c56a0129d1c0a0f5ed1e75662f0499f1ed7e917c405fd909dc",
							Command:         []string{"uid_entrypoint.sh", "argocd-repo-server", "--redis", "http://mock-server-address", "--redis-use-tls", "--redis-ca-certificate", "/app/config/reposerver/tls/redis/tls.crt", "--loglevel", "info", "--logformat", "text"},
							ImagePullPolicy: "Always",
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(8081),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(8081),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8081,
									Name:          "server",
								}, {
									ContainerPort: 8084,
									Name:          "metrics",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: util.BoolPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{
										"ALL",
									},
								},
								RunAsNonRoot: util.BoolPtr(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "ssh-known-hosts",
									MountPath: "/app/config/ssh",
								},
								{
									Name:      "tls-certs",
									MountPath: "/app/config/tls",
								},
								{
									Name:      "gpg-keys",
									MountPath: "/app/config/gpg/source",
								},
								{
									Name:      "gpg-keyring",
									MountPath: "/app/config/gpg/keys",
								},
								{
									Name:      "tmp",
									MountPath: "/tmp",
								},
								{
									Name:      "argocd-repo-server-tls",
									MountPath: "/app/config/reposerver/tls",
								},
								{
									Name:      "argocd-operator-redis-tls",
									MountPath: "/app/config/reposerver/tls/redis",
								},
								{
									Name:      "plugins",
									MountPath: "/home/argocd/cmp-server/plugins",
								},
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: util.BoolPtr(true),
					},
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					ServiceAccountName: "test-argocd-repo-server",
				},
			},
		},
	}
}
