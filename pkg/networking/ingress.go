package networking

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// IngressRequest objects contain all the required information to produce a ingress object in return
type IngressRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       networkingv1.IngressSpec

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    ctrlClient.Client
}

// newIngress returns a new Ingress instance for the given ArgoCD.
func newIngress(objectMeta metav1.ObjectMeta, spec networkingv1.IngressSpec) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: objectMeta,
		Spec:       spec,
	}
}

func CreateIngress(ingress *networkingv1.Ingress, client ctrlClient.Client) error {
	return client.Create(context.TODO(), ingress)
}

// UpdateIngress updates the specified Ingress using the provided client.
func UpdateIngress(ingress *networkingv1.Ingress, client ctrlClient.Client) error {
	_, err := GetIngress(ingress.Name, ingress.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), ingress); err != nil {
		return err
	}
	return nil
}

func DeleteIngress(name, namespace string, client ctrlClient.Client) error {
	existingIngress, err := GetIngress(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingIngress); err != nil {
		return err
	}
	return nil
}

func GetIngress(name, namespace string, client ctrlClient.Client) (*networkingv1.Ingress, error) {
	existingIngress := &networkingv1.Ingress{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingIngress)
	if err != nil {
		return nil, err
	}
	return existingIngress, nil
}

func ListIngresss(namespace string, client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*networkingv1.IngressList, error) {
	existingIngresss := &networkingv1.IngressList{}
	err := client.List(context.TODO(), existingIngresss, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingIngresss, nil
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
