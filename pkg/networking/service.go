package networking

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceRequest objects contain all the required information to produce a service object in return
type ServiceRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       corev1.ServiceSpec

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    cntrlClient.Client
}

// newService returns a new Service instance for the given ArgoCD.
func newService(objectMeta metav1.ObjectMeta, spec corev1.ServiceSpec) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: objectMeta,
		Spec:       spec,
	}
}

func CreateService(service *corev1.Service, client cntrlClient.Client) error {
	return client.Create(context.TODO(), service)
}

// UpdateService updates the specified Service using the provided client.
func UpdateService(service *corev1.Service, client cntrlClient.Client) error {
	_, err := GetService(service.Name, service.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), service); err != nil {
		return err
	}
	return nil
}

func DeleteService(name, namespace string, client cntrlClient.Client) error {
	existingService, err := GetService(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingService); err != nil {
		return err
	}
	return nil
}

func GetService(name, namespace string, client cntrlClient.Client) (*corev1.Service, error) {
	existingService := &corev1.Service{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingService)
	if err != nil {
		return nil, err
	}
	return existingService, nil
}

func ListServices(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*corev1.ServiceList, error) {
	existingServices := &corev1.ServiceList{}
	err := client.List(context.TODO(), existingServices, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingServices, nil
}

func RequestService(request ServiceRequest) (*corev1.Service, error) {
	var (
		mutationErr error
	)
	service := newService(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, service, request.Client)
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
