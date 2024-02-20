package argocd

import (
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_reconcileSecrets(t *testing.T) {
	testArgoCD := test.MakeTestArgoCD(nil)
	reconciler := makeTestArgoCDReconciler(
		testArgoCD,
	)

	expectedResources := []client.Object{
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "argocd-secret"
			},
		),
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-cluster"
			},
		),
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-tls"
			},
		),
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-ca"
			},
		),
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-default-cluster-config"
			},
		),
	}

	reconciler.varSetter()
	reconciler.secretVarSetter()

	err := reconciler.reconcileSecrets()
	assert.NoError(t, err)

	for _, obj := range expectedResources {
		_, err := resource.GetObject(obj.GetName(), test.TestNamespace, obj, reconciler.Client)
		assert.NoError(t, err)
	}
}

func Test_deleteSecrets(t *testing.T) {
	testArgoCD := test.MakeTestArgoCD(nil)

	resources := []client.Object{
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "argocd-secret"
			},
		),
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-cluster"
			},
		),
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-tls"
			},
		),
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-ca"
			},
		),
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-default-cluster-config"
			},
		),
	}

	reconciler := makeTestArgoCDReconciler(
		testArgoCD,
		resources...,
	)

	reconciler.varSetter()
	reconciler.secretVarSetter()

	err := reconciler.deleteSecrets()
	assert.NoError(t, err)

	for _, obj := range resources {
		_, err := resource.GetObject(obj.GetName(), test.TestNamespace, obj, reconciler.Client)
		assert.True(t, apierrors.IsNotFound(err))
	}
}

func Test_reconcileArgoCDSecret(t *testing.T) {
	testArgoCD := test.MakeTestArgoCD(nil)

	resources := []client.Object{
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-cluster"
				s.Data = map[string][]byte{
					"admin.password": []byte(test.TestVal),
				}
			},
		),
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-tls"
				s.Data = map[string][]byte{
					"tls.crt": []byte(test.TestCert),
					"tls.key": []byte(test.TestKey),
				}
			},
		),
	}

	reconciler := makeTestArgoCDReconciler(
		testArgoCD,
		resources...,
	)

	reconciler.varSetter()
	reconciler.secretVarSetter()

	expectedKeys := []string{"server.secretkey", "admin.password", "admin.passwordMtime", "tls.crt", "tls.key"}

	err := reconciler.reconcileArgoCDSecret()
	assert.NoError(t, err)

	existing, err := workloads.GetSecret("argocd-secret", test.TestNamespace, reconciler.Client)
	assert.NoError(t, err)

	sort.Strings(expectedKeys)
	assert.Equal(t, expectedKeys, util.ByteMapKeys(existing.Data))

	// introduce drift in TLS secret
	tlsSecret := test.MakeTestSecret(nil,
		func(s *corev1.Secret) {
			s.Name = "test-argocd-tls"
			s.Data = map[string][]byte{
				"tls.crt": []byte(test.TestKey),
				"tls.key": []byte(test.TestVal),
			}
		},
	)
	workloads.UpdateSecret(tlsSecret, reconciler.Client)

	err = reconciler.reconcileArgoCDSecret()
	assert.NoError(t, err)

	existing, err = workloads.GetSecret("argocd-secret", test.TestNamespace, reconciler.Client)
	assert.NoError(t, err)

	assert.Equal(t, test.TestKey, string(existing.Data["tls.crt"]))
	assert.Equal(t, test.TestVal, string(existing.Data["tls.key"]))
}

func Test_reconcileClusterPermissionsSecret(t *testing.T) {
	testArgoCD := test.MakeTestArgoCD(nil)
	reconciler := makeTestArgoCDReconciler(
		testArgoCD,
	)

	reconciler.ResourceManagedNamespaces = map[string]string{"test-ns-1": "", "test-ns-2": "", "test-ns-3": ""}
	reconciler.ClusterScoped = false

	reconciler.varSetter()
	reconciler.secretVarSetter()

	err := reconciler.reconcileClusterPermissionsSecret()
	assert.NoError(t, err)

	existing, err := workloads.GetSecret("test-argocd-default-cluster-config", test.TestNamespace, reconciler.Client)
	assert.NoError(t, err)

	assert.Equal(t, "test-ns-1,test-ns-2,test-ns-3", string(existing.Data["namespaces"]))

	// update managed ns list
	reconciler.ResourceManagedNamespaces = map[string]string{"test-ns-4": "", "test-ns-5": "", "test-ns-3": ""}

	err = reconciler.reconcileClusterPermissionsSecret()
	assert.NoError(t, err)

	existing, err = workloads.GetSecret("test-argocd-default-cluster-config", test.TestNamespace, reconciler.Client)
	assert.NoError(t, err)

	assert.Equal(t, "test-ns-3,test-ns-4,test-ns-5", string(existing.Data["namespaces"]))

	// set instance to cluster-scoped
	reconciler.ClusterScoped = true

	err = reconciler.reconcileClusterPermissionsSecret()
	assert.NoError(t, err)

	existing, err = workloads.GetSecret("test-argocd-default-cluster-config", test.TestNamespace, reconciler.Client)
	assert.NoError(t, err)

	assert.Equal(t, "", string(existing.Data["namespaces"]))

}

func Test_reconcileAdminCredsSecret(t *testing.T) {
	tests := []struct {
		name          string
		reconciler    *ArgoCDReconciler
		expectedError bool
		expectedKeys  []string
		expectedValue string
	}{
		{
			name: "default admin credentials secret",
			reconciler: makeTestArgoCDReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedError: false,
			expectedKeys:  []string{"admin.password"},
			expectedValue: "",
		},
		{
			name: "admin credentials secret drift",
			reconciler: makeTestArgoCDReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestSecret(nil,
					func(s *corev1.Secret) {
						s.Name = "test-argocd-cluster"
						s.Data = map[string][]byte{
							"admin.password": []byte(test.TestVal),
						}
					},
				),
			),
			expectedError: false,
			expectedKeys:  []string{"admin.password"},
			expectedValue: test.TestVal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.varSetter()
			tt.reconciler.secretVarSetter()

			err := tt.reconciler.reconcileAdminCredentialsSecret()
			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

			existing, err := workloads.GetSecret("test-argocd-cluster", test.TestNamespace, tt.reconciler.Client)

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

			if len(tt.expectedKeys) > 0 {
				sort.Strings(tt.expectedKeys)
				assert.Equal(t, tt.expectedKeys, util.ByteMapKeys(existing.Data))

				if tt.expectedValue != "" {
					assert.Equal(t, tt.expectedValue, string(existing.Data["admin.password"]))
				}
			}
		},
		)
	}
}

func Test_reconcileTLSSecret(t *testing.T) {
	tests := []struct {
		name          string
		reconciler    *ArgoCDReconciler
		expectedError bool
		expectedKeys  []string
	}{
		{
			name: "no CA secret found",
			reconciler: makeTestArgoCDReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedError: true,
			expectedKeys:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.varSetter()
			tt.reconciler.secretVarSetter()

			err := tt.reconciler.reconcileTLSSecret()
			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

			existing, err := workloads.GetSecret("test-argocd-tls", test.TestNamespace, tt.reconciler.Client)

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

			if len(tt.expectedKeys) > 0 {
				sort.Strings(tt.expectedKeys)
				assert.Equal(t, tt.expectedKeys, util.ByteMapKeys(existing.Data))
			}
		},
		)
	}
}

func Test_reconcileCASecret(t *testing.T) {
	tests := []struct {
		name          string
		reconciler    *ArgoCDReconciler
		expectedError bool
		expectedKeys  []string
	}{
		{
			name: "default CA secret",
			reconciler: makeTestArgoCDReconciler(
				test.MakeTestArgoCD(nil),
			),
			expectedError: false,
			expectedKeys:  []string{"tls.crt", "ca.crt", "tls.key"},
		},
		{
			name: "CA secret drift",
			reconciler: makeTestArgoCDReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestSecret(nil,
					func(s *corev1.Secret) {
						s.Name = "test-argocd-ca"
						s.Data = map[string][]byte{
							"tls.crt": []byte(test.TestKey),
							"tls.key": []byte(test.TestVal),
						}
					},
				),
			),
			expectedError: false,
			expectedKeys:  []string{"tls.crt", "tls.key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.reconciler.varSetter()
			tt.reconciler.secretVarSetter()

			err := tt.reconciler.reconcileCASecret()
			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

			existing, err := workloads.GetSecret("test-argocd-ca", test.TestNamespace, tt.reconciler.Client)

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

			if len(tt.expectedKeys) > 0 {
				sort.Strings(tt.expectedKeys)
				assert.Equal(t, tt.expectedKeys, util.ByteMapKeys(existing.Data))
			}
		},
		)
	}
}

func TestDeleteSecret(t *testing.T) {
	tests := []struct {
		name          string
		reconciler    *ArgoCDReconciler
		secretExist   bool
		expectedError bool
	}{
		{
			name: "Secret exists",
			reconciler: makeTestArgoCDReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestSecret(nil),
			),
			secretExist:   true,
			expectedError: false,
		},
		{
			name: "Secret does not exist",
			reconciler: makeTestArgoCDReconciler(
				test.MakeTestArgoCD(nil),
			),
			secretExist:   false,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.reconciler.deleteSecret(test.TestName, test.TestNamespace)

			if tt.secretExist {
				_, err := workloads.GetSecret(test.TestName, test.TestNamespace, tt.reconciler.Client)
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
