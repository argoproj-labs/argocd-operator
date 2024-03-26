package redis

import (
	"os"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestTLSVerificationDisabled(t *testing.T) {
	tests := []struct {
		name           string
		reconciler     *RedisReconciler
		expectedResult bool
	}{
		{
			name: "TLS Verification Disabled",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Redis.DisableTLSVerification = true
					},
				),
			),
			expectedResult: true,
		},
		{
			name: "TLS Verification Not Disabled",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Redis.DisableTLSVerification = false
					},
				),
			),
			expectedResult: false,
		},
		{
			name: "Default (TLS Verification Not Specified)",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.reconciler.TLSVerificationDisabled()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestUseTLS(t *testing.T) {
	tests := []struct {
		name           string
		reconciler     *RedisReconciler
		expectedResult bool
	}{
		{
			name: "no TLS secret found",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedResult: false,
		},
		{
			name: "secret not of type TLS",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestSecret(nil,
					func(s *corev1.Secret) {
						s.Name = "argocd-operator-redis-tls"
						s.Type = corev1.SecretTypeBasicAuth
					},
				),
			),
			expectedResult: false,
		},
		{
			name: "TLS secret with no owner instance found",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestSecret(nil,
					func(s *corev1.Secret) {
						s.Name = "argocd-operator-redis-tls"
						s.Type = corev1.SecretTypeTLS
					},
				),
			),
			expectedResult: false,
		},
		{
			name: "TLS secret with owner Instance found",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestSecret(nil,
					func(s *corev1.Secret) {
						s.Name = "argocd-operator-redis-tls"
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

func TestGetServerAddress(t *testing.T) {
	tests := []struct {
		name            string
		instance        *argoproj.ArgoCD
		expectedAddress string
	}{
		{
			name: "non HA mode",
			instance: test.MakeTestArgoCD(nil,
				func(cr *argoproj.ArgoCD) {
					cr.Spec.HA.Enabled = false
				},
			),
			expectedAddress: "test-argocd-redis.test-ns.svc.cluster.local:6379",
		},
		{
			name: "HA mode",
			instance: test.MakeTestArgoCD(nil,
				func(cr *argoproj.ArgoCD) {
					cr.Spec.HA.Enabled = true
				},
			),
			expectedAddress: "test-argocd-redis-ha-haproxy.test-ns.svc.cluster.local:6379",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := makeTestRedisReconciler(tt.instance)
			r.varSetter()

			got := r.GetServerAddress()

			assert.Equal(t, tt.expectedAddress, got)
		})
	}
}

func TestGetContainerImage(t *testing.T) {
	tests := []struct {
		name            string
		reconciler      *RedisReconciler
		setEnvVarFunc   func()
		UnsetEnvVarFunc func()
		expectedResult  string
	}{
		{
			name: "CR Spec Specifies Image",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Redis.Image = "custom-image"
						cr.Spec.Redis.Version = "custom-version"
					},
				),
			),
			setEnvVarFunc: func() {
				os.Setenv("ARGOCD_REDIS_IMAGE", "default-argocd-redis-image:v1.0")
			},
			UnsetEnvVarFunc: func() {
				os.Unsetenv("ARGOCD_REDIS_IMAGE")
			},
			expectedResult: "custom-image:custom-version",
		},
		{
			name: "Env Var Specifies Image",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			setEnvVarFunc: func() {
				os.Setenv("ARGOCD_REDIS_IMAGE", "default-argocd-redis-image:v1.0")
			},
			UnsetEnvVarFunc: func() {
				os.Unsetenv("ARGOCD_REDIS_IMAGE")
			},
			expectedResult: "default-argocd-redis-image:v1.0",
		},
		{
			name: "Default Image and Tag Used",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			setEnvVarFunc:  nil,
			expectedResult: "redis@sha256:8061ca607db2a0c80010aeb5fc9bed0253448bc68711eaa14253a392f6c48280",
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

func TestGetHAContainerImage(t *testing.T) {
	tests := []struct {
		name            string
		reconciler      *RedisReconciler
		setEnvVarFunc   func()
		UnsetEnvVarFunc func()
		expectedResult  string
	}{
		{
			name: "CR Spec Specifies Image",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil,
					func(cr *argoproj.ArgoCD) {
						cr.Spec.Redis.Image = "custom-image"
						cr.Spec.Redis.Version = "custom-version"
					},
				),
			),
			setEnvVarFunc: func() {
				os.Setenv("ARGOCD_REDIS_HA_IMAGE", "default-argocd-redis-ha-image:v1.0")
			},
			UnsetEnvVarFunc: func() {
				os.Unsetenv("ARGOCD_REDIS_HA_IMAGE")
			},
			expectedResult: "custom-image:custom-version",
		},
		{
			name: "Env Var Specifies Image",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			setEnvVarFunc: func() {
				os.Setenv("ARGOCD_REDIS_HA_IMAGE", "default-argocd-redis-ha-image:v1.0")
			},
			UnsetEnvVarFunc: func() {
				os.Unsetenv("ARGOCD_REDIS_HA_IMAGE")
			},
			expectedResult: "default-argocd-redis-ha-image:v1.0",
		},
		{
			name: "Default Image and Tag Used",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			setEnvVarFunc:  nil,
			expectedResult: "redis@sha256:8061ca607db2a0c80010aeb5fc9bed0253448bc68711eaa14253a392f6c48280",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if specified
			if tt.setEnvVarFunc != nil {
				tt.setEnvVarFunc()
				defer tt.UnsetEnvVarFunc()
			}

			result := tt.reconciler.getHAContainerImage()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestGetArgs(t *testing.T) {
	tests := []struct {
		name        string
		TLSEnabled  bool
		expectedCmd []string
	}{
		{
			name:        "TLS disabled",
			TLSEnabled:  false,
			expectedCmd: []string{"--save", "", "--appendonly", "no"},
		},
		{
			name:        "TLS disabled",
			TLSEnabled:  true,
			expectedCmd: []string{"--save", "", "--appendonly", "no", "--tls-port", "6379", "--port", "0", "--tls-cert-file", "/app/config/redis/tls/tls.crt", "--tls-key-file", "/app/config/redis/tls/tls.key", "--tls-auth-clients", "no"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := makeTestRedisReconciler(test.MakeTestArgoCD(nil))
			reconciler.TLSEnabled = tt.TLSEnabled

			gotArgs := reconciler.getCmd()

			assert.Equal(t, tt.expectedCmd, gotArgs)
		})
	}
}
