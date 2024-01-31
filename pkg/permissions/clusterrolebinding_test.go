package permissions

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func TestRequestClusterRoleBinding(t *testing.T) {
	tests := []struct {
		name       string
		crbReq     ClusterRoleBindingRequest
		desiredCrb *rbacv1.ClusterRoleBinding
	}{

		{
			name: "request clusterrolebinding",
			crbReq: ClusterRoleBindingRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},

				RoleRef:  test.MakeTestRoleRef(test.TestName),
				Subjects: test.MakeTestSubjects(types.NamespacedName{Name: test.TestName, Namespace: test.TestNamespace}),
			},
			desiredCrb: test.MakeTestClusterRoleBinding(nil, func(crb *rbacv1.ClusterRoleBinding) {
				crb.Name = test.TestName
				crb.Labels = test.TestKVP
				crb.Annotations = test.TestKVP
				crb.RoleRef = test.MakeTestRoleRef(test.TestName)
				crb.Subjects = test.MakeTestSubjects(types.NamespacedName{Name: test.TestName, Namespace: test.TestNamespace})
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotCrb := RequestClusterRoleBinding(test.crbReq)
			assert.Equal(t, test.desiredCrb, gotCrb)
		})
	}
}

func TestCreateClusterRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredClusterRoleBinding := test.MakeTestClusterRoleBinding(nil, func(crb *rbacv1.ClusterRoleBinding) {
		crb.TypeMeta = metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		}
		crb.Name = test.TestName
		crb.Labels = test.TestKVP
		crb.Annotations = test.TestKVP
	})
	err := CreateClusterRoleBinding(desiredClusterRoleBinding, testClient)
	assert.NoError(t, err)

	createdClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: test.TestName,
	}, createdClusterRoleBinding)

	assert.NoError(t, err)
	assert.Equal(t, desiredClusterRoleBinding, createdClusterRoleBinding)
}

func TestGetClusterRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestClusterRoleBinding(nil, func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = test.TestName
	})).Build()

	_, err := GetClusterRoleBinding(test.TestName, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetClusterRoleBinding(test.TestName, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListClusterRoleBindings(t *testing.T) {
	crb1 := test.MakeTestClusterRoleBinding(nil, func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = "crb-1"
		crb.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	crb2 := test.MakeTestClusterRoleBinding(nil, func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = "crb-2"
	})
	crb3 := test.MakeTestClusterRoleBinding(nil, func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = "crb-3"
		crb.Labels[common.AppK8sKeyComponent] = "new-component-2"
	})

	testClient := fake.NewClientBuilder().WithObjects(
		crb1, crb2, crb3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredClusterRoleBindings := []string{"crb-1", "crb-3"}

	existingClusterRoleBindingList, err := ListClusterRoleBindings(testClient, listOpts)
	assert.NoError(t, err)

	existingClusterRoleBindings := []string{}
	for _, crb := range existingClusterRoleBindingList.Items {
		existingClusterRoleBindings = append(existingClusterRoleBindings, crb.Name)
	}
	sort.Strings(existingClusterRoleBindings)

	assert.Equal(t, desiredClusterRoleBindings, existingClusterRoleBindings)
}

func TestUpdateClusterRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestClusterRoleBinding(nil, func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = test.TestName
	})).Build()

	desiredClusterRoleBinding := test.MakeTestClusterRoleBinding(nil, func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = test.TestName
		crb.RoleRef = test.MakeTestRoleRef(test.TestName)
		crb.Subjects = test.MakeTestSubjects(types.NamespacedName{Name: test.TestName, Namespace: test.TestNamespace})
	})

	err := UpdateClusterRoleBinding(desiredClusterRoleBinding, testClient)
	assert.NoError(t, err)

	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: test.TestName,
	}, existingClusterRoleBinding)

	assert.NoError(t, err)
	assert.Equal(t, desiredClusterRoleBinding.Subjects, existingClusterRoleBinding.Subjects)

	testClient = fake.NewClientBuilder().Build()
	existingClusterRoleBinding = test.MakeTestClusterRoleBinding(nil, func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = test.TestName
	})
	err = UpdateClusterRoleBinding(existingClusterRoleBinding, testClient)
	assert.Error(t, err)
}

func TestDeleteClusterRoleBinding(t *testing.T) {
	testClusterRoleBinding := test.MakeTestClusterRoleBinding(nil, func(rb *rbacv1.ClusterRoleBinding) {
		rb.Name = test.TestName
	})

	testClient := fake.NewClientBuilder().WithObjects(testClusterRoleBinding).Build()

	err := DeleteClusterRoleBinding(test.TestName, testClient)
	assert.NoError(t, err)

	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: test.TestName,
	}, existingClusterRoleBinding)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
