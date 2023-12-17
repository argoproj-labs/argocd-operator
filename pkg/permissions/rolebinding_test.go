package permissions

import (
	"context"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type roleBindingOpt func(*rbacv1.RoleBinding)

func getTestRoleBinding(opts ...roleBindingOpt) *rbacv1.RoleBinding {
	desiredRoleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
	}

	for _, opt := range opts {
		opt(desiredRoleBinding)
	}
	return desiredRoleBinding
}

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
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				RoleRef:  testRoleRef,
				Subjects: testSubjects,
			},
			desiredRb: getTestRoleBinding(func(rb *rbacv1.RoleBinding) {
				rb.Name = testName
				rb.Namespace = testNamespace
				rb.Labels = testKVP
				rb.Annotations = testKVP
				rb.RoleRef = testRoleRef
				rb.Subjects = testSubjects
			}),
		},
		{
			name: "request rolebinding, custom name, labels, annotations",
			rbReq: RoleBindingRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				RoleRef:  testRoleRef,
				Subjects: testSubjects,
			},
			desiredRb: getTestRoleBinding(func(rb *rbacv1.RoleBinding) {
				rb.Name = testName
				rb.Namespace = testNamespace
				rb.Labels = util.MergeMaps(rb.Labels, testKVP)
				rb.Annotations = util.MergeMaps(rb.Annotations, testKVP)
				rb.RoleRef = testRoleRef
				rb.Subjects = testSubjects
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

	desiredRoleBinding := getTestRoleBinding(func(rb *rbacv1.RoleBinding) {
		rb.TypeMeta = metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		}
		rb.Name = testName
		rb.Namespace = testNamespace
		rb.Labels = testKVP
		rb.Annotations = testKVP
	})
	err := CreateRoleBinding(desiredRoleBinding, testClient)
	assert.NoError(t, err)

	createdRoleBinding := &rbacv1.RoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdRoleBinding)

	assert.NoError(t, err)
	assert.Equal(t, desiredRoleBinding, createdRoleBinding)
}

func TestGetRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestRoleBinding(func(rb *rbacv1.RoleBinding) {
		rb.Name = testName
		rb.Namespace = testNamespace
	})).Build()

	_, err := GetRoleBinding(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetRoleBinding(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestListRoleBindings(t *testing.T) {
	rb1 := getTestRoleBinding(func(rb *rbacv1.RoleBinding) {
		rb.Name = "rb-1"
		rb.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	rb2 := getTestRoleBinding(func(rb *rbacv1.RoleBinding) { rb.Name = "rb-2" })
	rb3 := getTestRoleBinding(func(rb *rbacv1.RoleBinding) {
		rb.Name = "rb-3"
		rb.Labels[common.AppK8sKeyComponent] = "new-component-2"
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

	existingRoleBindingList, err := ListRoleBindings(testNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingRoleBindings := []string{}
	for _, rb := range existingRoleBindingList.Items {
		existingRoleBindings = append(existingRoleBindings, rb.Name)
	}
	sort.Strings(existingRoleBindings)

	assert.Equal(t, desiredRoleBindings, existingRoleBindings)
}

func TestUpdateRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestRoleBinding(func(rb *rbacv1.RoleBinding) {
		rb.Name = testName
		rb.Namespace = testNamespace
	})).Build()

	desiredRoleBinding := getTestRoleBinding(func(rb *rbacv1.RoleBinding) {
		rb.Name = testName
		rb.RoleRef = rbacv1.RoleRef{
			Kind:     "Role",
			Name:     "desired-role-name",
			APIGroup: "rbac.authorization.k8s.io",
		}
		rb.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "new-sa",
				Namespace: testNamespace,
			},
		}
		rb.Namespace = testNamespace

	})

	err := UpdateRoleBinding(desiredRoleBinding, testClient)
	assert.NoError(t, err)

	existingRoleBinding := &rbacv1.RoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingRoleBinding)

	assert.NoError(t, err)
	assert.Equal(t, desiredRoleBinding.RoleRef, existingRoleBinding.RoleRef)
	assert.Equal(t, desiredRoleBinding.Subjects, existingRoleBinding.Subjects)

	testClient = fake.NewClientBuilder().Build()
	existingRoleBinding = getTestRoleBinding(func(rb *rbacv1.RoleBinding) {
		rb.Name = testName
	})
	err = UpdateRoleBinding(existingRoleBinding, testClient)
	assert.Error(t, err)
}

func TestDeleteRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestRoleBinding(func(rb *rbacv1.RoleBinding) {
		rb.Name = testName
	})).Build()

	err := DeleteRoleBinding(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingRoleBinding := &rbacv1.RoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingRoleBinding)

	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))

	testClient = fake.NewClientBuilder().Build()
	err = DeleteRoleBinding(testName, testNamespace, testClient)
	assert.NoError(t, err)
}
