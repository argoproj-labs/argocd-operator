package workloads

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	autoscaling "k8s.io/api/autoscaling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// HorizontalPodAutoscalerRequest objects contain all the required information to produce a horizontalPodAutoscaler object in return
type HorizontalPodAutoscalerRequest struct {
	Name              string
	InstanceName      string
	InstanceNamespace string
	Component         string
	Labels            map[string]string
	Annotations       map[string]string

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    interface{}
}

// newHorizontalPodAutoscaler returns a new HorizontalPodAutoscaler instance for the given ArgoCD.
func newHorizontalPodAutoscaler(name, instanceName, instanceNamespace, component string, labels, annotations map[string]string) *autoscaling.HorizontalPodAutoscaler {
	var horizontalPodAutoscalerName string
	if name != "" {
		horizontalPodAutoscalerName = name
	} else {
		horizontalPodAutoscalerName = argoutil.GenerateResourceName(instanceName, component)

	}
	return &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:        horizontalPodAutoscalerName,
			Namespace:   instanceNamespace,
			Labels:      argoutil.MergeMaps(common.DefaultLabels(horizontalPodAutoscalerName, instanceName, component), labels),
			Annotations: argoutil.MergeMaps(common.DefaultAnnotations(instanceName, instanceNamespace), annotations),
		},
	}
}

func CreateHorizontalPodAutoscaler(horizontalPodAutoscaler *autoscaling.HorizontalPodAutoscaler, client ctrlClient.Client) error {
	return client.Create(context.TODO(), horizontalPodAutoscaler)
}

// UpdateHorizontalPodAutoscaler updates the specified HorizontalPodAutoscaler using the provided client.
func UpdateHorizontalPodAutoscaler(horizontalPodAutoscaler *autoscaling.HorizontalPodAutoscaler, client ctrlClient.Client) error {
	_, err := GetHorizontalPodAutoscaler(horizontalPodAutoscaler.Name, horizontalPodAutoscaler.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), horizontalPodAutoscaler); err != nil {
		return err
	}
	return nil
}

func DeleteHorizontalPodAutoscaler(name, namespace string, client ctrlClient.Client) error {
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

func GetHorizontalPodAutoscaler(name, namespace string, client ctrlClient.Client) (*autoscaling.HorizontalPodAutoscaler, error) {
	existingHorizontalPodAutoscaler := &autoscaling.HorizontalPodAutoscaler{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingHorizontalPodAutoscaler)
	if err != nil {
		return nil, err
	}
	return existingHorizontalPodAutoscaler, nil
}

func ListHorizontalPodAutoscalers(namespace string, client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*autoscaling.HorizontalPodAutoscalerList, error) {
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
	horizontalPodAutoscaler := newHorizontalPodAutoscaler(request.Name, request.InstanceName, request.InstanceNamespace, request.Component, request.Labels, request.Annotations)

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
