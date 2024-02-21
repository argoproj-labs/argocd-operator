package redis

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/networking"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestReconcileService(t *testing.T) {
	tests := []struct {
		name            string
		reconciler      *RedisReconciler
		expectedError   bool
		expectedService *corev1.Service
	}{
		{
			name: "Service does not exist",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedError:   false,
			expectedService: getDesiredSvc(),
		},
		{
			name: "Service drift",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestService(getDesiredSvc(),
					func(svc *corev1.Service) {
						svc.Name = "test-argocd-redis"
						// Modify some fields to simulate drift
						svc.Spec.Ports = []corev1.ServicePort{
							{
								Name: "server",
								Port: 8087,
							},
						}
					},
				),
			),
			expectedError:   false,
			expectedService: getDesiredSvc(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.varSetter()

			err := tt.reconciler.reconcileService()
			assert.NoError(t, err)

			existing, err := networking.GetService("test-argocd-redis", test.TestNamespace, tt.reconciler.Client)

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

			if tt.expectedService != nil {
				match := true

				// Check for partial match on relevant fields
				ftc := []argocdcommon.FieldToCompare{
					{
						Existing: existing.Labels,
						Desired:  tt.expectedService.Labels,
					},
					{
						Existing: existing.Annotations,
						Desired:  tt.expectedService.Annotations,
					},
					{
						Existing: existing.Spec,
						Desired:  tt.expectedService.Spec,
					},
				}
				argocdcommon.PartialMatch(ftc, &match)
				assert.True(t, match)
			}

		})
	}
}

func TestReconcileHAProxyService(t *testing.T) {
	tests := []struct {
		name            string
		reconciler      *RedisReconciler
		expectedError   bool
		expectedService *corev1.Service
	}{
		{
			name: "Service does not exist",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedError:   false,
			expectedService: getDesiredHAProxySvc(),
		},
		{
			name: "Service drift",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestService(getDesiredHAProxySvc(),
					func(svc *corev1.Service) {
						svc.Name = "test-argocd-redis-ha-haproxy"
						// Modify some fields to simulate drift
						svc.Spec.Ports = []corev1.ServicePort{
							{
								Name: "server",
								Port: 8087,
							},
						}
					},
				),
			),
			expectedError:   false,
			expectedService: getDesiredHAProxySvc(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.varSetter()

			err := tt.reconciler.reconcileHAProxyService()
			assert.NoError(t, err)

			existing, err := networking.GetService("test-argocd-redis-ha-haproxy", test.TestNamespace, tt.reconciler.Client)

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

			if tt.expectedService != nil {
				match := true

				// Check for partial match on relevant fields
				ftc := []argocdcommon.FieldToCompare{
					{
						Existing: existing.Labels,
						Desired:  tt.expectedService.Labels,
					},
					{
						Existing: existing.Spec.Ports,
						Desired:  tt.expectedService.Spec.Ports,
					},
				}
				argocdcommon.PartialMatch(ftc, &match)
				assert.True(t, match)
			}

		})
	}
}

func TestDeleteService(t *testing.T) {
	tests := []struct {
		name          string
		reconciler    *RedisReconciler
		serviceExist  bool
		expectedError bool
	}{
		{
			name: "Service exists",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestService(nil),
			),
			serviceExist:  true,
			expectedError: false,
		},
		{
			name: "Service does not exist",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			serviceExist:  false,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := tt.reconciler.deleteService(test.TestName, test.TestNamespace)

			if tt.serviceExist {
				_, err := networking.GetService(test.TestName, test.TestNamespace, tt.reconciler.Client)
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

func getDesiredSvc() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-argocd-redis",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":       "test-argocd-redis",
				"app.kubernetes.io/part-of":    "argocd",
				"app.kubernetes.io/instance":   "test-argocd",
				"app.kubernetes.io/managed-by": "argocd-operator",
				"app.kubernetes.io/component":  "redis",
			},
			Annotations: map[string]string{
				"argocds.argoproj.io/name":      "test-argocd",
				"argocds.argoproj.io/namespace": "test-ns",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name": "test-argocd-redis",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "tcp-redis",
					Port:       6379,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(6379),
				},
			},
		},
	}
}

func getDesiredHAProxySvc() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-argocd-redis",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":       "test-argocd-redis-ha-haproxy",
				"app.kubernetes.io/part-of":    "argocd",
				"app.kubernetes.io/instance":   "test-argocd",
				"app.kubernetes.io/managed-by": "argocd-operator",
				"app.kubernetes.io/component":  "redis",
			},
			Annotations: map[string]string{
				"argocds.argoproj.io/name":      "test-argocd",
				"argocds.argoproj.io/namespace": "test-ns",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name": "test-argocd-redis-ha-haproxy",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "haproxy",
					Port:       6379,
					Protocol:   "TCP",
					TargetPort: intstr.FromString("redis"),
				},
			},
		},
	}
}

func getDesiredHAMasterSvc() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-argocd-redis-ha",
			Namespace: "test-ns",
			Labels: map[string]string{
				"app.kubernetes.io/name":       "test-argocd-redis-ha",
				"app.kubernetes.io/part-of":    "argocd",
				"app.kubernetes.io/instance":   "test-argocd",
				"app.kubernetes.io/managed-by": "argocd-operator",
				"app.kubernetes.io/component":  "redis",
			},
			Annotations: map[string]string{
				"argocds.argoproj.io/name":      "test-argocd",
				"argocds.argoproj.io/namespace": "test-ns",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app.kubernetes.io/name": "test-argocd-redis-ha",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "server",
					Port:       6379,
					Protocol:   "TCP",
					TargetPort: intstr.FromString("redis"),
				}, {
					Name:       "sentinel",
					Port:       26379,
					Protocol:   "TCP",
					TargetPort: intstr.FromString("sentinel"),
				},
			},
		},
	}
}
