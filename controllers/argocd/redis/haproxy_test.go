package redis

import (
	"os"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
)

func Test_GetHAProxyAddress(t *testing.T) {
	r := makeTestRedisReconciler(test.MakeTestArgoCD(nil))
	r.varSetter()

	got := r.GetHAProxyAddress()

	assert.Equal(t, "test-argocd-redis-ha-haproxy.test-ns.svc.cluster.local:6379", got)

}

func TestGetHAProxyContainerImage(t *testing.T) {
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
						cr.Spec.HA.RedisProxyImage = "custom-image"
						cr.Spec.HA.RedisProxyVersion = "custom-version"
					},
				),
			),
			setEnvVarFunc: func() {
				os.Setenv("ARGOCD_REDIS_HA_PROXY_IMAGE", "default-argocd-redis-haproxy-img:v1.0")
			},
			UnsetEnvVarFunc: func() {
				os.Unsetenv("ARGOCD_REDIS_HA_PROXY_IMAGE")
			},
			expectedResult: "custom-image:custom-version",
		},
		{
			name: "Env Var Specifies Image",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			setEnvVarFunc: func() {
				os.Setenv("ARGOCD_REDIS_HA_PROXY_IMAGE", "default-argocd-redis-haproxy-img:v1.0")
			},
			UnsetEnvVarFunc: func() {
				os.Unsetenv("ARGOCD_REDIS_HA_PROXY_IMAGE")
			},
			expectedResult: "default-argocd-redis-haproxy-img:v1.0",
		},
		{
			name: "Default Image and Tag Used",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			setEnvVarFunc:  nil,
			expectedResult: "haproxy@sha256:7392fbbbb53e9e063ca94891da6656e6062f9d021c0e514888a91535b9f73231",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if specified
			if tt.setEnvVarFunc != nil {
				tt.setEnvVarFunc()
				defer tt.UnsetEnvVarFunc()
			}

			result := tt.reconciler.getHAProxyContainerImage()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
