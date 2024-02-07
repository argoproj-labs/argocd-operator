package workloads

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func TestRequestSecret(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name          string
		deployReq     SecretRequest
		desiredSecret *corev1.Secret
		wantErr       bool
	}{
		{
			name: "request secret",
			deployReq: SecretRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				StringData: test.TestKVP,
				Type:       corev1.SecretTypeBasicAuth,
			},
			desiredSecret: test.MakeTestSecret(nil, func(s *corev1.Secret) {
				s.Name = test.TestName
				s.Namespace = test.TestNamespace
				s.Labels = test.TestKVP
				s.Annotations = test.TestKVP
				s.StringData = test.TestKVP
				s.Type = corev1.SecretTypeBasicAuth
			}),
			wantErr: false,
		},
		{
			name: "request secret, successful mutation",
			deployReq: SecretRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				StringData: test.TestKVP,
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredSecret: test.MakeTestSecret(nil, func(s *corev1.Secret) {
				s.Name = test.TestNameMutated
				s.Namespace = test.TestNamespace
				s.Labels = test.TestKVP
				s.Annotations = test.TestKVP
				s.StringData = test.TestKVPMutated
			}),
			wantErr: false,
		},
		{
			name: "request secret, failed mutation",
			deployReq: SecretRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredSecret: test.MakeTestSecret(nil, func(s *corev1.Secret) {
				s.Name = test.TestName
				s.Namespace = test.TestNamespace
				s.Labels = test.TestKVP
				s.Annotations = test.TestKVP
			}),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSecret, err := RequestSecret(test.deployReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredSecret, gotSecret)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateSecret(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredSecret := test.MakeTestSecret(nil, func(s *corev1.Secret) {
		s.TypeMeta = metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		}
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
		s.Labels = test.TestKVP
		s.Annotations = test.TestKVP
		s.StringData = test.TestKVP
	})
	err := CreateSecret(desiredSecret, testClient)
	assert.NoError(t, err)

	createdSecret := &corev1.Secret{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdSecret)

	assert.NoError(t, err)
	assert.Equal(t, desiredSecret, createdSecret)
}

func TestGetSecret(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestSecret(nil, func(s *corev1.Secret) {
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
		s.StringData = test.TestKVP
	})).Build()

	_, err := GetSecret(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetSecret(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListSecrets(t *testing.T) {
	secret1 := test.MakeTestSecret(nil, func(s *corev1.Secret) {
		s.Name = "secret-1"
		s.Labels[common.AppK8sKeyComponent] = "new-component-1"
		s.Namespace = test.TestNamespace
	})
	secret2 := test.MakeTestSecret(nil, func(s *corev1.Secret) {
		s.Name = "secret-2"
		s.Namespace = test.TestNamespace
	})
	secret3 := test.MakeTestSecret(nil, func(s *corev1.Secret) {
		s.Name = "secret-3"
		s.Labels[common.AppK8sKeyComponent] = "new-component-2"
		s.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(
		secret1, secret2, secret3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredSecrets := []string{"secret-1", "secret-3"}

	existingSecretList, err := ListSecrets(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingSecrets := []string{}
	for _, secret := range existingSecretList.Items {
		existingSecrets = append(existingSecrets, secret.Name)
	}
	sort.Strings(existingSecrets)

	assert.Equal(t, desiredSecrets, existingSecrets)
}

func TestUpdateSecret(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestSecret(nil, func(s *corev1.Secret) {
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
	})).Build()

	desiredSecret := test.MakeTestSecret(nil, func(s *corev1.Secret) {
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
		s.Data = map[string][]byte{
			"admin.password": []byte("testpassword2023"),
		}
	})
	err := UpdateSecret(desiredSecret, testClient)
	assert.NoError(t, err)

	existingSecret := &corev1.Secret{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingSecret)

	assert.NoError(t, err)
	assert.Equal(t, desiredSecret.Data, existingSecret.Data)

	testClient = fake.NewClientBuilder().Build()
	existingSecret = test.MakeTestSecret(nil, func(d *corev1.Secret) {
		d.Name = test.TestName
	})
	err = UpdateSecret(existingSecret, testClient)
	assert.Error(t, err)
}

func TestDeleteSecret(t *testing.T) {
	testSecret := test.MakeTestSecret(nil, func(s *corev1.Secret) {
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testSecret).Build()

	err := DeleteSecret(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingSecret := &corev1.Secret{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingSecret)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
