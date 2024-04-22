package redis

import (
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/permissions"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// func TestReconcileRole(t *testing.T) {
// 	tests := []struct {
// 		name          string
// 		reconciler    *RedisReconciler
// 		expectedError bool
// 		expectedRole  *rbacv1.Role
// 	}{
// 		{
// 			name: "Role does not exist",
// 			reconciler: makeTestRedisReconciler(
// 				test.MakeTestArgoCD(nil),
// 			),
// 			expectedError: false,
// 			expectedRole:  getDesiredRole(),
// 		},
// 		{
// 			name: "Role drift",
// 			reconciler: makeTestRedisReconciler(
// 				test.MakeTestArgoCD(nil),
// 				test.MakeTestRole(getDesiredRole(),
// 					func(role *rbacv1.Role) {
// 						role.Name = "test-argocd-redis"
// 						// Modify some fields to simulate drift
// 						role.Rules = []rbacv1.PolicyRule{
// 							{
// 								APIGroups: []string{""},
// 								Resources: []string{"configmaps"},
// 								Verbs:     []string{"get", "list"},
// 							},
// 						}
// 					},
// 				),
// 			),
// 			expectedError: false,
// 			expectedRole:  getDesiredRole(),
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			tt.reconciler.varSetter()

// 			err := tt.reconciler.reconcileRole()
// 			assert.NoError(t, err)

// 			existing, err := permissions.GetRole("test-argocd-redis", test.TestNamespace, tt.reconciler.Client)

// 			if tt.expectedError {
// 				assert.Error(t, err, "Expected an error but got none.")
// 			} else {
// 				assert.NoError(t, err, "Expected no error but got one.")
// 			}

// 			if tt.expectedRole != nil {
// 				match := true

// 				// Check for partial match on relevant fields
// 				ftc := []argocdcommon.FieldToCompare{
// 					{
// 						Existing: existing.Labels,
// 						Desired:  tt.expectedRole.Labels,
// 					},
// 					{
// 						Existing: existing.Rules,
// 						Desired:  tt.expectedRole.Rules,
// 					},
// 				}
// 				argocdcommon.PartialMatch(ftc, &match)
// 				assert.True(t, match)
// 			}
// 		})
// 	}
// }

// func TestReconcileHARole(t *testing.T) {
// 	tests := []struct {
// 		name          string
// 		reconciler    *RedisReconciler
// 		expectedError bool
// 		expectedRole  *rbacv1.Role
// 	}{
// 		{
// 			name: "Role does not exist",
// 			reconciler: makeTestRedisReconciler(
// 				test.MakeTestArgoCD(nil,
// 					func(ac *argoproj.ArgoCD) {
// 						ac.Spec.HA.Enabled = true
// 					},
// 				),
// 			),
// 			expectedError: false,
// 			expectedRole:  getDesiredHARole(),
// 		},
// 		{
// 			name: "Role drift",
// 			reconciler: makeTestRedisReconciler(
// 				test.MakeTestArgoCD(nil,
// 					func(ac *argoproj.ArgoCD) {
// 						ac.Spec.HA.Enabled = true
// 					},
// 				),
// 				test.MakeTestRole(getDesiredRole(),
// 					func(role *rbacv1.Role) {
// 						role.Name = "test-argocd-redis-ha"
// 						// Modify some fields to simulate drift
// 						role.Rules = []rbacv1.PolicyRule{
// 							{
// 								APIGroups: []string{""},
// 								Resources: []string{"configmaps"},
// 								Verbs:     []string{"get", "list"},
// 							},
// 						}
// 					},
// 				),
// 			),
// 			expectedError: false,
// 			expectedRole:  getDesiredHARole(),
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			tt.reconciler.varSetter()

// 			err := tt.reconciler.reconcileHARole()
// 			assert.NoError(t, err)

// 			existing, err := permissions.GetRole("test-argocd-redis-ha", test.TestNamespace, tt.reconciler.Client)

// 			if tt.expectedError {
// 				assert.Error(t, err, "Expected an error but got none.")
// 			} else {
// 				assert.NoError(t, err, "Expected no error but got one.")
// 			}

// 			if tt.expectedRole != nil {
// 				match := true

// 				// Check for partial match on relevant fields
// 				ftc := []argocdcommon.FieldToCompare{
// 					{
// 						Existing: existing.Labels,
// 						Desired:  tt.expectedRole.Labels,
// 					},
// 					{
// 						Existing: existing.Rules,
// 						Desired:  tt.expectedRole.Rules,
// 					},
// 				}
// 				argocdcommon.PartialMatch(ftc, &match)
// 				assert.True(t, match)
// 			}
// 		})
// 	}
// }

func TestDeleteRole(t *testing.T) {
	tests := []struct {
		name          string
		reconciler    *RedisReconciler
		roleExist     bool
		expectedError bool
	}{
		{
			name: "Role exists",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestRole(nil),
			),
			roleExist:     true,
			expectedError: false,
		},
		{
			name: "Role does not exist",
			reconciler: makeTestRedisReconciler(
				test.MakeTestArgoCD(nil),
			),
			roleExist:     false,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := tt.reconciler.deleteRole(test.TestName, test.TestNamespace)

			if tt.roleExist {
				_, err := permissions.GetRole(test.TestName, test.TestNamespace, tt.reconciler.Client)
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

func getDesiredRole() *rbacv1.Role {
	return &rbacv1.Role{
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
		Rules: []rbacv1.PolicyRule{},
	}
}

func getDesiredHARole() *rbacv1.Role {
	return &rbacv1.Role{
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
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"",
				},
				Resources: []string{
					"endpoints",
				},
				Verbs: []string{
					"get",
				},
			},
		},
	}
}
