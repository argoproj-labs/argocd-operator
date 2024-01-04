package workloads

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
)

type deploymentOpt func(*appsv1.Deployment)

func getTestDeployment(opts ...deploymentOpt) *appsv1.Deployment {
	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{},
		},
	}

	for _, opt := range opts {
		opt(desiredDeployment)
	}
	return desiredDeployment
}

func TestRequestDeployment(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name              string
		deployReq         DeploymentRequest
		desiredDeployment *appsv1.Deployment
		wantErr           bool
	}{
		{
			name: "request deployment, no mutation, custom name, labels, annotations",
			deployReq: DeploymentRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: testKVP,
					},
				},
			},
			desiredDeployment: getTestDeployment(func(d *appsv1.Deployment) {
				d.Name = testName
				d.Namespace = testNamespace
				d.Labels = testKVP
				d.Annotations = testKVP
				d.Spec.Selector.MatchLabels = testKVP
			}),
			wantErr: false,
		},
		{
			name: "request deployment, successful mutation",
			deployReq: DeploymentRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: testKVP,
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredDeployment: getTestDeployment(func(d *appsv1.Deployment) {
				d.Name = testNameMutated
				d.Namespace = testNamespace
				d.Labels = testKVP
				d.Annotations = testKVP
				d.Spec.Selector.MatchLabels = testKVP
				d.Spec.Replicas = &testReplicasMutated
			}),
			wantErr: false,
		},
		{
			name: "request deployment, failed mutation",
			deployReq: DeploymentRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        testName,
					Namespace:   testNamespace,
					Labels:      testKVP,
					Annotations: testKVP,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: testKVP,
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredDeployment: getTestDeployment(func(d *appsv1.Deployment) {
				d.Name = testNameMutated
				d.Namespace = testNamespace
				d.Labels = testKVP
				d.Annotations = testKVP
				d.Spec.Selector.MatchLabels = testKVP
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

	desiredDeployment := getTestDeployment(func(d *appsv1.Deployment) {
		d.TypeMeta = metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		}
		d.Name = testName
		d.Namespace = testNamespace
		d.Labels = testKVP
		d.Annotations = testKVP
	})
	err := CreateDeployment(desiredDeployment, testClient)
	assert.NoError(t, err)

	createdDeployment := &appsv1.Deployment{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdDeployment)

	assert.NoError(t, err)
	assert.Equal(t, desiredDeployment, createdDeployment)
}

func TestGetDeployment(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestDeployment(func(d *appsv1.Deployment) {
		d.Name = testName
		d.Namespace = testNamespace
		d.Labels = testKVP
		d.Annotations = testKVP
	})).Build()

	_, err := GetDeployment(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetDeployment(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestListDeployments(t *testing.T) {
	deployment1 := getTestDeployment(func(d *appsv1.Deployment) {
		d.Name = "deployment-1"
		d.Namespace = testNamespace
		d.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	deployment2 := getTestDeployment(func(d *appsv1.Deployment) { d.Name = "deployment-2" })
	deployment3 := getTestDeployment(func(d *appsv1.Deployment) {
		d.Name = "deployment-3"
		d.Labels[common.AppK8sKeyComponent] = "new-component-2"
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

	existingDeploymentList, err := ListDeployments(testNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingDeployments := []string{}
	for _, deployment := range existingDeploymentList.Items {
		existingDeployments = append(existingDeployments, deployment.Name)
	}
	sort.Strings(existingDeployments)

	assert.Equal(t, desiredDeployments, existingDeployments)
}

func TestUpdateDeployment(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestDeployment(func(d *appsv1.Deployment) {
		d.Name = testName
		d.Namespace = testNamespace
	})).Build()

	desiredDeployment := getTestDeployment(func(d *appsv1.Deployment) {
		d.Name = testName
		d.Namespace = testNamespace
		d.Labels = map[string]string{
			"control-plane": "argocd-operator",
		}
	})
	err := UpdateDeployment(desiredDeployment, testClient)
	assert.NoError(t, err)

	existingDeployment := &appsv1.Deployment{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingDeployment)

	assert.NoError(t, err)
	assert.Equal(t, desiredDeployment.Labels, existingDeployment.Labels)

	testClient = fake.NewClientBuilder().Build()
	existingDeployment = getTestDeployment(func(d *appsv1.Deployment) {
		d.Name = testName
		d.Namespace = testNamespace
	})
	err = UpdateDeployment(existingDeployment, testClient)
	assert.Error(t, err)
}

func TestDeleteDeployment(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestDeployment(func(d *appsv1.Deployment) {
		d.Name = testName
		d.Namespace = testNamespace
	})).Build()

	err := DeleteDeployment(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingDeployment := &appsv1.Deployment{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingDeployment)

	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))

	testClient = fake.NewClientBuilder().Build()
	err = DeleteDeployment(testName, testNamespace, testClient)
	assert.NoError(t, err)
}
