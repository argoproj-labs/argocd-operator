package permissions

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type RoleBindingRequest struct {
	ObjectMeta metav1.ObjectMeta
	RoleRef    rbacv1.RoleRef
	Subjects   []rbacv1.Subject
}

// newRoleBinding returns a new RoleBinding instance.
func newRoleBinding(objMeta metav1.ObjectMeta, roleRef rbacv1.RoleRef, subjects []rbacv1.Subject) *rbacv1.RoleBinding {

	return &rbacv1.RoleBinding{
		ObjectMeta: objMeta,
		RoleRef:    roleRef,
		Subjects:   subjects,
	}
}

// RequestRoleBinding creates a new RoleBinding based on the provided RoleBindingRequest parameters.
func RequestRoleBinding(request RoleBindingRequest) *rbacv1.RoleBinding {
	return newRoleBinding(request.ObjectMeta, request.RoleRef, request.Subjects)
}

// CreateRoleBinding creates a RoleBinding resource using the provided client.
func CreateRoleBinding(rb *rbacv1.RoleBinding, client cntrlClient.Client) error {
	return client.Create(context.TODO(), rb)
}

// GetRoleBinding retrieves an existing RoleBinding resource specified by its name and namespace.
func GetRoleBinding(name, namespace string, client cntrlClient.Client) (*rbacv1.RoleBinding, error) {
	existingRB := &rbacv1.RoleBinding{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingRB)
	if err != nil {
		return nil, err
	}
	return existingRB, nil
}

// ListRoleBindings lists all RoleBinding resources in a given namespace.
func ListRoleBindings(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*rbacv1.RoleBindingList, error) {
	existingRBs := &rbacv1.RoleBindingList{}
	err := client.List(context.TODO(), existingRBs, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingRBs, nil
}

// UpdateRoleBinding updates an existing RoleBinding resource.
func UpdateRoleBinding(rb *rbacv1.RoleBinding, client cntrlClient.Client) error {
	_, err := GetRoleBinding(rb.Name, rb.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), rb); err != nil {
		return err
	}

	return nil
}

// DeleteRoleBinding deletes an existing RoleBinding resource specified by its name and namespace.
func DeleteRoleBinding(name, namespace string, client cntrlClient.Client) error {
	existingRB, err := GetRoleBinding(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingRB); err != nil {
		return err
	}
	return nil
}
