package workloads

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
)

// StatefulRequest objects contain all the required information to produce a stateful object in return
type StatefulSetRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       appsv1.StatefulSetSpec
	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    cntrlClient.Client
}

// newStateful returns a new Stateful instance for the given ArgoCD.
func newStatefulSet(objMeta metav1.ObjectMeta, spec appsv1.StatefulSetSpec) *appsv1.StatefulSet {

	return &appsv1.StatefulSet{
		ObjectMeta: objMeta,
		Spec:       spec,
	}
}

func CreateStatefulSet(StatefulSet *appsv1.StatefulSet, client cntrlClient.Client) error {
	return client.Create(context.TODO(), StatefulSet)
}

// UpdateStatefulSet updates the specified StatefulSet using the provided client.
func UpdateStatefulSet(StatefulSet *appsv1.StatefulSet, client cntrlClient.Client) error {
	_, err := GetStatefulSet(StatefulSet.Name, StatefulSet.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), StatefulSet); err != nil {
		return err
	}
	return nil
}

func DeleteStatefulSet(name, namespace string, client cntrlClient.Client) error {
	existingStatefulSet, err := GetStatefulSet(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingStatefulSet); err != nil {
		return err
	}
	return nil
}

func GetStatefulSet(name, namespace string, client cntrlClient.Client) (*appsv1.StatefulSet, error) {
	existingStatefulSet := &appsv1.StatefulSet{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingStatefulSet)
	if err != nil {
		return nil, err
	}
	return existingStatefulSet, nil
}

func ListStatefulSets(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*appsv1.StatefulSetList, error) {
	existingStatefulSets := &appsv1.StatefulSetList{}
	err := client.List(context.TODO(), existingStatefulSets, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingStatefulSets, nil
}

func RequestStatefulSet(request StatefulSetRequest) (*appsv1.StatefulSet, error) {
	var (
		mutationErr error
	)
	StatefulSet := newStatefulSet(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, StatefulSet, request.Client)
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
