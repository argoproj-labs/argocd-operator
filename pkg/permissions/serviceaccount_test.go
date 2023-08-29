package permissions

import (
	"context"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type serviceAccountOpt func(*corev1.ServiceAccount)

func getTestServiceAccount(opts ...serviceAccountOpt) *corev1.ServiceAccount {
	desiredServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
	}

	for _, opt := range opts {
		opt(desiredServiceAccount)
	}

	return desiredServiceAccount
}

func TestRequestServiceAccount(t *testing.T) {
	tests := []struct {
		name      string
		saReq     ServiceAccountRequest
		desiredSa *corev1.ServiceAccount
	}{
		{
			name: "request service account, custom name, labels, annotations",
			saReq: ServiceAccountRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
			},
			desiredSa: getTestServiceAccount(func(sa *corev1.ServiceAccount) {
				sa.Name = testName
				sa.Namespace = testNamespace
				sa.Labels = argoutil.MergeMaps(sa.Labels, testKVP)
				sa.Annotations = argoutil.MergeMaps(sa.Annotations, testKVP)
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSa := RequestServiceAccount(test.saReq)
			assert.Equal(t, test.desiredSa, gotSa)
		})
	}
}

func TestCreateServiceAccount(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredServiceAccount := getTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.TypeMeta = metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		}
		sa.Name = testName
		sa.Namespace = testNamespace
		sa.Labels = testKVP
		sa.Annotations = testKVP
	})
	err := CreateServiceAccount(desiredServiceAccount, testClient)
	assert.NoError(t, err)

	createdServiceAccount := &corev1.ServiceAccount{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdServiceAccount)

	assert.NoError(t, err)
	assert.Equal(t, desiredServiceAccount, createdServiceAccount)
}

func TestGetServiceAccount(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = testName
		sa.Namespace = testNamespace
	})).Build()

	_, err := GetServiceAccount(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetServiceAccount(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestListServiceAccounts(t *testing.T) {
	sa1 := getTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = "sa-1"
		sa.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	sa2 := getTestServiceAccount(func(sa *corev1.ServiceAccount) { sa.Name = "sa-2" })
	sa3 := getTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = "sa-3"
		sa.Labels[common.AppK8sKeyComponent] = "new-component-2"
	})

	testClient := fake.NewClientBuilder().WithObjects(
		sa1, sa2, sa3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]ctrlClient.ListOption, 0)
	listOpts = append(listOpts, ctrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredServiceAccounts := []string{"sa-1", "sa-3"}

	existingServiceAccountList, err := ListServiceAccounts(testNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingServiceAccounts := []string{}
	for _, sa := range existingServiceAccountList.Items {
		existingServiceAccounts = append(existingServiceAccounts, sa.Name)
	}
	sort.Strings(existingServiceAccounts)

	assert.Equal(t, desiredServiceAccounts, existingServiceAccounts)
}

func TestUpdateServiceAccount(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = testName
		sa.Namespace = testNamespace
	})).Build()

	desiredServiceAccount := getTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = testName
		sa.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: "new-secret",
			},
		}
		sa.Namespace = testNamespace
	})

	err := UpdateServiceAccount(desiredServiceAccount, testClient)
	assert.NoError(t, err)

	existingServiceAccount := &corev1.ServiceAccount{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingServiceAccount)

	assert.NoError(t, err)
	assert.Equal(t, desiredServiceAccount.ImagePullSecrets, existingServiceAccount.ImagePullSecrets)
	assert.Equal(t, desiredServiceAccount.AutomountServiceAccountToken, existingServiceAccount.AutomountServiceAccountToken)

	testClient = fake.NewClientBuilder().Build()
	existingServiceAccount = getTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = testName
	})
	err = UpdateServiceAccount(existingServiceAccount, testClient)
	assert.Error(t, err)
}

func TestDeleteServiceAccount(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = testName
	})).Build()

	err := DeleteServiceAccount(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingServiceAccount := &corev1.ServiceAccount{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingServiceAccount)

	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))

	testClient = fake.NewClientBuilder().Build()
	err = DeleteServiceAccount(testName, testNamespace, testClient)
	assert.NoError(t, err)
}
