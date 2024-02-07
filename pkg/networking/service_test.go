package networking

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
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
			name: "request service",
			deployReq: ServiceRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
			},
			mutation: false,
			desiredService: test.MakeTestService(nil, func(s *corev1.Service) {
				s.Labels = test.TestKVP
				s.Annotations = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request service, successful mutation",
			deployReq: ServiceRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},

				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			mutation: true,
			desiredService: test.MakeTestService(nil, func(s *corev1.Service) {
				s.Name = testServiceNameMutated
				s.Labels = test.TestKVP
				s.Annotations = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request service, failed mutation",
			deployReq: ServiceRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			mutation: true,
			desiredService: test.MakeTestService(nil, func(s *corev1.Service) {
				s.Labels = test.TestKVP
				s.Annotations = test.TestKVP
			}),
			wantErr: true,
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

	desiredService := test.MakeTestService(nil, func(s *corev1.Service) {
		s.TypeMeta = metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		}
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
		s.Labels = test.TestKVP
		s.Annotations = test.TestKVP
	})
	err := CreateService(desiredService, testClient)
	assert.NoError(t, err)

	createdService := &corev1.Service{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdService)

	assert.NoError(t, err)
	assert.Equal(t, desiredService, createdService)
}

func TestGetService(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestService(nil, func(s *corev1.Service) {
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
	})).Build()

	_, err := GetService(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().Build()

	_, err = GetService(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListServices(t *testing.T) {
	service1 := test.MakeTestService(nil, func(s *corev1.Service) {
		s.Name = "service-1"
		s.Namespace = test.TestNamespace
		s.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	service2 := test.MakeTestService(nil, func(s *corev1.Service) { s.Name = "service-2" })
	service3 := test.MakeTestService(nil, func(s *corev1.Service) {
		s.Name = "service-3"
		s.Labels[common.AppK8sKeyComponent] = "new-component-2"
	})

	testClient := fake.NewClientBuilder().WithObjects(
		service1, service2, service3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredServices := []string{"service-1", "service-3"}

	existingServiceList, err := ListServices(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingServices := []string{}
	for _, service := range existingServiceList.Items {
		existingServices = append(existingServices, service.Name)
	}
	sort.Strings(existingServices)

	assert.Equal(t, desiredServices, existingServices)
}

func TestUpdateService(t *testing.T) {
	testClient := fake.NewClientBuilder().WithObjects(test.MakeTestService(nil, func(s *corev1.Service) {
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
	})).Build()

	desiredService := test.MakeTestService(nil, func(s *corev1.Service) {
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
		s.Labels = map[string]string{
			"control-plane": "argocd-operator",
		}
	})
	err := UpdateService(desiredService, testClient)
	assert.NoError(t, err)

	existingService := &corev1.Service{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingService)

	assert.NoError(t, err)
	assert.Equal(t, desiredService.Labels, existingService.Labels)

	testClient = fake.NewClientBuilder().Build()
	existingService = test.MakeTestService(nil, func(s *corev1.Service) {
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
	})
	err = UpdateService(existingService, testClient)
	assert.Error(t, err)
}

func TestDeleteService(t *testing.T) {
	testService := test.MakeTestService(nil, func(s *corev1.Service) {
		s.Name = test.TestName
		s.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithObjects(testService).Build()

	err := DeleteService(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingService := &corev1.Service{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingService)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
