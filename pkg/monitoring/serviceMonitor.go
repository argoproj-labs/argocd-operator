package monitoring

import (
	"context"
	"fmt"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
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

func CreateServiceMonitor(serviceMonitor *monitoringv1.ServiceMonitor, client cntrlClient.Client) error {
	return client.Create(context.TODO(), serviceMonitor)
}

// UpdateServiceMonitor updates the specified ServiceMonitor using the provided client.
func UpdateServiceMonitor(serviceMonitor *monitoringv1.ServiceMonitor, client cntrlClient.Client) error {
	_, err := GetServiceMonitor(serviceMonitor.Name, serviceMonitor.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), serviceMonitor); err != nil {
		return err
	}
	return nil
}

func DeleteServiceMonitor(name, namespace string, client cntrlClient.Client) error {
	existingServiceMonitor, err := GetServiceMonitor(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingServiceMonitor); err != nil {
		return err
	}
	return nil
}

func GetServiceMonitor(name, namespace string, client cntrlClient.Client) (*monitoringv1.ServiceMonitor, error) {
	existingServiceMonitor := &monitoringv1.ServiceMonitor{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingServiceMonitor)
	if err != nil {
		return nil, err
	}
	return existingServiceMonitor, nil
}

func ListServiceMonitors(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*monitoringv1.ServiceMonitorList, error) {
	existingServiceMonitors := &monitoringv1.ServiceMonitorList{}
	err := client.List(context.TODO(), existingServiceMonitors, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingServiceMonitors, nil
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
