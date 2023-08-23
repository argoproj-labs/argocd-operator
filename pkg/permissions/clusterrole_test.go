package permissions

import (
	"context"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type clusterRoleOpt func(*rbacv1.ClusterRole)

func getTestClusterRole(opts ...clusterRoleOpt) *rbacv1.ClusterRole {
	desiredClusterClusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: argoutil.GenerateUniqueResourceName(testInstance, testInstanceNamespace, testComponent),
			Labels: map[string]string{
				common.AppK8sKeyName:      testInstance,
				common.AppK8sKeyPartOf:    common.ArgoCDAppName,
				common.AppK8sKeyManagedBy: testInstance,
				common.AppK8sKeyComponent: testComponent,
			},
			Annotations: map[string]string{
				common.ArgoCDArgoprojKeyName:      testInstance,
				common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
			},
		},
		Rules: testRules,
	}

	for _, opt := range opts {
		opt(desiredClusterClusterRole)
	}
	return desiredClusterClusterRole
}

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
			name: "request clusterrole, no mutation",
			rolReq: ClusterRoleRequest{
				Name:              "",
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Rules:             testRules,
			},
			mutation:           false,
			desiredClusterRole: getTestClusterRole(func(r *rbacv1.ClusterRole) {}),
			wantErr:            false,
		},
		{
			name: "request clusterrole, no mutation, custom name, labels, annotations",
			rolReq: ClusterRoleRequest{
				Name:              testName,
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Labels:            testKVP,
				Annotations:       testKVP,
				Rules:             testRules,
			},
			mutation: false,
			desiredClusterRole: getTestClusterRole(func(r *rbacv1.ClusterRole) {
				r.Name = testName
				r.Labels = argoutil.MergeMaps(r.Labels, testKVP)
				r.Annotations = argoutil.MergeMaps(r.Annotations, testKVP)
			}),
			wantErr: false,
		},
		{
			name: "request clusterrole, successful mutation",
			rolReq: ClusterRoleRequest{
				Name:              "",
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Rules:             testRules,
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			mutation:           true,
			desiredClusterRole: getTestClusterRole(func(r *rbacv1.ClusterRole) { r.Rules = testRulesMutated }),
			wantErr:            false,
		},
		{
			name: "request clusterrole, failed mutation",
			rolReq: ClusterRoleRequest{
				Name:              "",
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Rules:             testRules,
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			mutation:           true,
			desiredClusterRole: getTestClusterRole(func(r *rbacv1.ClusterRole) {}),
			wantErr:            true,
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

	desiredClusterRole := getTestClusterRole(func(r *rbacv1.ClusterRole) {
		r.Name = testName
		r.TypeMeta = metav1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: "rbac.authorization.k8s.io/v1",
		}

	})
	err := CreateClusterRole(desiredClusterRole, testClient)
	assert.NoError(t, err)

	createdClusterRole := &rbacv1.ClusterRole{}
	err = testClient.Get(context.TODO(), ctrlClient.ObjectKey{Name: testName}, createdClusterRole)

	assert.NoError(t, err)
	assert.Equal(t, desiredClusterRole, createdClusterRole)
}

func TestGetClusterRole(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestClusterRole(func(r *rbacv1.ClusterRole) {
		r.Name = testName
	})).Build()

	_, err := GetClusterRole(testName, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetClusterRole(testName, testClient)
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestListClusterRoles(t *testing.T) {
	role1 := getTestClusterRole(func(r *rbacv1.ClusterRole) {
		r.Name = "role-1"
		r.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	role2 := getTestClusterRole(func(r *rbacv1.ClusterRole) { r.Name = "role-2" })
	role3 := getTestClusterRole(func(r *rbacv1.ClusterRole) {
		r.Name = "role-3"
		r.Labels[common.AppK8sKeyComponent] = "new-component-2"
	})

	testClient := fake.NewClientBuilder().WithObjects(
		role1, role2, role3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]ctrlClient.ListOption, 0)
	listOpts = append(listOpts, ctrlClient.MatchingLabelsSelector{
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
	testClient := fake.NewClientBuilder().WithObjects(getTestClusterRole(func(r *rbacv1.ClusterRole) {
		r.Name = testName
	})).Build()

	desiredClusterRole := getTestClusterRole(func(r *rbacv1.ClusterRole) {
		r.Name = testName
		r.Rules = testRulesMutated
	})
	err := UpdateClusterRole(desiredClusterRole, testClient)
	assert.NoError(t, err)

	existingClusterRole := &rbacv1.ClusterRole{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: testName,
	}, existingClusterRole)

	assert.NoError(t, err)
	assert.Equal(t, desiredClusterRole.Rules, existingClusterRole.Rules)

	testClient = fake.NewClientBuilder().Build()
	existingClusterRole = getTestClusterRole(func(cr *rbacv1.ClusterRole) {
		cr.Name = testName
	})
	err = UpdateClusterRole(existingClusterRole, testClient)
	assert.Error(t, err)
}

func TestDeleteClusterRole(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestClusterRole(func(r *rbacv1.ClusterRole) {
		r.Name = testName
	})).Build()

	err := DeleteClusterRole(testName, testClient)
	assert.NoError(t, err)

	existingClusterRole := &rbacv1.ClusterRole{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: testName,
	}, existingClusterRole)

	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))

	testClient = fake.NewClientBuilder().Build()
	err = DeleteClusterRole(testName, testClient)
	assert.NoError(t, err)
}
