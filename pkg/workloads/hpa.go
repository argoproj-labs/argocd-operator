package workloads

import (
	"errors"
	"fmt"

	autoscaling "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
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

// CreateHorizontalPodAutoscaler creates the specified HorizontalPodAutoscaler using the provided client.
func CreateHorizontalPodAutoscaler(hpa *autoscaling.HorizontalPodAutoscaler, client cntrlClient.Client) error {
	return resource.CreateObject(hpa, client)
}

// UpdateHorizontalPodAutoscaler updates the specified HorizontalPodAutoscaler using the provided client.
func UpdateHorizontalPodAutoscaler(hpa *autoscaling.HorizontalPodAutoscaler, client cntrlClient.Client) error {
	return resource.UpdateObject(hpa, client)
}

// DeleteHorizontalPodAutoscaler deletes the HorizontalPodAutoscaler with the given name and namespace using the provided client.
// It ignores the "not found" error if the HorizontalPodAutoscaler does not exist.
func DeleteHorizontalPodAutoscaler(name, namespace string, client cntrlClient.Client) error {
	hpa := &autoscaling.HorizontalPodAutoscaler{}
	return resource.DeleteObject(name, namespace, hpa, client)
}

// GetHorizontalPodAutoscaler retrieves the HorizontalPodAutoscaler with the given name and namespace using the provided client.
func GetHorizontalPodAutoscaler(name, namespace string, client cntrlClient.Client) (*autoscaling.HorizontalPodAutoscaler, error) {
	hpa := &autoscaling.HorizontalPodAutoscaler{}
	obj, err := resource.GetObject(name, namespace, hpa, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as an autoscaling.HorizontalPodAutoscaler
	hpa, ok := obj.(*autoscaling.HorizontalPodAutoscaler)
	if !ok {
		return nil, errors.New("failed to assert the object as an autoscaling.HorizontalPodAutoscaler")
	}
	return hpa, nil
}

// ListHorizontalPodAutoscalers returns a list of HorizontalPodAutoscaler objects in the specified namespace using the provided client and list options.
func ListHorizontalPodAutoscalers(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*autoscaling.HorizontalPodAutoscalerList, error) {
	hpaList := &autoscaling.HorizontalPodAutoscalerList{}
	obj, err := resource.ListObjects(namespace, hpaList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as an autoscaling.HorizontalPodAutoscalerList
	hpaList, ok := obj.(*autoscaling.HorizontalPodAutoscalerList)
	if !ok {
		return nil, errors.New("failed to assert the object as an autoscaling.HorizontalPodAutoscalerList")
	}
	return hpaList, nil
}
