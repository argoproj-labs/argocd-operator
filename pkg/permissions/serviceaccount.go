package permissions

import (
	"errors"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceAccountRequest struct {
	ObjectMeta metav1.ObjectMeta

	Instance *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newServiceAccount returns a new ServiceAccount instance.
func newServiceAccount(objMeta metav1.ObjectMeta) *corev1.ServiceAccount {

	return &corev1.ServiceAccount{
		ObjectMeta: objMeta,
	}
}

// RequestServiceAccount creates a new ServiceAccount object based on the provided ServiceAccountRequest.
func RequestServiceAccount(request ServiceAccountRequest) *corev1.ServiceAccount {
	return newServiceAccount(request.ObjectMeta)
}

// CreateServiceAccount creates the specified ServiceAccount using the provided client.
func CreateServiceAccount(sa *corev1.ServiceAccount, client cntrlClient.Client) error {
	return resource.CreateObject(sa, client)
}

// GetServiceAccount retrieves the ServiceAccount with the given name and namespace using the provided client.
func GetServiceAccount(name, namespace string, client cntrlClient.Client) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{}
	obj, err := resource.GetObject(name, namespace, sa, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as a corev1.ServiceAccount
	serviceAccount, ok := obj.(*corev1.ServiceAccount)
	if !ok {
		return nil, errors.New("failed to assert the object as a corev1.ServiceAccount")
	}
	return serviceAccount, nil
}

// ListServiceAccounts returns a list of ServiceAccount objects in the specified namespace using the provided client and list options.
func ListServiceAccounts(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*corev1.ServiceAccountList, error) {
	serviceAccountList := &corev1.ServiceAccountList{}
	obj, err := resource.ListObjects(namespace, serviceAccountList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as a corev1.ServiceAccountList
	serviceAccountList, ok := obj.(*corev1.ServiceAccountList)
	if !ok {
		return nil, errors.New("failed to assert the object as a corev1.ServiceAccountList")
	}
	return serviceAccountList, nil
}

// UpdateServiceAccount updates the specified ServiceAccount using the provided client.
func UpdateServiceAccount(sa *corev1.ServiceAccount, client cntrlClient.Client) error {
	return resource.UpdateObject(sa, client)
}

// DeleteServiceAccount deletes the ServiceAccount with the given name and namespace using the provided client.
func DeleteServiceAccount(name, namespace string, client cntrlClient.Client) error {
	serviceAccount := &corev1.ServiceAccount{}
	return resource.DeleteObject(name, namespace, serviceAccount, client)
}
