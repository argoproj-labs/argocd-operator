package permissions

import (
	"context"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
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

type clusterRoleBindingOpt func(*rbacv1.ClusterRoleBinding)

func getTestClusterRoleBinding(opts ...clusterRoleBindingOpt) *rbacv1.ClusterRoleBinding {
	desiredClusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: argoutil.GenerateUniqueResourceName(testInstance, testInstanceNamespace, testComponent),
			Labels: map[string]string{
				common.ArgoCDKeyName:      testInstance,
				common.ArgoCDKeyPartOf:    common.ArgoCDAppName,
				common.ArgoCDKeyManagedBy: testInstance,
				common.ArgoCDKeyComponent: testComponent,
			},
			Annotations: map[string]string{
				common.AnnotationName:      testInstance,
				common.AnnotationNamespace: testInstanceNamespace,
			},
		},
		RoleRef:  testRoleRef,
		Subjects: testSubjects,
	}

	for _, opt := range opts {
		opt(desiredClusterRoleBinding)
	}
	return desiredClusterRoleBinding
}

func TestRequestClusterRoleBinding(t *testing.T) {
	tests := []struct {
		name       string
		crbReq     ClusterRoleBindingRequest
		desiredCrb *rbacv1.ClusterRoleBinding
	}{
		{
			name: "request clusterrolebinding",
			crbReq: ClusterRoleBindingRequest{
				Name:              "",
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				RoleRef:           testRoleRef,
				Subjects:          testSubjects,
			},
			desiredCrb: getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {}),
		},
		{
			name: "request clusterrolebinding, custom name, labels, annotations",
			crbReq: ClusterRoleBindingRequest{
				Name:              testName,
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Labels:            testKVP,
				Annotations:       testKVP,
				RoleRef:           testRoleRef,
				Subjects:          testSubjects,
			},
			desiredCrb: getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {
				crb.Name = testName
				crb.Labels = argoutil.MergeMaps(crb.Labels, testKVP)
				crb.Annotations = argoutil.MergeMaps(crb.Annotations, testKVP)
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

	desiredClusterRoleBinding := getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {
		crb.TypeMeta = metav1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		}
		crb.Name = testName
	})
	err := CreateClusterRoleBinding(desiredClusterRoleBinding, testClient)
	assert.NoError(t, err)

	createdClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: testName,
	}, createdClusterRoleBinding)

	assert.NoError(t, err)
	assert.Equal(t, desiredClusterRoleBinding, createdClusterRoleBinding)
}

func TestGetClusterRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = testName
	})).Build()

	_, err := GetClusterRoleBinding(testName, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetClusterRoleBinding(testName, testClient)
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestListClusterRoleBindings(t *testing.T) {
	crb1 := getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = "crb-1"
		crb.Labels[common.ArgoCDKeyComponent] = "new-component-1"
	})
	crb2 := getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = "crb-2"
	})
	crb3 := getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = "crb-3"
		crb.Labels[common.ArgoCDKeyComponent] = "new-component-2"
	})

	testClient := fake.NewClientBuilder().WithObjects(
		crb1, crb2, crb3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.ArgoCDKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]ctrlClient.ListOption, 0)
	listOpts = append(listOpts, ctrlClient.MatchingLabelsSelector{
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
	testClient := fake.NewClientBuilder().WithObjects(getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = testName
	})).Build()

	desiredClusterRoleBinding := getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = testName
		crb.RoleRef = testRoleRef
		crb.Subjects = testSubjects
	})

	err := UpdateClusterRoleBinding(desiredClusterRoleBinding, testClient)
	assert.NoError(t, err)

	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: testName,
	}, existingClusterRoleBinding)

	assert.NoError(t, err)
	assert.Equal(t, desiredClusterRoleBinding.Subjects, existingClusterRoleBinding.Subjects)

	testClient = fake.NewClientBuilder().Build()
	existingClusterRoleBinding = getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = testName
	})
	err = UpdateClusterRoleBinding(existingClusterRoleBinding, testClient)
	assert.Error(t, err)
}

func TestDeleteClusterRoleBinding(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestClusterRoleBinding(func(crb *rbacv1.ClusterRoleBinding) {
		crb.Name = testName
	})).Build()

	err := DeleteClusterRoleBinding(testName, testClient)
	assert.NoError(t, err)

	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Name: testName,
	}, existingClusterRoleBinding)

	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))

	testClient = fake.NewClientBuilder().Build()
	err = DeleteClusterRoleBinding(testName, testClient)
	assert.NoError(t, err)
}
