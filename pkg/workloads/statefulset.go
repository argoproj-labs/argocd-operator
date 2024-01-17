package workloads

import (
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
)

// StatefulRequest objects contain all the required information to produce a stateful object in return
type StatefulSetRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       appsv1.StatefulSetSpec
	Instance   *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newStateful returns a new Stateful instance for the given ArgoCD.
func newStatefulSet(objMeta metav1.ObjectMeta, spec appsv1.StatefulSetSpec) *appsv1.StatefulSet {

	return &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Spec:       spec,
	}
}

func RequestStatefulSet(request StatefulSetRequest) (*appsv1.StatefulSet, error) {
	var (
		mutationErr error
	)
	StatefulSet := newStatefulSet(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, StatefulSet, request.Client, request.MutationArgs, request.MutationArgs)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return StatefulSet, fmt.Errorf("RequestStatefulSet: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return StatefulSet, nil
}

// CreateStatefulSet creates the specified StatefulSet using the provided client.
func CreateStatefulSet(statefulSet *appsv1.StatefulSet, client cntrlClient.Client) error {
	return resource.CreateObject(statefulSet, client)
}

// UpdateStatefulSet updates the specified StatefulSet using the provided client.
func UpdateStatefulSet(statefulSet *appsv1.StatefulSet, client cntrlClient.Client) error {
	return resource.UpdateObject(statefulSet, client)
}

// DeleteStatefulSet deletes the StatefulSet with the given name and namespace using the provided client.
func DeleteStatefulSet(name, namespace string, client cntrlClient.Client) error {
	statefulSet := &appsv1.StatefulSet{}
	return resource.DeleteObject(name, namespace, statefulSet, client)
}

// GetStatefulSet retrieves the StatefulSet with the given name and namespace using the provided client.
func GetStatefulSet(name, namespace string, client cntrlClient.Client) (*appsv1.StatefulSet, error) {
	statefulSet := &appsv1.StatefulSet{}
	obj, err := resource.GetObject(name, namespace, statefulSet, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as an appsv1.StatefulSet
	statefulSet, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		return nil, errors.New("failed to assert the object as an appsv1.StatefulSet")
	}
	return statefulSet, nil
}

// ListStatefulSets returns a list of StatefulSet objects in the specified namespace using the provided client and list options.
func ListStatefulSets(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*appsv1.StatefulSetList, error) {
	statefulSetList := &appsv1.StatefulSetList{}
	obj, err := resource.ListObjects(namespace, statefulSetList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as an appsv1.StatefulSetList
	statefulSetList, ok := obj.(*appsv1.StatefulSetList)
	if !ok {
		return nil, errors.New("failed to assert the object as an appsv1.StatefulSetList")
	}
	return statefulSetList, nil
}
