package workloads

import (
	"context"
	"fmt"

	autoscaling "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
)

// HorizontalPodAutoscalerRequest objects contain all the required information to produce a horizontalPodAutoscaler object in return
type HorizontalPodAutoscalerRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       autoscaling.HorizontalPodAutoscalerSpec

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    cntrlClient.Client
}

// newHorizontalPodAutoscaler returns a new HorizontalPodAutoscaler instance for the given ArgoCD.
func newHorizontalPodAutoscaler(objMeta metav1.ObjectMeta, spec autoscaling.HorizontalPodAutoscalerSpec) *autoscaling.HorizontalPodAutoscaler {

	return &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: objMeta,
		Spec:       spec,
	}
}

func CreateHorizontalPodAutoscaler(horizontalPodAutoscaler *autoscaling.HorizontalPodAutoscaler, client cntrlClient.Client) error {
	return client.Create(context.TODO(), horizontalPodAutoscaler)
}

// UpdateHorizontalPodAutoscaler updates the specified HorizontalPodAutoscaler using the provided client.
func UpdateHorizontalPodAutoscaler(horizontalPodAutoscaler *autoscaling.HorizontalPodAutoscaler, client cntrlClient.Client) error {
	_, err := GetHorizontalPodAutoscaler(horizontalPodAutoscaler.Name, horizontalPodAutoscaler.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), horizontalPodAutoscaler); err != nil {
		return err
	}
	return nil
}

func DeleteHorizontalPodAutoscaler(name, namespace string, client cntrlClient.Client) error {
	existingHorizontalPodAutoscaler, err := GetHorizontalPodAutoscaler(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingHorizontalPodAutoscaler); err != nil {
		return err
	}
	return nil
}

func GetHorizontalPodAutoscaler(name, namespace string, client cntrlClient.Client) (*autoscaling.HorizontalPodAutoscaler, error) {
	existingHorizontalPodAutoscaler := &autoscaling.HorizontalPodAutoscaler{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingHorizontalPodAutoscaler)
	if err != nil {
		return nil, err
	}
	return existingHorizontalPodAutoscaler, nil
}

func ListHorizontalPodAutoscalers(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*autoscaling.HorizontalPodAutoscalerList, error) {
	existingHorizontalPodAutoscalers := &autoscaling.HorizontalPodAutoscalerList{}
	err := client.List(context.TODO(), existingHorizontalPodAutoscalers, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingHorizontalPodAutoscalers, nil
}

func RequestHorizontalPodAutoscaler(request HorizontalPodAutoscalerRequest) (*autoscaling.HorizontalPodAutoscaler, error) {
	var (
		mutationErr error
	)
	horizontalPodAutoscaler := newHorizontalPodAutoscaler(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, horizontalPodAutoscaler, request.Client)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return horizontalPodAutoscaler, fmt.Errorf("RequestHorizontalPodAutoscaler: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return horizontalPodAutoscaler, nil
}
