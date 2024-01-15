package networking

import (
	"errors"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
)

// IngressRequest objects contain all the required information to produce a ingress object in return
type IngressRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       networkingv1.IngressSpec

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    cntrlClient.Client
}

// newIngress returns a new Ingress instance for the given ArgoCD.
func newIngress(objectMeta metav1.ObjectMeta, spec networkingv1.IngressSpec) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: objectMeta,
		Spec:       spec,
	}
}

func RequestIngress(request IngressRequest) (*networkingv1.Ingress, error) {
	var (
		mutationErr error
	)
	ingress := newIngress(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, ingress, request.Client)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return ingress, fmt.Errorf("RequestIngress: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return ingress, nil
}

// CreateIngress creates the specified Ingress using the provided client.
func CreateIngress(ingress *networkingv1.Ingress, client cntrlClient.Client) error {
	return resource.CreateObject(ingress, client)
}

// UpdateIngress updates the specified Ingress using the provided client.
func UpdateIngress(ingress *networkingv1.Ingress, client cntrlClient.Client) error {
	return resource.UpdateObject(ingress, client)
}

// DeleteIngress deletes the Ingress with the given name and namespace using the provided client.
func DeleteIngress(name, namespace string, client cntrlClient.Client) error {
	ingress := &networkingv1.Ingress{}
	return resource.DeleteObject(name, namespace, ingress, client)
}

// GetIngress retrieves the Ingress with the given name and namespace using the provided client.
func GetIngress(name, namespace string, client cntrlClient.Client) (*networkingv1.Ingress, error) {
	ingress := &networkingv1.Ingress{}
	obj, err := resource.GetObject(name, namespace, ingress, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as a networkingv1.Ingress
	ingress, ok := obj.(*networkingv1.Ingress)
	if !ok {
		return nil, errors.New("failed to assert the object as a networkingv1.Ingress")
	}
	return ingress, nil
}

// ListIngresss returns a list of Ingress objects in the specified namespace using the provided client and list options.
func ListIngresss(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*networkingv1.IngressList, error) {
	ingressList := &networkingv1.IngressList{}
	obj, err := resource.ListObjects(namespace, ingressList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as a networkingv1.IngressList
	ingressList, ok := obj.(*networkingv1.IngressList)
	if !ok {
		return nil, errors.New("failed to assert the object as a networkingv1.IngressList")
	}
	return ingressList, nil
}
