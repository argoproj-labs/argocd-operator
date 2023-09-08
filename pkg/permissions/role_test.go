package permissions

import (
	"context"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
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

type roleOpt func(*rbacv1.Role)

func getTestRole(opts ...roleOpt) *rbacv1.Role {
	desiredRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
	}

	for _, opt := range opts {
		opt(desiredRole)
	}
	return desiredRole
}

func TestRequestRole(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name        string
		rolReq      RoleRequest
		desiredRole *rbacv1.Role
		wantErr     bool
	}{
		{
			name: "request role, no mutation, custom name, labels, annotations",
			rolReq: RoleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Rules: testRules,
			},
			desiredRole: getTestRole(func(r *rbacv1.Role) {
				r.Name = testName
				r.Namespace = testNamespace
				r.Labels = testKVP
				r.Annotations = testKVP
				r.Rules = testRules
			}),
			wantErr: false,
		},
		{
			name: "request role, successful mutation",
			rolReq: RoleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Rules: testRules,
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredRole: getTestRole(func(r *rbacv1.Role) {
				r.Name = testName
				r.Namespace = testNamespace
				r.Labels = testKVP
				r.Annotations = testKVP
				r.Rules = testRulesMutated
			}),
			wantErr: false,
		},
		{
			name: "request role, failed mutation",
			rolReq: RoleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Rules: testRules,
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredRole: getTestRole(func(r *rbacv1.Role) {
				r.Name = testName
				r.Namespace = testNamespace
				r.Labels = testKVP
				r.Annotations = testKVP
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

	desiredRole := getTestRole(func(r *rbacv1.Role) {
		r.TypeMeta = metav1.TypeMeta{
			Kind:       "Role",
			APIVersion: "rbac.authorization.k8s.io/v1",
		}
		r.Name = testName
		r.Namespace = testNamespace
		r.Labels = testKVP
		r.Annotations = testKVP
	})
	err := CreateRole(desiredRole, testClient)
	assert.NoError(t, err)

	createdRole := &rbacv1.Role{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdRole)

	assert.NoError(t, err)
	assert.Equal(t, desiredRole, createdRole)
}

func TestGetRole(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestRole(func(r *rbacv1.Role) {
		r.Name = testName
		r.Namespace = testNamespace
	})).Build()

	_, err := GetRole(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetRole(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestListRoles(t *testing.T) {
	role1 := getTestRole(func(r *rbacv1.Role) {
		r.Name = "role-1"
		r.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	role2 := getTestRole(func(r *rbacv1.Role) { r.Name = "role-2" })
	role3 := getTestRole(func(r *rbacv1.Role) {
		r.Name = "role-3"
		r.Labels[common.AppK8sKeyComponent] = "new-component-2"
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

	existingRoleList, err := ListRoles(testNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingRoles := []string{}
	for _, role := range existingRoleList.Items {
		existingRoles = append(existingRoles, role.Name)
	}
	sort.Strings(existingRoles)

	assert.Equal(t, desiredRoles, existingRoles)

}

func TestUpdateRole(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestRole(func(r *rbacv1.Role) {
		r.Name = testName
		r.Namespace = testNamespace
	})).Build()

	desiredRole := getTestRole(func(r *rbacv1.Role) {
		r.Name = testName
		r.Rules = testRulesMutated
		r.Namespace = testNamespace
	})
	err := UpdateRole(desiredRole, testClient)
	assert.NoError(t, err)

	existingRole := &rbacv1.Role{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingRole)

	assert.NoError(t, err)
	assert.Equal(t, desiredRole.Rules, existingRole.Rules)

	testClient = fake.NewClientBuilder().Build()
	existingRole = getTestRole(func(r *rbacv1.Role) {
		r.Name = testName
	})
	err = UpdateRole(existingRole, testClient)
	assert.Error(t, err)
}

func TestDeleteRole(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestRole(func(r *rbacv1.Role) {
		r.Name = testName
	})).Build()

	err := DeleteRole(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingRole := &rbacv1.Role{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingRole)

	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))

	testClient = fake.NewClientBuilder().Build()
	err = DeleteRole(testName, testNamespace, testClient)
	assert.NoError(t, err)
}
