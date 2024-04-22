package reposerver

import (
	"os"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/tests/mock"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestUseTLS(t *testing.T) {
	tests := []struct {
		name           string
		reconciler     *RepoServerReconciler
		expectedResult bool
	}{
		{
			name: "no TLS secret found",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedResult: false,
		},
		{
			name: "secret not of type TLS",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestSecret(nil,
					func(s *corev1.Secret) {
						s.Name = "argocd-repo-server-tls"
						s.Type = corev1.SecretTypeBasicAuth
					},
				),
			),
			expectedResult: false,
		},
		{
			name: "TLS secret with no owner instance found",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestSecret(nil,
					func(s *corev1.Secret) {
						s.Name = "argocd-repo-server-tls"
						s.Type = corev1.SecretTypeTLS
					},
				),
			),
			expectedResult: false,
		},
		{
			name: "TLS secret with owner Instance found",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestSecret(nil,
					func(s *corev1.Secret) {
						s.Name = "argocd-repo-server-tls"
						s.Type = corev1.SecretTypeTLS
						s.Annotations["argocds.argoproj.io/name"] = test.TestArgoCDName
						s.Annotations["argocds.argoproj.io/namespace"] = test.TestNamespace
					},
				),
			),
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.UseTLS()
			assert.Equal(t, tt.expectedResult, tt.reconciler.TLSEnabled)
		})
	}
}

func TestTLSVerificationRequested(t *testing.T) {
	tests := []struct {
		name           string
		reconciler     *RepoServerReconciler
		expectedResult bool
	}{
		{
			name: "TLS Verification Requested",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Repo.VerifyTLS = true
					},
				),
			),
			expectedResult: true,
		},
		{
			name: "TLS Verification Not Requested",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Repo.VerifyTLS = false
					},
				),
			),
			expectedResult: false,
		},
		{
			name: "Default (TLS Verification Not Specified)",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.reconciler.TLSVerificationRequested()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetResources(t *testing.T) {
	tests := []struct {
		name           string
		reconciler     *RepoServerReconciler
		expectedResult corev1.ResourceRequirements
	}{
		{
			name: "Resource Requirements Specified",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Repo.Resources = &corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("200m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						}
					},
				),
			),
			expectedResult: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
		},
		{
			name: "Default (Resource Requirements Not Specified)",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedResult: corev1.ResourceRequirements{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.reconciler.getResources()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetContainerImage(t *testing.T) {
	tests := []struct {
		name            string
		reconciler      *RepoServerReconciler
		setEnvVarFunc   func()
		UnsetEnvVarFunc func()
		expectedResult  string
	}{
		{
			name: "CR Spec Specifies Image",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Repo.Image = "custom-image"
						cr.Spec.Repo.Version = "custom-version"
					},
				),
			),
			setEnvVarFunc: func() {
				os.Setenv("ARGOCD_IMAGE", "default-argocd-img:v1.0")
			},
			UnsetEnvVarFunc: func() {
				os.Unsetenv("ARGOCD_IMAGE")
			},
			expectedResult: "custom-image:custom-version",
		},
		{
			name: "Env Var Specifies Image",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			setEnvVarFunc: func() {
				os.Setenv("ARGOCD_IMAGE", "default-argocd-img:v1.0")
			},
			UnsetEnvVarFunc: func() {
				os.Unsetenv("ARGOCD_IMAGE")
			},
			expectedResult: "default-argocd-img:v1.0",
		},
		{
			name: "Default Image and Tag Used",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			setEnvVarFunc:  nil,
			expectedResult: "quay.io/argoproj/argocd@sha256:5cfead7ae4c50884873c042250d51373f3a8904a210f3ab6d88fcebfcfb0c03a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if specified
			if tt.setEnvVarFunc != nil {
				tt.setEnvVarFunc()
				defer tt.UnsetEnvVarFunc()
			}

			result := tt.reconciler.getContainerImage()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetServerAddress(t *testing.T) {
	tests := []struct {
		name           string
		reconciler     *RepoServerReconciler
		expectedResult string
	}{
		{
			name: "Custom Remote Specified",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Repo.Remote = util.StringPtr("https://custom.repo.server")
					},
				),
			),
			expectedResult: "https://custom.repo.server",
		},
		{
			name: "Default (Remote Not Specified)",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedResult: "test-argocd-repo-server.test-ns.svc.cluster.local:8081",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.varSetter()
			result := tt.reconciler.GetServerAddress()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetReplicas(t *testing.T) {
	tests := []struct {
		name           string
		reconciler     *RepoServerReconciler
		expectedResult *int32
	}{
		{
			name: "Replicas Specified",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						replicas := int32(3)
						cr.Spec.Repo.Replicas = &replicas
					},
				),
			),
			expectedResult: util.Int32Ptr(3),
		},
		{
			name: "Negative Replicas Specified",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						replicas := int32(-1)
						cr.Spec.Repo.Replicas = &replicas
					},
				),
			),
			expectedResult: nil,
		},
		{
			name: "Default (Replicas Not Specified)",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedResult: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.reconciler.getReplicas()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetArgs(t *testing.T) {

	tests := []struct {
		name        string
		reconciler  *RepoServerReconciler
		useTLS      bool
		expectedCmd []string
	}{
		{
			name: "redis disabled",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Redis.Enabled = util.BoolPtr(false)
					},
				),
			),
			useTLS:      false,
			expectedCmd: []string{"uid_entrypoint.sh", "argocd-repo-server", "--loglevel", "info", "--logformat", "text"},
		},
		{
			name: "redis enabled, UseTLS true, disable TLS verification true",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Redis.Enabled = util.BoolPtr(true)
						cr.Spec.Redis.DisableTLSVerification = true
					},
				),
			),
			useTLS:      true,
			expectedCmd: []string{"uid_entrypoint.sh", "argocd-repo-server", "--redis", "http://mock-redis-server", "--redis-use-tls", "--redis-insecure-skip-tls-verify", "--loglevel", "info", "--logformat", "text"},
		},
		{
			name: "redis enabled, UseTLS true, disable TLS verification false",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Redis.Enabled = util.BoolPtr(true)
						cr.Spec.Redis.DisableTLSVerification = false
					},
				),
			),
			useTLS:      true,
			expectedCmd: []string{"uid_entrypoint.sh", "argocd-repo-server", "--redis", "http://mock-redis-server", "--redis-use-tls", "--redis-ca-certificate", "/app/config/reposerver/tls/redis/tls.crt", "--loglevel", "info", "--logformat", "text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRedisName := "test-argocd-redis"
			mockRedis := mock.NewRedis(mockRedisName, test.TestNamespace, tt.reconciler.Client)
			mockRedis.SetUseTLS(tt.useTLS)
			mockRedis.SetServerAddress("http://mock-redis-server")
			tt.reconciler.Redis = mockRedis
			gotArgs := tt.reconciler.getArgs()

			assert.Equal(t, tt.expectedCmd, gotArgs)

		})
	}
}
