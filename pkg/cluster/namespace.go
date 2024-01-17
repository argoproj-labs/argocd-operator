package cluster

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"

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

// CreateNamespace creates the specified Namespace using the provided client.
func CreateNamespace(namespace *corev1.Namespace, client cntrlClient.Client) error {
	return resource.CreateObject(namespace, client)
}

// GetNamespace retrieves the Namespace with the given name using the provided client.
func GetNamespace(name string, client cntrlClient.Client) (*corev1.Namespace, error) {
	namespace := &corev1.Namespace{}
	obj, err := resource.GetObject(name, "", namespace, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as a corev1.Namespace
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return nil, errors.New("failed to assert the object as a corev1.Namespace")
	}
	return namespace, nil
}

// ListNamespaces returns a list of Namespace objects using the provided client and list options.
func ListNamespaces(client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*corev1.NamespaceList, error) {
	namespaceList := &corev1.NamespaceList{}
	obj, err := resource.ListObjects("", namespaceList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as a corev1.NamespaceList
	namespaceList, ok := obj.(*corev1.NamespaceList)
	if !ok {
		return nil, errors.New("failed to assert the object as a corev1.NamespaceList")
	}
	return namespaceList, nil
}

// UpdateNamespace updates the specified Namespace using the provided client.
func UpdateNamespace(namespace *corev1.Namespace, client cntrlClient.Client) error {
	return resource.UpdateObject(namespace, client)
}

// DeleteNamespace deletes the Namespace with the given name using the provided client.
func DeleteNamespace(name string, client cntrlClient.Client) error {
	namespace := &corev1.Namespace{}
	return resource.DeleteObject(name, "", namespace, client)
}
