package workloads

import (
	"context"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type deploymentOpt func(*appsv1.Deployment)

func getTestDeployment(opts ...deploymentOpt) *appsv1.Deployment {
	desiredDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      argoutil.GenerateResourceName(testInstance, testComponent),
			Namespace: testInstanceNamespace,
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
		mutation          bool
		wantErr           bool
	}{
		{
			name: "request deployment, no mutation",
			deployReq: DeploymentRequest{
				Name:              "",
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
			},
			mutation:          false,
			desiredDeployment: getTestDeployment(func(d *appsv1.Deployment) {}),
			wantErr:           false,
		},
		{
			name: "request deployment, no mutation, custom name, labels, annotations",
			deployReq: DeploymentRequest{
				Name:              testName,
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Labels:            testKVP,
				Annotations:       testKVP,
			},
			mutation: false,
			desiredDeployment: getTestDeployment(func(d *appsv1.Deployment) {
				d.Name = testName
				d.Labels = argoutil.MergeMaps(d.Labels, testKVP)
				d.Annotations = argoutil.MergeMaps(d.Annotations, testKVP)
			}),
			wantErr: false,
		},
		{
			name: "request deployment, successful mutation",
			deployReq: DeploymentRequest{
				Name:              "",
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			mutation:          true,
			desiredDeployment: getTestDeployment(func(d *appsv1.Deployment) { d.Name = testDeploymentNameMutated }),
			wantErr:           false,
		},
		{
			name: "request deployment, failed mutation",
			deployReq: DeploymentRequest{
				Name:              "",
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			mutation:          true,
			desiredDeployment: getTestDeployment(func(d *appsv1.Deployment) {}),
			wantErr:           true,
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
		d.Labels[common.ArgoCDKeyComponent] = "new-component-1"
	})
	deployment2 := getTestDeployment(func(d *appsv1.Deployment) { d.Name = "deployment-2" })
	deployment3 := getTestDeployment(func(d *appsv1.Deployment) {
		d.Name = "deployment-3"
		d.Labels[common.ArgoCDKeyComponent] = "new-component-2"
	})

	testClient := fake.NewClientBuilder().WithObjects(
		deployment1, deployment2, deployment3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.ArgoCDKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]ctrlClient.ListOption, 0)
	listOpts = append(listOpts, ctrlClient.MatchingLabelsSelector{
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
	})
	err := UpdateDeployment(desiredDeployment, testClient)
	assert.NoError(t, err)

	existingDeployment := &appsv1.Deployment{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingDeployment)

	assert.NoError(t, err)
	assert.Equal(t, desiredDeployment.Name, existingDeployment.Name)

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
