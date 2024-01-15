package permissions

import (
	"errors"

	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// CreateRoleBinding creates the specified RoleBinding using the provided client.
func CreateRoleBinding(rb *rbacv1.RoleBinding, client cntrlClient.Client) error {
	return resource.CreateObject(rb, client)
}

// GetRoleBinding retrieves the RoleBinding with the given name and namespace using the provided client.
func GetRoleBinding(name, namespace string, client cntrlClient.Client) (*rbacv1.RoleBinding, error) {
	rb := &rbacv1.RoleBinding{}
	obj, err := resource.GetObject(name, namespace, rb, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as an rbacv1.RoleBinding
	roleBinding, ok := obj.(*rbacv1.RoleBinding)
	if !ok {
		return nil, errors.New("failed to assert the object as an rbacv1.RoleBinding")
	}
	return roleBinding, nil
}

// ListRoleBindings returns a list of RoleBinding objects in the specified namespace using the provided client and list options.
func ListRoleBindings(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*rbacv1.RoleBindingList, error) {
	roleBindingList := &rbacv1.RoleBindingList{}
	obj, err := resource.ListObjects(namespace, roleBindingList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as an rbacv1.RoleBindingList
	roleBindingList, ok := obj.(*rbacv1.RoleBindingList)
	if !ok {
		return nil, errors.New("failed to assert the object as an rbacv1.RoleBindingList")
	}
	return roleBindingList, nil
}

// UpdateRoleBinding updates the specified RoleBinding using the provided client.
func UpdateRoleBinding(rb *rbacv1.RoleBinding, client cntrlClient.Client) error {
	return resource.UpdateObject(rb, client)
}

// DeleteRoleBinding deletes the RoleBinding with the given name and namespace using the provided client.
func DeleteRoleBinding(name, namespace string, client cntrlClient.Client) error {
	roleBinding := &rbacv1.RoleBinding{}
	return resource.DeleteObject(name, namespace, roleBinding, client)
}
