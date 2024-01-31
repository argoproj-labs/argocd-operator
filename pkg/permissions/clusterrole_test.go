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

func TestRequestClusterClusterRole(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name               string
		rolReq             ClusterRoleRequest
		desiredClusterRole *rbacv1.ClusterRole
		mutation           bool
		wantErr            bool
	}{
		{
			name: "request clusterrole",
			rolReq: ClusterRoleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Rules: test.TestRules,
			},
			mutation: false,
			desiredClusterRole: test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) {
				r.Name = test.TestName
				r.Labels = test.TestKVP
				r.Annotations = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request clusterrole, successful mutation",
			rolReq: ClusterRoleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Rules: test.TestRules,
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			mutation: true,
			desiredClusterRole: test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) {
				r.Name = test.TestName
				r.Labels = test.TestKVP
				r.Annotations = test.TestKVP
				r.Rules = testRulesMutated
			}),
			wantErr: false,
		},
		{
			name: "request clusterrole, failed mutation",
			rolReq: ClusterRoleRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Rules: test.TestRules,
				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			mutation: true,
			desiredClusterRole: test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) {
				r.Name = test.TestName
				r.Labels = test.TestKVP
				r.Annotations = test.TestKVP
			}),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotRole, err := RequestClusterRole(test.rolReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredClusterRole, gotRole)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateClusterRole(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredClusterRole := test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) {
		r.Name = test.TestName
		r.TypeMeta = metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: "rbac.authorization.k8s.io/v1",
		}
		r.Labels = test.TestKVP
		r.Annotations = test.TestKVP

	})
	err := CreateClusterRole(desiredClusterRole, testClient)
	assert.NoError(t, err)

	createdClusterRole := &rbacv1.ClusterRole{}
	err = testClient.Get(context.TODO(), cntrlClient.ObjectKey{Name: test.TestName}, createdClusterRole)

	assert.NoError(t, err)
	assert.Equal(t, desiredClusterRole, createdClusterRole)
}

func TestGetClusterRole(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) {
		r.Name = test.TestName
	})).Build()

	_, err := GetClusterRole(test.TestName, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetClusterRole(test.TestName, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListClusterRoles(t *testing.T) {
	role1 := test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) {
		r.Name = "role-1"
		r.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	role2 := test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) { r.Name = "role-2" })
	role3 := test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) {
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

	existingRoleList, err := ListClusterRoles(testClient, listOpts)
	assert.NoError(t, err)

	existingRoles := []string{}
	for _, role := range existingRoleList.Items {
		existingRoles = append(existingRoles, role.Name)
	}
	sort.Strings(existingRoles)

	assert.Equal(t, desiredRoles, existingRoles)
}

func TestUpdateClusterRole(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) {
		r.Name = test.TestName
	})).Build()

	desiredClusterRole := test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) {
		r.Name = test.TestName
		r.Rules = testRulesMutated
	})
	err := UpdateClusterRole(desiredClusterRole, testClient)
	assert.NoError(t, err)

	existingClusterRole := &rbacv1.ClusterRole{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: test.TestName,
	}, existingClusterRole)

	assert.NoError(t, err)
	assert.Equal(t, desiredClusterRole.Rules, existingClusterRole.Rules)

	testClient = fake.NewClientBuilder().Build()
	existingClusterRole = test.MakeTestClusterRole(nil, func(cr *rbacv1.ClusterRole) {
		cr.Name = test.TestName
	})
	err = UpdateClusterRole(existingClusterRole, testClient)
	assert.Error(t, err)
}

func TestDeleteClusterRole(t *testing.T) {
	testClusterRole := test.MakeTestClusterRole(nil, func(r *rbacv1.ClusterRole) {
		r.Name = test.TestName
	})

	testClient := fake.NewClientBuilder().WithObjects(testClusterRole).Build()

	err := DeleteClusterRole(test.TestName, testClient)
	assert.NoError(t, err)

	existingClusterRole := &rbacv1.ClusterRole{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: test.TestName,
	}, existingClusterRole)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
