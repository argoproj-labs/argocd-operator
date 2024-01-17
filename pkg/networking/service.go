package networking

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/go-logr/logr"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
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

func EnsureAutoTLSAnnotation(svc *corev1.Service, secretName string, enabled bool, log logr.Logger) bool {
	var autoTLSAnnotationName, autoTLSAnnotationValue string

	// We currently only support OpenShift for automatic TLS
	if IsRouteAPIAvailable() {
		autoTLSAnnotationName = common.ServiceBetaOpenshiftKeyCertSecret
		if svc.Annotations == nil {
			svc.Annotations = make(map[string]string)
		}
		autoTLSAnnotationValue = secretName
	}

	if autoTLSAnnotationName != "" {
		val, ok := svc.Annotations[autoTLSAnnotationName]
		if enabled {
			if !ok || val != secretName {
				log.Info(fmt.Sprintf("requesting AutoTLS on service %s", svc.ObjectMeta.Name))
				svc.Annotations[autoTLSAnnotationName] = autoTLSAnnotationValue
				return true
			}
		} else {
			if ok {
				log.Info(fmt.Sprintf("removing AutoTLS from service %s", svc.ObjectMeta.Name))
				delete(svc.Annotations, autoTLSAnnotationName)
				return true
			}
		}
	}

	return false
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
