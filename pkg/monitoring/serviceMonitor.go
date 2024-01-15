package monitoring

import (
	"errors"
	"fmt"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
)

// ServiceMonitorRequest objects contain all the required information to produce a serviceMonitor object in return
type ServiceMonitorRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       monitoringv1.ServiceMonitorSpec
	Instance   *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newServiceMonitor returns a new ServiceMonitor instance for the given ArgoCD.
func newServiceMonitor(objectMeta metav1.ObjectMeta, spec monitoringv1.ServiceMonitorSpec) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: objectMeta,
		Spec:       spec,
	}
}

func RequestServiceMonitor(request ServiceMonitorRequest) (*monitoringv1.ServiceMonitor, error) {
	var (
		mutationErr error
	)
	serviceMonitor := newServiceMonitor(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, serviceMonitor, request.Client, request.MutationArgs)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return serviceMonitor, fmt.Errorf("RequestServiceMonitor: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return serviceMonitor, nil
}

// CreateServiceMonitor creates the specified ServiceMonitor using the provided client.
func CreateServiceMonitor(serviceMonitor *monitoringv1.ServiceMonitor, client cntrlClient.Client) error {
	return resource.CreateObject(serviceMonitor, client)
}

// UpdateServiceMonitor updates the specified ServiceMonitor using the provided client.
func UpdateServiceMonitor(serviceMonitor *monitoringv1.ServiceMonitor, client cntrlClient.Client) error {
	return resource.UpdateObject(serviceMonitor, client)
}

// DeleteServiceMonitor deletes the ServiceMonitor with the given name and namespace using the provided client.
func DeleteServiceMonitor(name, namespace string, client cntrlClient.Client) error {
	serviceMonitor := &monitoringv1.ServiceMonitor{}
	return resource.DeleteObject(name, namespace, serviceMonitor, client)
}

// GetServiceMonitor retrieves the ServiceMonitor with the given name and namespace using the provided client.
func GetServiceMonitor(name, namespace string, client cntrlClient.Client) (*monitoringv1.ServiceMonitor, error) {
	serviceMonitor := &monitoringv1.ServiceMonitor{}
	obj, err := resource.GetObject(name, namespace, serviceMonitor, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as a monitoringv1.ServiceMonitor
	serviceMonitor, ok := obj.(*monitoringv1.ServiceMonitor)
	if !ok {
		return nil, errors.New("failed to assert the object as a monitoringv1.ServiceMonitor")
	}
	return serviceMonitor, nil
}

// ListServiceMonitors returns a list of ServiceMonitor objects in the specified namespace using the provided client and list options.
func ListServiceMonitors(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*monitoringv1.ServiceMonitorList, error) {
	serviceMonitorList := &monitoringv1.ServiceMonitorList{}
	obj, err := resource.ListObjects(namespace, serviceMonitorList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as a monitoringv1.ServiceMonitorList
	serviceMonitorList, ok := obj.(*monitoringv1.ServiceMonitorList)
	if !ok {
		return nil, errors.New("failed to assert the object as a monitoringv1.ServiceMonitorList")
	}
	return serviceMonitorList, nil
}
