package workloads

import (
	"errors"
	"fmt"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
)

// HorizontalPodAutoscalerRequest objects contain all the required information to produce a horizontalPodAutoscaler object in return
type HorizontalPodAutoscalerRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       autoscalingv1.HorizontalPodAutoscalerSpec
	Instance   *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newHorizontalPodAutoscaler returns a new HorizontalPodAutoscaler instance for the given ArgoCD.
func newHorizontalPodAutoscaler(objMeta metav1.ObjectMeta, spec autoscalingv1.HorizontalPodAutoscalerSpec) *autoscalingv1.HorizontalPodAutoscaler {

	return &autoscalingv1.HorizontalPodAutoscaler{
		ObjectMeta: objMeta,
		Spec:       spec,
	}
}

func RequestHorizontalPodAutoscaler(request HorizontalPodAutoscalerRequest) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	var (
		mutationErr error
	)
	horizontalPodAutoscaler := newHorizontalPodAutoscaler(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, horizontalPodAutoscaler, request.Client, request.MutationArgs, request.MutationArgs)
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
func CreateHorizontalPodAutoscaler(hpa *autoscalingv1.HorizontalPodAutoscaler, client cntrlClient.Client) error {
	return resource.CreateObject(hpa, client)
}

// UpdateHorizontalPodAutoscaler updates the specified HorizontalPodAutoscaler using the provided client.
func UpdateHorizontalPodAutoscaler(hpa *autoscalingv1.HorizontalPodAutoscaler, client cntrlClient.Client) error {
	return resource.UpdateObject(hpa, client)
}

// DeleteHorizontalPodAutoscaler deletes the HorizontalPodAutoscaler with the given name and namespace using the provided client.
func DeleteHorizontalPodAutoscaler(name, namespace string, client cntrlClient.Client) error {
	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	return resource.DeleteObject(name, namespace, hpa, client)
}

// GetHorizontalPodAutoscaler retrieves the HorizontalPodAutoscaler with the given name and namespace using the provided client.
func GetHorizontalPodAutoscaler(name, namespace string, client cntrlClient.Client) (*autoscalingv1.HorizontalPodAutoscaler, error) {
	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	obj, err := resource.GetObject(name, namespace, hpa, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as an autoscalingv1.HorizontalPodAutoscaler
	hpa, ok := obj.(*autoscalingv1.HorizontalPodAutoscaler)
	if !ok {
		return nil, errors.New("failed to assert the object as an autoscalingv1.HorizontalPodAutoscaler")
	}
	return hpa, nil
}

// ListHorizontalPodAutoscalers returns a list of HorizontalPodAutoscaler objects in the specified namespace using the provided client and list options.
func ListHorizontalPodAutoscalers(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*autoscalingv1.HorizontalPodAutoscalerList, error) {
	hpaList := &autoscalingv1.HorizontalPodAutoscalerList{}
	obj, err := resource.ListObjects(namespace, hpaList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as an autoscalingv1.HorizontalPodAutoscalerList
	hpaList, ok := obj.(*autoscalingv1.HorizontalPodAutoscalerList)
	if !ok {
		return nil, errors.New("failed to assert the object as an autoscalingv1.HorizontalPodAutoscalerList")
	}
	return hpaList, nil
}
