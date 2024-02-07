package workloads

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
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

func TestRequestDeployment(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name              string
		deployReq         DeploymentRequest
		desiredDeployment *appsv1.Deployment
		wantErr           bool
	}{
		{
			name: "request deployment",
			deployReq: DeploymentRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: test.TestKVP,
					},
				},
			},
			desiredDeployment: test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
				d.Name = test.TestName
				d.Namespace = test.TestNamespace
				d.Labels = test.TestKVP
				d.Annotations = test.TestKVP
				d.Spec.Selector.MatchLabels = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request deployment, successful mutation",
			deployReq: DeploymentRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: test.TestKVP,
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredDeployment: test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
				d.Name = test.TestNameMutated
				d.Namespace = test.TestNamespace
				d.Labels = test.TestKVP
				d.Annotations = test.TestKVP
				d.Spec.Selector.MatchLabels = test.TestKVP
				d.Spec.Replicas = &testReplicasMutated
			}),
			wantErr: false,
		},
		{
			name: "request deployment, failed mutation",
			deployReq: DeploymentRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: test.TestKVP,
					},
				},
				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredDeployment: test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
				d.Name = test.TestNameMutated
				d.Namespace = test.TestNamespace
				d.Labels = test.TestKVP
				d.Annotations = test.TestKVP
				d.Spec.Selector.MatchLabels = test.TestKVP
			}),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotDeployment, err := RequestDeployment(test.deployReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredDeployment, gotDeployment)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateDeployment(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredDeployment := test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
		d.TypeMeta = metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		}
		d.Name = test.TestName
		d.Namespace = test.TestNamespace
		d.Labels = test.TestKVP
		d.Annotations = test.TestKVP
	})
	err := CreateDeployment(desiredDeployment, testClient)
	assert.NoError(t, err)

	createdDeployment := &appsv1.Deployment{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdDeployment)

	assert.NoError(t, err)
	assert.Equal(t, desiredDeployment, createdDeployment)
}

func TestGetDeployment(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
		d.Name = test.TestName
		d.Namespace = test.TestNamespace
		d.Labels = test.TestKVP
		d.Annotations = test.TestKVP
	})).Build()

	_, err := GetDeployment(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetDeployment(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListDeployments(t *testing.T) {
	deployment1 := test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
		d.Name = "deployment-1"
		d.Namespace = test.TestNamespace
		d.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	deployment2 := test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
		d.Name = "deployment-2"
		d.Namespace = test.TestNamespace

	})
	deployment3 := test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
		d.Name = "deployment-3"
		d.Labels[common.AppK8sKeyComponent] = "new-component-2"
		d.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(
		deployment1, deployment2, deployment3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredDeployments := []string{"deployment-1", "deployment-3"}

	existingDeploymentList, err := ListDeployments(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingDeployments := []string{}
	for _, deployment := range existingDeploymentList.Items {
		existingDeployments = append(existingDeployments, deployment.Name)
	}
	sort.Strings(existingDeployments)

	assert.Equal(t, desiredDeployments, existingDeployments)
}

func TestUpdateDeployment(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
		d.Name = test.TestName
		d.Namespace = test.TestNamespace
	})).Build()

	desiredDeployment := test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
		d.Name = test.TestName
		d.Namespace = test.TestNamespace
		d.Labels = map[string]string{
			"control-plane": "argocd-operator",
		}
	})
	err := UpdateDeployment(desiredDeployment, testClient)
	assert.NoError(t, err)

	existingDeployment := &appsv1.Deployment{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingDeployment)

	assert.NoError(t, err)
	assert.Equal(t, desiredDeployment.Labels, existingDeployment.Labels)

	testClient = fake.NewClientBuilder().Build()
	existingDeployment = test.MakeTestDeployment(nil, func(d *appsv1.Deployment) {
		d.Name = test.TestName
		d.Namespace = test.TestNamespace
	})
	err = UpdateDeployment(existingDeployment, testClient)
	assert.Error(t, err)
}

func TestDeleteDeployment(t *testing.T) {
	testDeployment := test.MakeTestDeployment(nil, func(deployment *appsv1.Deployment) {
		deployment.Name = test.TestName
		deployment.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testDeployment).Build()

	err := DeleteDeployment(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingDeployment := &appsv1.Deployment{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingDeployment)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
