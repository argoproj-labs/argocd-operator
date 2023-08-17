package networking

import (
	"context"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type serviceOpt func(*corev1.Service)

func getTestService(opts ...serviceOpt) *corev1.Service {
	desiredService := &corev1.Service{
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
		opt(desiredService)
	}
	return desiredService
}

func TestRequestService(t *testing.T) {

	testClient := fake.NewClientBuilder().Build()

	tests := []struct {
		name           string
		deployReq      ServiceRequest
		desiredService *corev1.Service
		mutation       bool
		wantErr        bool
	}{
		{
			name: "request service, no mutation",
			deployReq: ServiceRequest{
				Name:              "",
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
			},
			mutation:       false,
			desiredService: getTestService(func(s *corev1.Service) {}),
			wantErr:        false,
		},
		{
			name: "request service, no mutation, custom name, labels, annotations",
			deployReq: ServiceRequest{
				Name:              testName,
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Labels:            testKVP,
				Annotations:       testKVP,
			},
			mutation: false,
			desiredService: getTestService(func(s *corev1.Service) {
				s.Name = testName
				s.Labels = argoutil.MergeMaps(s.Labels, testKVP)
				s.Annotations = argoutil.MergeMaps(s.Annotations, testKVP)
			}),
			wantErr: false,
		},
		{
			name: "request service, successful mutation",
			deployReq: ServiceRequest{
				Name:              "",
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			mutation:       true,
			desiredService: getTestService(func(s *corev1.Service) { s.Name = testServiceNameMutated }),
			wantErr:        false,
		},
		{
			name: "request service, failed mutation",
			deployReq: ServiceRequest{
				Name:              "",
				InstanceName:      testInstance,
				InstanceNamespace: testInstanceNamespace,
				Component:         testComponent,
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			mutation:       true,
			desiredService: getTestService(func(s *corev1.Service) {}),
			wantErr:        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotService, err := RequestService(test.deployReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredService, gotService)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateService(t *testing.T) {
	testClient := fake.NewClientBuilder().Build()

	desiredService := getTestService(func(s *corev1.Service) {
		s.TypeMeta = metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		}
		s.Name = testName
		s.Namespace = testNamespace
	})
	err := CreateService(desiredService, testClient)
	assert.NoError(t, err)

	createdService := &corev1.Service{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdService)

	assert.NoError(t, err)
	assert.Equal(t, desiredService, createdService)
}

func TestGetService(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestService(func(s *corev1.Service) {
		s.Name = testName
		s.Namespace = testNamespace
	})).Build()

	_, err := GetService(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetService(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestListServices(t *testing.T) {
	service1 := getTestService(func(s *corev1.Service) {
		s.Name = "service-1"
		s.Namespace = testNamespace
		s.Labels[common.ArgoCDKeyComponent] = "new-component-1"
	})
	service2 := getTestService(func(s *corev1.Service) { s.Name = "service-2" })
	service3 := getTestService(func(s *corev1.Service) {
		s.Name = "service-3"
		s.Labels[common.ArgoCDKeyComponent] = "new-component-2"
	})

	testClient := fake.NewClientBuilder().WithObjects(
		service1, service2, service3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.ArgoCDKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]ctrlClient.ListOption, 0)
	listOpts = append(listOpts, ctrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredServices := []string{"service-1", "service-3"}

	existingServiceList, err := ListServices(testNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingServices := []string{}
	for _, service := range existingServiceList.Items {
		existingServices = append(existingServices, service.Name)
	}
	sort.Strings(existingServices)

	assert.Equal(t, desiredServices, existingServices)
}

func TestUpdateService(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestService(func(s *corev1.Service) {
		s.Name = testName
		s.Namespace = testNamespace
	})).Build()

	desiredService := getTestService(func(s *corev1.Service) {
		s.Name = testName
		s.Namespace = testNamespace
		s.Labels = map[string]string{
			"control-plane": "argocd-operator",
		}
	})
	err := UpdateService(desiredService, testClient)
	assert.NoError(t, err)

	existingService := &corev1.Service{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingService)

	assert.NoError(t, err)
	assert.Equal(t, desiredService.Labels, existingService.Labels)

	testClient = fake.NewClientBuilder().Build()
	existingService = getTestService(func(s *corev1.Service) {
		s.Name = testName
		s.Namespace = testNamespace
	})
	err = UpdateService(existingService, testClient)
	assert.Error(t, err)
}

func TestDeleteService(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(getTestService(func(s *corev1.Service) {
		s.Name = testName
		s.Namespace = testNamespace
	})).Build()

	err := DeleteService(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingService := &corev1.Service{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingService)

	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))

	testClient = fake.NewClientBuilder().Build()
	err = DeleteService(testName, testNamespace, testClient)
	assert.NoError(t, err)
}
