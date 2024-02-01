package reposerver

import (
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	"github.com/argoproj-labs/argocd-operator/pkg/workloads"
	"github.com/argoproj-labs/argocd-operator/tests/mock"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileTLSSecret(t *testing.T) {
	mockServerName := "test-argocd-server"
	mockAppControllerName := "test-argocd-app-controller"

	tests := []struct {
		name          string
		resources     []client.Object
		secretExist   bool
		expectedError bool
	}{
		{
			name: "no TLS secret",
			resources: []client.Object{
				test.MakeTestDeployment(nil,
					func(d *appsv1.Deployment) {
						d.Name = mockServerName
					},
				),
				test.MakeTestDeployment(nil,
					func(d *appsv1.Deployment) {
						d.Name = "test-argocd-repo-server"
					},
				),
				test.MakeTestStatefulSet(nil,
					func(d *appsv1.StatefulSet) {
						d.Name = mockAppControllerName
					},
				),
			},
			secretExist:   false,
			expectedError: false,
		},
		{
			name: "missing target deployments and statefulset",
			resources: []client.Object{
				test.MakeTestSecret(nil,
					func(s *corev1.Secret) {
						s.Name = "argocd-repo-server-tls"
						s.Type = corev1.SecretTypeTLS
						s.Data = map[string][]byte{
							"tls.crt": []byte(test.TestCert),
							"tls.key": []byte(test.TestKey),
						}
					},
				),
			},
			secretExist:   true,
			expectedError: true,
		},
		{
			name: "reconcile TLS secret",
			resources: []client.Object{
				test.MakeTestSecret(nil,
					func(s *corev1.Secret) {
						s.Name = "argocd-repo-server-tls"
						s.Type = corev1.SecretTypeTLS
						s.Data = map[string][]byte{
							"tls.crt": []byte(test.TestCert),
							"tls.key": []byte(test.TestKey),
						}
					},
				),
				test.MakeTestDeployment(nil,
					func(d *appsv1.Deployment) {
						d.Name = mockServerName
					},
				),
				test.MakeTestDeployment(nil,
					func(d *appsv1.Deployment) {
						d.Name = "test-argocd-repo-server"
					},
				),
				test.MakeTestStatefulSet(nil,
					func(d *appsv1.StatefulSet) {
						d.Name = mockAppControllerName
					},
				),
			},
			secretExist:   true,
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			reconciler := makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
				tt.resources...,
			)
			reconciler.TLSEnabled = tt.secretExist
			reconciler.varSetter()

			reconciler.Server = mock.NewServer(mockServerName, test.TestNamespace, reconciler.Client)
			reconciler.Appcontroller = mock.NewAppController(mockAppControllerName, test.TestNamespace, reconciler.Client)

			err := reconciler.reconcileTLSSecret()
			if !tt.secretExist {
				assert.NoError(t, err)
				return
			}

			if tt.expectedError {
				assert.Error(t, err, "Expected an error but got none.")
				return
			} else {
				assert.NoError(t, err, "Expected no error but got one.")
			}

			res, err := resource.GetObject(test.TestArgoCDName, test.TestNamespace, &argoproj.ArgoCD{}, reconciler.Client)
			assert.NoError(t, err)
			argocd := res.(*argoproj.ArgoCD)
			assert.NotEqual(t, "", argocd.Status.RepoTLSChecksum)

			dep, err := workloads.GetDeployment(mockServerName, test.TestNamespace, reconciler.Client)
			assert.NoError(t, err)
			_, ok := dep.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
			assert.True(t, ok)

			dep, err = workloads.GetDeployment(resourceName, test.TestNamespace, reconciler.Client)
			assert.NoError(t, err)
			_, ok = dep.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
			assert.True(t, ok)

			ss, err := workloads.GetStatefulSet(mockAppControllerName, test.TestNamespace, reconciler.Client)
			assert.NoError(t, err)
			_, ok = ss.Spec.Template.ObjectMeta.Labels["repo.tls.cert.changed"]
			assert.True(t, ok)

		})
	}
}

func TestDeleteSecret(t *testing.T) {
	tests := []struct {
		name          string
		reconciler    *RepoServerReconciler
		secretExist   bool
		expectedError bool
	}{
		{
			name: "Secret exists",
			reconciler: makeTestReposerverReconciler(
				test.MakeTestArgoCD(nil),
				test.MakeTestSecret(nil),
			),
			secretExist:   true,
			expectedError: false,
		},
		{
			name: "Secret does not exist",
			reconciler: makeTestReposerverReconciler(
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
