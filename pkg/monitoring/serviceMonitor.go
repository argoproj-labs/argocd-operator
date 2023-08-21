package monitoring

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceMonitorRequest objects contain all the required information to produce a serviceMonitor object in return
type ServiceMonitorRequest struct {
	Name              string
	InstanceName      string
	InstanceNamespace string
	Component         string
	Labels            map[string]string
	Annotations       map[string]string
	Selector          metav1.LabelSelector
	Endpoints         []monitoringv1.Endpoint

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    interface{}
}

// newServiceMonitor returns a new ServiceMonitor instance for the given ArgoCD.
func newServiceMonitor(name, instanceName, instanceNamespace, component string, labels, annotations map[string]string, selector metav1.LabelSelector, endpoints []monitoringv1.Endpoint) *monitoringv1.ServiceMonitor {
	var serviceMonitorName string
	if name != "" {
		serviceMonitorName = name
	} else {
		serviceMonitorName = argoutil.GenerateResourceName(instanceName, component)

	}
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:        serviceMonitorName,
			Namespace:   instanceNamespace,
			Labels:      argoutil.MergeMaps(argoutil.LabelsForCluster(instanceName, component), labels),
			Annotations: argoutil.MergeMaps(argoutil.AnnotationsForCluster(instanceName, instanceNamespace), annotations),
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector:  selector,
			Endpoints: endpoints,
		},
	}
}

func CreateServiceMonitor(serviceMonitor *monitoringv1.ServiceMonitor, client ctrlClient.Client) error {
	return client.Create(context.TODO(), serviceMonitor)
}

// UpdateServiceMonitor updates the specified ServiceMonitor using the provided client.
func UpdateServiceMonitor(serviceMonitor *monitoringv1.ServiceMonitor, client ctrlClient.Client) error {
	_, err := GetServiceMonitor(serviceMonitor.Name, serviceMonitor.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), serviceMonitor); err != nil {
		return err
	}
	return nil
}

func DeleteServiceMonitor(name, namespace string, client ctrlClient.Client) error {
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

func GetServiceMonitor(name, namespace string, client ctrlClient.Client) (*monitoringv1.ServiceMonitor, error) {
	existingServiceMonitor := &monitoringv1.ServiceMonitor{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingServiceMonitor)
	if err != nil {
		return nil, err
	}
	return existingServiceMonitor, nil
}

func ListServiceMonitors(namespace string, client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*monitoringv1.ServiceMonitorList, error) {
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
	serviceMonitor := newServiceMonitor(request.Name, request.InstanceName, request.InstanceNamespace, request.Component, request.Labels, request.Annotations, request.Selector, request.Endpoints)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, serviceMonitor, request.Client)
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
