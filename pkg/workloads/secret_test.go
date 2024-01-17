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
)

type secretOpt func(*corev1.Secret)

func getTestSecret(opts ...secretOpt) *corev1.Secret {
	desiredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		StringData: map[string]string{},
	}

	for _, opt := range opts {
		opt(desiredSecret)
	}
	return desiredSecret
}

func TestRequestSecret(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name          string
		deployReq     SecretRequest
		desiredSecret *corev1.Secret
		wantErr       bool
	}{
		{
			name: "request secret, no mutation, custom name, labels, annotations",
			deployReq: SecretRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				StringData: testKVP,
				Type:       corev1.SecretTypeBasicAuth,
			},
			desiredSecret: getTestSecret(func(s *corev1.Secret) {
				s.Name = testName
				s.Namespace = testNamespace
				s.Labels = testKVP
				s.Annotations = testKVP
				s.StringData = testKVP
				s.Type = corev1.SecretTypeBasicAuth
			}),
			wantErr: false,
		},
		{
			name: "request secret, successful mutation",
			deployReq: SecretRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				StringData: testKVP,
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredSecret: getTestSecret(func(s *corev1.Secret) {
				s.Name = testNameMutated
				s.Namespace = testNamespace
				s.Labels = testKVP
				s.Annotations = testKVP
				s.StringData = testKVPMutated
			}),
			wantErr: false,
		},
		{
			name: "request secret, failed mutation",
			deployReq: SecretRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredSecret: getTestSecret(func(s *corev1.Secret) {
				s.Name = testName
				s.Namespace = testNamespace
				s.Labels = testKVP
				s.Annotations = testKVP
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

	desiredSecret := getTestSecret(func(s *corev1.Secret) {
		s.TypeMeta = metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		}
		s.Name = testName
		s.Namespace = testNamespace
		s.Labels = testKVP
		s.Annotations = testKVP
		s.StringData = testKVP
	})
	err := CreateSecret(desiredSecret, testClient)
	assert.NoError(t, err)

	createdSecret := &corev1.Secret{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdSecret)

	assert.NoError(t, err)
	assert.Equal(t, desiredSecret, createdSecret)
}

func TestGetSecret(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestSecret(func(s *corev1.Secret) {
		s.Name = testName
		s.Namespace = testNamespace
		s.StringData = testKVP
	})).Build()

	_, err := GetSecret(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetSecret(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListSecrets(t *testing.T) {
	secret1 := getTestSecret(func(s *corev1.Secret) {
		s.Name = "secret-1"
		s.Labels[common.AppK8sKeyComponent] = "new-component-1"
		s.Namespace = testNamespace
	})
	secret2 := getTestSecret(func(s *corev1.Secret) {
		s.Name = "secret-2"
		s.Namespace = testNamespace
	})
	secret3 := getTestSecret(func(s *corev1.Secret) {
		s.Name = "secret-3"
		s.Labels[common.AppK8sKeyComponent] = "new-component-2"
		s.Namespace = testNamespace
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

	existingSecretList, err := ListSecrets(testNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingSecrets := []string{}
	for _, secret := range existingSecretList.Items {
		existingSecrets = append(existingSecrets, secret.Name)
	}
	sort.Strings(existingSecrets)

	assert.Equal(t, desiredSecrets, existingSecrets)
}

func TestUpdateSecret(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestSecret(func(s *corev1.Secret) {
		s.Name = testName
		s.Namespace = testNamespace
	})).Build()

	desiredSecret := getTestSecret(func(s *corev1.Secret) {
		s.Name = testName
		s.Namespace = testNamespace
		s.Data = map[string][]byte{
			"admin.password": []byte("testpassword2023"),
		}
	})
	err := UpdateSecret(desiredSecret, testClient)
	assert.NoError(t, err)

	existingSecret := &corev1.Secret{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingSecret)

	assert.NoError(t, err)
	assert.Equal(t, desiredSecret.Data, existingSecret.Data)

	testClient = fake.NewClientBuilder().Build()
	existingSecret = getTestSecret(func(d *corev1.Secret) {
		d.Name = testName
	})
	err = UpdateSecret(existingSecret, testClient)
	assert.Error(t, err)
}

func TestDeleteSecret(t *testing.T) {
	testSecret := getTestSecret(func(s *corev1.Secret) {
		s.Name = testName
		s.Namespace = testNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testSecret).Build()

	err := DeleteSecret(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingSecret := &corev1.Secret{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingSecret)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
