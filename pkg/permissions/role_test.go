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
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/tests/test"
)

func TestRequestRole(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name        string
		rolReq      RoleRequest
		desiredRole *rbacv1.Role
		wantErr     bool
	}{
		{
			name: "request role",
			rolReq: RoleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Rules: test.TestRules,
			},
			desiredRole: test.MakeTestRole(nil, func(r *rbacv1.Role) {
				r.Name = test.TestName
				r.Namespace = test.TestNamespace
				r.Labels = test.TestKVP
				r.Annotations = test.TestKVP
				r.Rules = test.TestRules
			}),
			wantErr: false,
		},
		{
			name: "request role, successful mutation",
			rolReq: RoleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Rules: test.TestRules,
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredRole: test.MakeTestRole(nil, func(r *rbacv1.Role) {
				r.Name = test.TestName
				r.Namespace = test.TestNamespace
				r.Labels = test.TestKVP
				r.Annotations = test.TestKVP
				r.Rules = testRulesMutated
			}),
			wantErr: false,
		},
		{
			name: "request role, failed mutation",
			rolReq: RoleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Rules: test.TestRules,
				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredRole: test.MakeTestRole(nil, func(r *rbacv1.Role) {
				r.Name = test.TestName
				r.Namespace = test.TestNamespace
				r.Labels = test.TestKVP
				r.Annotations = test.TestKVP
				r.Rules = testRulesMutated

			}),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotRole, err := RequestRole(test.rolReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredRole, gotRole)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateRole(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredRole := test.MakeTestRole(nil, func(r *rbacv1.Role) {
		r.TypeMeta = metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: "rbac.authorization.k8s.io/v1",
		}
		r.Name = test.TestName
		r.Namespace = test.TestNamespace
		r.Labels = test.TestKVP
		r.Annotations = test.TestKVP
	})
	err := CreateRole(desiredRole, testClient)
	assert.NoError(t, err)

	createdRole := &rbacv1.Role{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdRole)

	assert.NoError(t, err)
	assert.Equal(t, desiredRole, createdRole)
}

func TestGetRole(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestRole(nil, func(r *rbacv1.Role) {
		r.Name = test.TestName
		r.Namespace = test.TestNamespace
	})).Build()

	_, err := GetRole(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetRole(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListRoles(t *testing.T) {
	role1 := test.MakeTestRole(nil, func(r *rbacv1.Role) {
		r.Name = "role-1"
		r.Labels[common.AppK8sKeyComponent] = "new-component-1"
		r.Namespace = test.TestNamespace
	})
	role2 := test.MakeTestRole(nil, func(r *rbacv1.Role) {
		r.Name = "role-2"
		r.Namespace = test.TestNamespace
	})
	role3 := test.MakeTestRole(nil, func(r *rbacv1.Role) {
		r.Name = "role-3"
		r.Labels[common.AppK8sKeyComponent] = "new-component-2"
		r.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(
		role1, role2, role3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredRoles := []string{"role-1", "role-3"}

	existingRoleList, err := ListRoles(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingRoles := []string{}
	for _, role := range existingRoleList.Items {
		existingRoles = append(existingRoles, role.Name)
	}
	sort.Strings(existingRoles)

	assert.Equal(t, desiredRoles, existingRoles)

}

func TestUpdateRole(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestRole(nil, func(r *rbacv1.Role) {
		r.Name = test.TestName
		r.Namespace = test.TestNamespace
	})).Build()

	desiredRole := test.MakeTestRole(nil, func(r *rbacv1.Role) {
		r.Name = test.TestName
		r.Rules = testRulesMutated
		r.Namespace = test.TestNamespace
	})
	err := UpdateRole(desiredRole, testClient)
	assert.NoError(t, err)

	existingRole := &rbacv1.Role{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingRole)

	assert.NoError(t, err)
	assert.Equal(t, desiredRole.Rules, existingRole.Rules)

	testClient = fake.NewClientBuilder().Build()
	existingRole = test.MakeTestRole(nil, func(r *rbacv1.Role) {
		r.Name = test.TestName
	})
	err = UpdateRole(existingRole, testClient)
	assert.Error(t, err)
}

func TestDeleteRole(t *testing.T) {
	testRole := test.MakeTestRole(nil, func(r *rbacv1.Role) {
		r.Name = test.TestName
		r.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testRole).Build()

	err := DeleteRole(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingRole := &rbacv1.Role{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingRole)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
