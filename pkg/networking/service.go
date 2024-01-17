package networking

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

// ServiceRequest objects contain all the required information to produce a service object in return
type ServiceRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       corev1.ServiceSpec
	Instance   *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newService returns a new Service instance for the given ArgoCD.
func newService(objectMeta metav1.ObjectMeta, spec corev1.ServiceSpec) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: objectMeta,
		Spec:       spec,
	}
}

func RequestService(request ServiceRequest) (*corev1.Service, error) {
	var (
		mutationErr error
	)
	service := newService(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, service, request.Client, request.MutationArgs...)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return service, fmt.Errorf("RequestService: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return service, nil
}

// CreateService creates the specified Service using the provided client.
func CreateService(service *corev1.Service, client cntrlClient.Client) error {
	return resource.CreateObject(service, client)
}

// UpdateService updates the specified Service using the provided client.
func UpdateService(service *corev1.Service, client cntrlClient.Client) error {
	return resource.UpdateObject(service, client)
}

// DeleteService deletes the Service with the given name and namespace using the provided client.
func DeleteService(name, namespace string, client cntrlClient.Client) error {
	service := &corev1.Service{}
	return resource.DeleteObject(name, namespace, service, client)
}

// GetService retrieves the Service with the given name and namespace using the provided client.
func GetService(name, namespace string, client cntrlClient.Client) (*corev1.Service, error) {
	service := &corev1.Service{}
	obj, err := resource.GetObject(name, namespace, service, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as a corev1.Service
	service, ok := obj.(*corev1.Service)
	if !ok {
		return nil, errors.New("failed to assert the object as a corev1.Service")
	}
	return service, nil
}

// ListServices returns a list of Service objects in the specified namespace using the provided client and list options.
func ListServices(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*corev1.ServiceList, error) {
	serviceList := &corev1.ServiceList{}
	obj, err := resource.ListObjects(namespace, serviceList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as a corev1.ServiceList
	serviceList, ok := obj.(*corev1.ServiceList)
	if !ok {
		return nil, errors.New("failed to assert the object as a corev1.ServiceList")
	}
	return serviceList, nil
}
