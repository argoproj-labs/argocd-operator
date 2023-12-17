package permissions

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceAccountRequest struct {
	ObjectMeta metav1.ObjectMeta
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

// CreateServiceAccount creates the given ServiceAccount using the provided client.
func CreateServiceAccount(sa *corev1.ServiceAccount, client cntrlClient.Client) error {
	return client.Create(context.TODO(), sa)
}

// GetServiceAccount retrieves the ServiceAccount with the specified name and namespace from the client.
func GetServiceAccount(name, namespace string, client cntrlClient.Client) (*corev1.ServiceAccount, error) {
	existingSA := &corev1.ServiceAccount{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingSA)
	if err != nil {
		return nil, err
	}
	return existingSA, nil
}

// ListServiceAccounts lists all ServiceAccounts in the specified namespace using the provided client and list options.
func ListServiceAccounts(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*corev1.ServiceAccountList, error) {
	existingSAs := &corev1.ServiceAccountList{}
	err := client.List(context.TODO(), existingSAs, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingSAs, nil
}

// UpdateServiceAccount updates the given ServiceAccount using the provided client.
func UpdateServiceAccount(sa *corev1.ServiceAccount, client cntrlClient.Client) error {
	_, err := GetServiceAccount(sa.Name, sa.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), sa); err != nil {
		return err
	}

	return nil
}

// DeleteServiceAccount deletes the ServiceAccount with the specified name and namespace from the client.
func DeleteServiceAccount(name, namespace string, client cntrlClient.Client) error {
	existingSA, err := GetServiceAccount(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingSA); err != nil {
		return err
	}
	return nil
}
