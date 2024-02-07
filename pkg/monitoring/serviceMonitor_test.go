package monitoring

import (
	"context"
	"sort"
	"testing"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/openshift/client-go/apps/clientset/versioned/scheme"
	"github.com/stretchr/testify/assert"
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

func TestRequestServiceMonitor(t *testing.T) {

	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))
	testClient := fake.NewClientBuilder().WithScheme(s).Build()

	tests := []struct {
		name                  string
		serviceMonitorReq     ServiceMonitorRequest
		desiredServiceMonitor *monitoringv1.ServiceMonitor
		wantErr               bool
	}{
		{
			name: "request serviceMonitor",
			serviceMonitorReq: ServiceMonitorRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: test.TestKVP,
					},
				},
			},
			desiredServiceMonitor: test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {
				sm.Labels = test.TestKVP
				sm.Annotations = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request serviceMonitor",
			serviceMonitorReq: ServiceMonitorRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: test.TestKVP,
					},
				},
			},
			desiredServiceMonitor: test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {
				sm.Labels = test.TestKVP
				sm.Annotations = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request serviceMonitor, successful mutation",
			serviceMonitorReq: ServiceMonitorRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: test.TestKVP,
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredServiceMonitor: test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {
				sm.Name = test.TestNameMutated
				sm.Labels = test.TestKVP
				sm.Annotations = test.TestKVP
			}),
			wantErr: false,
		},
		{
			name: "request serviceMonitor, failed mutation",
			serviceMonitorReq: ServiceMonitorRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:        test.TestName,
					Namespace:   test.TestNamespace,
					Labels:      test.TestKVP,
					Annotations: test.TestKVP,
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: test.TestKVP,
					},
				},
				Mutations: []mutation.MutateFunc{
					test.TestMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredServiceMonitor: test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {}),
			wantErr:               true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotServiceMonitor, err := RequestServiceMonitor(test.serviceMonitorReq)

			if !test.wantErr {
				assert.NoError(t, err)
				assert.Equal(t, test.desiredServiceMonitor, gotServiceMonitor)

			} else {
				assert.Error(t, err)
			}

		})
	}
}

func TestCreateServiceMonitor(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))
	testClient := fake.NewClientBuilder().WithScheme(s).Build()

	desiredServiceMonitor := test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {
		sm.TypeMeta = metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		}
		sm.Name = test.TestName
		sm.Namespace = test.TestNamespace
		sm.Labels = test.TestKVP
		sm.Annotations = test.TestKVP
	})
	err := CreateServiceMonitor(desiredServiceMonitor, testClient)
	assert.NoError(t, err)

	createdServiceMonitor := &monitoringv1.ServiceMonitor{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, createdServiceMonitor)

	assert.NoError(t, err)
	assert.Equal(t, desiredServiceMonitor, createdServiceMonitor)
}

func TestGetServiceMonitor(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = test.TestName
		sm.Namespace = test.TestNamespace
	})).Build()

	_, err := GetServiceMonitor(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()

	_, err = GetServiceMonitor(test.TestName, test.TestNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}

func TestListServiceMonitors(t *testing.T) {
	serviceMonitor1 := test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = "serviceMonitor-1"
		sm.Namespace = test.TestNamespace
		sm.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	serviceMonitor2 := test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) { sm.Name = "serviceMonitor-2" })
	serviceMonitor3 := test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = "serviceMonitor-3"
		sm.Namespace = test.TestNamespace
		sm.Labels[common.AppK8sKeyComponent] = "new-component-2"
	})

	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(
		serviceMonitor1, serviceMonitor2, serviceMonitor3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]cntrlClient.ListOption, 0)
	listOpts = append(listOpts, cntrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredServiceMonitors := []string{"serviceMonitor-1", "serviceMonitor-3"}

	existingServiceMonitorList, err := ListServiceMonitors(test.TestNamespace, testClient, listOpts)
	assert.NoError(t, err)

	existingServiceMonitors := []string{}
	for _, serviceMonitor := range existingServiceMonitorList.Items {
		existingServiceMonitors = append(existingServiceMonitors, serviceMonitor.Name)
	}
	sort.Strings(existingServiceMonitors)

	assert.Equal(t, desiredServiceMonitors, existingServiceMonitors)
}

func TestUpdateServiceMonitor(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	// Create the initial ServiceMonitor
	initialServiceMonitor := test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = test.TestName
		sm.Namespace = test.TestNamespace
	})

	// Create the client with the initial ServiceMonitor
	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(initialServiceMonitor).Build()

	// Fetch the ServiceMonitor from the client
	desiredServiceMonitor := &monitoringv1.ServiceMonitor{}
	err := testClient.Get(context.TODO(), types.NamespacedName{Name: test.TestName, Namespace: test.TestNamespace}, desiredServiceMonitor)
	assert.NoError(t, err)

	desiredServiceMonitor.Spec.Endpoints = []monitoringv1.Endpoint{
		{
			Port: "test-port",
		},
	}

	err = UpdateServiceMonitor(desiredServiceMonitor, testClient)
	assert.NoError(t, err)

	existingServiceMonitor := &monitoringv1.ServiceMonitor{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingServiceMonitor)

	assert.NoError(t, err)
	assert.Equal(t, desiredServiceMonitor.Spec.Endpoints, existingServiceMonitor.Spec.Endpoints)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()
	existingServiceMonitor = test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = test.TestName
		sm.Labels = nil
	})
	err = UpdateServiceMonitor(existingServiceMonitor, testClient)
	assert.Error(t, err)
}

func TestDeleteServiceMonitor(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testServiceMonitor := test.MakeTestServiceMonitor(nil, func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = test.TestName
		sm.Namespace = test.TestNamespace
	})

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(testServiceMonitor).Build()

	err := DeleteServiceMonitor(test.TestName, test.TestNamespace, testClient)
	assert.NoError(t, err)

	existingServiceMonitor := &monitoringv1.ServiceMonitor{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: test.TestNamespace,
		Name:      test.TestName,
	}, existingServiceMonitor)

	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))
}
