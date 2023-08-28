package monitoring

import (
	"context"
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/openshift/client-go/apps/clientset/versioned/scheme"
	"github.com/stretchr/testify/assert"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type serviceMonitorOpt func(*monitoringv1.ServiceMonitor)

func getTestServiceMonitor(opts ...serviceMonitorOpt) *monitoringv1.ServiceMonitor {
	desiredServiceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: testNamespace,
			Labels: map[string]string{
				common.AppK8sKeyName:      testInstance,
				common.AppK8sKeyPartOf:    common.ArgoCDAppName,
				common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
				common.AppK8sKeyComponent: testComponent,
			},
			Annotations: map[string]string{
				common.ArgoCDArgoprojKeyName:      testInstance,
				common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.AppK8sKeyName: argoutil.GenerateResourceName(testInstance, common.ArgoCDMetrics),
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: common.ArgoCDMetrics,
				},
			},
		},
	}

	for _, opt := range opts {
		opt(desiredServiceMonitor)
	}
	return desiredServiceMonitor
}

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
			name: "request serviceMonitor, no mutation",
			serviceMonitorReq: ServiceMonitorRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
					},
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.AppK8sKeyName: argoutil.GenerateResourceName(testInstance, common.ArgoCDMetrics),
						},
					},
					Endpoints: []monitoringv1.Endpoint{
						{
							Port: common.ArgoCDMetrics,
						},
					},
				},
			},
			desiredServiceMonitor: getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) {}),
			wantErr:               false,
		},
		{
			name: "request serviceMonitor, no mutation, custom name, labels, annotations",
			serviceMonitorReq: ServiceMonitorRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
						testKey:                   testVal,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
						testKey:                           testVal,
					},
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.AppK8sKeyName: argoutil.GenerateResourceName(testInstance, common.ArgoCDMetrics),
						},
					},
					Endpoints: []monitoringv1.Endpoint{
						{
							Port: common.ArgoCDMetrics,
						},
					},
				},
			},
			desiredServiceMonitor: getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) {
				sm.Name = testName
				sm.Labels = argoutil.MergeMaps(sm.Labels, testKVP)
				sm.Annotations = argoutil.MergeMaps(sm.Annotations, testKVP)
			}),
			wantErr: false,
		},
		{
			name: "request serviceMonitor, successful mutation",
			serviceMonitorReq: ServiceMonitorRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testServiceMonitorNameMutated,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
					},
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.AppK8sKeyName: argoutil.GenerateResourceName(testInstance, common.ArgoCDMetrics),
						},
					},
					Endpoints: []monitoringv1.Endpoint{
						{
							Port: common.ArgoCDMetrics,
						},
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncSuccessful,
				},
				Client: testClient,
			},
			desiredServiceMonitor: getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) { sm.Name = testServiceMonitorNameMutated }),
			wantErr:               false,
		},
		{
			name: "request serviceMonitor, failed mutation",
			serviceMonitorReq: ServiceMonitorRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testName,
					Namespace: testNamespace,
					Labels: map[string]string{
						common.AppK8sKeyName:      testInstance,
						common.AppK8sKeyPartOf:    common.ArgoCDAppName,
						common.AppK8sKeyManagedBy: common.ArgoCDOperatorName,
						common.AppK8sKeyComponent: testComponent,
					},
					Annotations: map[string]string{
						common.ArgoCDArgoprojKeyName:      testInstance,
						common.ArgoCDArgoprojKeyNamespace: testInstanceNamespace,
					},
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Selector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							common.AppK8sKeyName: argoutil.GenerateResourceName(testInstance, common.ArgoCDMetrics),
						},
					},
					Endpoints: []monitoringv1.Endpoint{
						{
							Port: common.ArgoCDMetrics,
						},
					},
				},
				Mutations: []mutation.MutateFunc{
					testMutationFuncFailed,
				},
				Client: testClient,
			},
			desiredServiceMonitor: getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) {}),
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

	desiredServiceMonitor := getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) {
		sm.TypeMeta = metav1.TypeMeta{
			Kind:       "ServiceMonitor",
			APIVersion: "monitoring.coreos.com/v1",
		}
		sm.Name = testName
		sm.Namespace = testNamespace
	})
	err := CreateServiceMonitor(desiredServiceMonitor, testClient)
	assert.NoError(t, err)

	createdServiceMonitor := &monitoringv1.ServiceMonitor{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, createdServiceMonitor)

	assert.NoError(t, err)
	assert.Equal(t, desiredServiceMonitor, createdServiceMonitor)
}

func TestGetServiceMonitor(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = testName
		sm.Namespace = testNamespace
	})).Build()

	_, err := GetServiceMonitor(testName, testNamespace, testClient)
	assert.NoError(t, err)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()

	_, err = GetServiceMonitor(testName, testNamespace, testClient)
	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))
}

func TestListServiceMonitors(t *testing.T) {
	serviceMonitor1 := getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = "serviceMonitor-1"
		sm.Namespace = testNamespace
		sm.Labels[common.AppK8sKeyComponent] = "new-component-1"
	})
	serviceMonitor2 := getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) { sm.Name = "serviceMonitor-2" })
	serviceMonitor3 := getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = "serviceMonitor-3"
		sm.Namespace = testNamespace
		sm.Labels[common.AppK8sKeyComponent] = "new-component-2"
	})

	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(
		serviceMonitor1, serviceMonitor2, serviceMonitor3,
	).Build()

	componentReq, _ := labels.NewRequirement(common.AppK8sKeyComponent, selection.In, []string{"new-component-1", "new-component-2"})
	selector := labels.NewSelector().Add(*componentReq)

	listOpts := make([]ctrlClient.ListOption, 0)
	listOpts = append(listOpts, ctrlClient.MatchingLabelsSelector{
		Selector: selector,
	})

	desiredServiceMonitors := []string{"serviceMonitor-1", "serviceMonitor-3"}

	existingServiceMonitorList, err := ListServiceMonitors(testNamespace, testClient, listOpts)
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
	initialServiceMonitor := getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = testName
		sm.Namespace = testNamespace
	})

	// Create the client with the initial ServiceMonitor
	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(initialServiceMonitor).Build()

	// Fetch the ServiceMonitor from the client
	desiredServiceMonitor := &monitoringv1.ServiceMonitor{}
	err := testClient.Get(context.TODO(), types.NamespacedName{Name: testName, Namespace: testNamespace}, desiredServiceMonitor)
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
		Namespace: testNamespace,
		Name:      testName,
	}, existingServiceMonitor)

	assert.NoError(t, err)
	assert.Equal(t, desiredServiceMonitor.Spec.Endpoints, existingServiceMonitor.Spec.Endpoints)

	testClient = fake.NewClientBuilder().WithScheme(s).Build()
	existingServiceMonitor = getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = testName
		sm.Labels = nil
	})
	err = UpdateServiceMonitor(existingServiceMonitor, testClient)
	assert.Error(t, err)
}

func TestDeleteServiceMonitor(t *testing.T) {
	s := scheme.Scheme
	assert.NoError(t, monitoringv1.AddToScheme(s))

	testClient := fake.NewClientBuilder().WithScheme(s).WithObjects(getTestServiceMonitor(func(sm *monitoringv1.ServiceMonitor) {
		sm.Name = testName
		sm.Namespace = testNamespace
	})).Build()

	err := DeleteServiceMonitor(testName, testNamespace, testClient)
	assert.NoError(t, err)

	existingServiceMonitor := &monitoringv1.ServiceMonitor{}
	err = testClient.Get(context.TODO(), types.NamespacedName{
		Namespace: testNamespace,
		Name:      testName,
	}, existingServiceMonitor)

	assert.Error(t, err)
	assert.True(t, k8serrors.IsNotFound(err))

	testClient = fake.NewClientBuilder().WithScheme(s).Build()
	err = DeleteServiceMonitor(testName, testNamespace, testClient)
	assert.NoError(t, err)
}
