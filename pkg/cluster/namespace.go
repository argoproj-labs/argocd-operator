package cluster

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type NamespaceRequest struct {
	Name        string
	Labels      map[string]string
	Annotations map[string]string

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    ctrlClient.Client
}

func newNamespace(name string, labels, annotations map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
	}
}

func RequestNamespace(request NamespaceRequest) (*corev1.Namespace, error) {
	var (
		mutationErr error
	)
	namespace := newNamespace(request.Name, request.Labels, request.Annotations)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, namespace, request.Client)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return namespace, fmt.Errorf("RequestNamespace: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}
	return namespace, nil
}

func CreateNamespace(namespace *corev1.Namespace, client ctrlClient.Client) error {
	return client.Create(context.TODO(), namespace)
}

func GetNamespace(name string, client ctrlClient.Client) (*corev1.Namespace, error) {
	existingNamespace := &corev1.Namespace{}
	err := client.Get(context.TODO(), ctrlClient.ObjectKey{Name: name}, existingNamespace)
	if err != nil {
		return nil, err
	}
	return existingNamespace, nil
}

func ListNamespaces(client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*corev1.NamespaceList, error) {
	existingNamespaces := &corev1.NamespaceList{}
	err := client.List(context.TODO(), existingNamespaces, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingNamespaces, nil
}

func UpdateNamespace(namespace *corev1.Namespace, client ctrlClient.Client) error {
	_, err := GetNamespace(namespace.Name, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), namespace); err != nil {
		return err
	}

	return nil
}

func DeleteNamespace(name string, client ctrlClient.Client) error {
	existingNamespace, err := GetNamespace(name, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingNamespace); err != nil {
		return err
	}
	return nil
}