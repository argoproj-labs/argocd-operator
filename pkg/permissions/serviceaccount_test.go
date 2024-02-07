package permissions

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
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func TestRequestServiceAccount(t *testing.T) {
	tests := []struct {
		name      string
		saReq     ServiceAccountRequest
		desiredSa *corev1.ServiceAccount
	}{
		{
			name: "request service account",
			saReq: ServiceAccountRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
			},
			desiredSa: test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
				sa.Name = test.TestName
				sa.Namespace = test.TestNamespace
				sa.Labels = util.MergeMaps(sa.Labels, test.TestKVP)
				sa.Annotations = util.MergeMaps(sa.Annotations, test.TestKVP)
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

	desiredServiceAccount := test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.TypeMeta = metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		}
		sa.Name = test.TestName
		sa.Namespace = test.TestNamespace
		sa.Labels = test.TestKVP
		sa.Annotations = test.TestKVP
	})
	err := CreateServiceAccount(desiredServiceAccount, testClient)
	assert.NoError(t, err)

	createdServiceAccount := &corev1.ServiceAccount{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdServiceAccount)

	assert.NoError(t, err)
	assert.Equal(t, desiredServiceAccount, createdServiceAccount)
}

func TestGetServiceAccount(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = test.TestName
		sa.Namespace = test.TestNamespace
	})).Build()

	_, err := GetServiceAccount(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetServiceAccount(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListServiceAccounts(t *testing.T) {
	sa1 := test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = "sa-1"
		sa.Labels[common.AppK8sKeyComponent] = "new-component-1"
		sa.Namespace = test.TestNamespace
	})
	sa2 := test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = "sa-2"
		sa.Namespace = test.TestNamespace
	})
	sa3 := test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = "sa-3"
		sa.Labels[common.AppK8sKeyComponent] = "new-component-2"
		sa.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(
		sa1, sa2, sa3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredServiceAccounts := []string{"sa-1", "sa-3"}

	existingServiceAccountList, err := ListServiceAccounts(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingServiceAccounts := []string{}
	for _, sa := range existingServiceAccountList.Items {
		existingServiceAccounts = append(existingServiceAccounts, sa.Name)
	}
	sort.Strings(existingServiceAccounts)

	assert.Equal(t, desiredServiceAccounts, existingServiceAccounts)
}

func TestUpdateServiceAccount(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = test.TestName
		sa.Namespace = test.TestNamespace
	})).Build()

	desiredServiceAccount := test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = test.TestName
		sa.ImagePullSecrets = []corev1.LocalObjectReference{
			{
				Name: "new-secret",
			},
		}
		sa.Namespace = test.TestNamespace
	})

	err := UpdateServiceAccount(desiredServiceAccount, testClient)
	assert.NoError(t, err)

	existingServiceAccount := &corev1.ServiceAccount{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingServiceAccount)

	assert.NoError(t, err)
	assert.Equal(t, desiredServiceAccount.ImagePullSecrets, existingServiceAccount.ImagePullSecrets)
	assert.Equal(t, desiredServiceAccount.AutomountServiceAccountToken, existingServiceAccount.AutomountServiceAccountToken)

	testClient = fake.NewClientBuilder().Build()
	existingServiceAccount = test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = test.TestName
	})
	err = UpdateServiceAccount(existingServiceAccount, testClient)
	assert.Error(t, err)
}

func TestDeleteServiceAccount(t *testing.T) {
	testServiceAccount := test.MakeTestServiceAccount(func(sa *corev1.ServiceAccount) {
		sa.Name = test.TestName
		sa.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testServiceAccount).Build()

	err := DeleteServiceAccount(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingServiceAccount := &corev1.ServiceAccount{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingServiceAccount)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
