package cluster

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespaceRequest objects contain all the required information to produce a namespace object in return
type NamespaceRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       corev1.NamespaceSpec
	Instance   *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

func newNamespace(objMeta metav1.ObjectMeta, spec corev1.NamespaceSpec) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: objMeta,
		Spec:       spec,
	}
}

// RequestNamespace accepts a NamespaceRequest object and returns a populated namespace resource.
// It also runs any specified mutations to the namespace resource before returning it
func RequestNamespace(request NamespaceRequest) (*corev1.Namespace, error) {
	var (
		mutationErr error
	)
	namespace := newNamespace(request.ObjectMeta, request.Spec)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, namespace, request.Client, request.MutationArgs)
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

// CreateNamespace creates a provided namespace on the cluster
func CreateNamespace(namespace *corev1.Namespace, client cntrlClient.Client) error {
	return client.Create(context.TODO(), namespace)
}

// GetNamespace retrieves a specified namespace from the cluster
func GetNamespace(name string, client cntrlClient.Client) (*corev1.Namespace, error) {
	existingNamespace := &corev1.Namespace{}
	err := client.Get(context.TODO(), cntrlClient.ObjectKey{Name: name}, existingNamespace)
	if err != nil {
		return nil, err
	}
	return existingNamespace, nil
}

// ListNamespace returns a list of namespaces from the cluster after applying specified listOptions
func ListNamespaces(client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*corev1.NamespaceList, error) {
	existingNamespaces := &corev1.NamespaceList{}
	err := client.List(context.TODO(), existingNamespaces, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingNamespaces, nil
}

// UpdateNamespace updates a provided namespace on the cluster
func UpdateNamespace(namespace *corev1.Namespace, client cntrlClient.Client) error {
	_, err := GetNamespace(namespace.Name, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), namespace); err != nil {
		return err
	}

	return nil
}

// DeleteNamespace deletes a specified namespace from the cluster
func DeleteNamespace(name string, client cntrlClient.Client) error {
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
