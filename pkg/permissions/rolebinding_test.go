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
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func TestRequestRoleBinding(t *testing.T) {
	tests := []struct {
		name      string
		rbReq     RoleBindingRequest
		desiredRb *rbacv1.RoleBinding
	}{
		{
			name: "request rolebinding",
			rbReq: RoleBindingRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				RoleRef:  test.MakeTestRoleRef(test.TestName),
				Subjects: test.MakeTestSubjects(types.NamespacedName{Name: test.TestName, Namespace: test.TestNamespace}),
			},
			desiredRb: test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
				rb.Name = test.TestName
				rb.Namespace = test.TestNamespace
				rb.Labels = test.TestKVP
				rb.Annotations = test.TestKVP
				rb.RoleRef = test.MakeTestRoleRef(test.TestName)
				rb.Subjects = test.MakeTestSubjects(types.NamespacedName{Name: test.TestName, Namespace: test.TestNamespace})
			}),
		},
		{
			name: "request rolebinding, custom name, labels, annotations",
			rbReq: RoleBindingRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				RoleRef:  test.MakeTestRoleRef(test.TestName),
				Subjects: test.MakeTestSubjects(types.NamespacedName{Name: test.TestName, Namespace: test.TestNamespace}),
			},
			desiredRb: test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
				rb.Name = test.TestName
				rb.Namespace = test.TestNamespace
				rb.Labels = util.MergeMaps(rb.Labels, test.TestKVP)
				rb.Annotations = util.MergeMaps(rb.Annotations, test.TestKVP)
				rb.RoleRef = test.MakeTestRoleRef(test.TestName)
				rb.Subjects = test.MakeTestSubjects(types.NamespacedName{Name: test.TestName, Namespace: test.TestNamespace})
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotRb := RequestRoleBinding(test.rbReq)
			assert.Equal(t, test.desiredRb, gotRb)

		})
	}

}

func TestCreateRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredRoleBinding := test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
		rb.TypeMeta = metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		}
		rb.Name = test.TestName
		rb.Namespace = test.TestNamespace
		rb.Labels = test.TestKVP
		rb.Annotations = test.TestKVP
	})
	err := CreateRoleBinding(desiredRoleBinding, testClient)
	assert.NoError(t, err)

	createdRoleBinding := &rbacv1.RoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdRoleBinding)

	assert.NoError(t, err)
	assert.Equal(t, desiredRoleBinding, createdRoleBinding)
}

func TestGetRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
		rb.Name = test.TestName
		rb.Namespace = test.TestNamespace
	})).Build()

	_, err := GetRoleBinding(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetRoleBinding(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListRoleBindings(t *testing.T) {
	rb1 := test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
		rb.Name = "rb-1"
		rb.Labels[common.AppK8sKeyComponent] = "new-component-1"
		rb.Namespace = test.TestNamespace
	})
	rb2 := test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
		rb.Name = "rb-2"
		rb.Namespace = test.TestNamespace
	})
	rb3 := test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
		rb.Name = "rb-3"
		rb.Labels[common.AppK8sKeyComponent] = "new-component-2"
		rb.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(
		rb1, rb2, rb3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredRoleBindings := []string{"rb-1", "rb-3"}

	existingRoleBindingList, err := ListRoleBindings(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingRoleBindings := []string{}
	for _, rb := range existingRoleBindingList.Items {
		existingRoleBindings = append(existingRoleBindings, rb.Name)
	}
	sort.Strings(existingRoleBindings)

	assert.Equal(t, desiredRoleBindings, existingRoleBindings)
}

func TestUpdateRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
		rb.Name = test.TestName
		rb.Namespace = test.TestNamespace
	})).Build()

	desiredRoleBinding := test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
		rb.Name = test.TestName
		rb.RoleRef = rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "desired-role-name",
			APIGroup: "rbac.authorization.k8s.io",
		}
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "new-sa",
				Namespace: test.TestNamespace,
			},
		}
		rb.Namespace = test.TestNamespace

	})

	err := UpdateRoleBinding(desiredRoleBinding, testClient)
	assert.NoError(t, err)

	existingRoleBinding := &rbacv1.RoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingRoleBinding)

	assert.NoError(t, err)
	assert.Equal(t, desiredRoleBinding.RoleRef, existingRoleBinding.RoleRef)
	assert.Equal(t, desiredRoleBinding.Subjects, existingRoleBinding.Subjects)

	testClient = fake.NewClientBuilder().Build()
	existingRoleBinding = test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
		rb.Name = test.TestName
	})
	err = UpdateRoleBinding(existingRoleBinding, testClient)
	assert.Error(t, err)
}

func TestDeleteRoleBinding(t *testing.T) {
	testRoleBinding := test.MakeTestRoleBinding(nil, func(rb *rbacv1.RoleBinding) {
		rb.Name = test.TestName
		rb.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testRoleBinding).Build()

	err := DeleteRoleBinding(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingRoleBinding := &rbacv1.RoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingRoleBinding)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
