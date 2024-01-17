package permissions

import (
	"errors"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
)

// RoleRequest objects contain all the required information to produce a role object in return
type RoleRequest struct {
	ObjectMeta metav1.ObjectMeta
	Rules      []rbacv1.PolicyRule
	Instance   *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newRole returns a new Role instance.
func newRole(objMeta metav1.ObjectMeta, rules []rbacv1.PolicyRule) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: objMeta,
		Rules:      rules,
	}
}

// RequestRole creates a Role object based on the provided RoleRequest.
// It applies any specified mutation functions to the Role.
func RequestRole(request RoleRequest) (*rbacv1.Role, error) {
	var (
		mutationErr error
	)
	role := newRole(request.ObjectMeta, request.Rules)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, role, request.Client, request.MutationArgs)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return role, fmt.Errorf("RequestRole: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return role, nil
}

// CreateRole creates the specified Role using the provided client.
func CreateRole(role *rbacv1.Role, client cntrlClient.Client) error {
	return resource.CreateObject(role, client)
}

// GetRole retrieves the Role with the given name and namespace using the provided client.
func GetRole(name, namespace string, client cntrlClient.Client) (*rbacv1.Role, error) {
	role := &rbacv1.Role{}
	obj, err := resource.GetObject(name, namespace, role, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as an rbacv1.Role
	role, ok := obj.(*rbacv1.Role)
	if !ok {
		return nil, errors.New("failed to assert the object as an rbacv1.Role")
	}
	return role, nil
}

// ListRoles returns a list of Role objects in the specified namespace using the provided client and list options.
func ListRoles(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*rbacv1.RoleList, error) {
	roleList := &rbacv1.RoleList{}
	obj, err := resource.ListObjects(namespace, roleList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as an rbacv1.Role
	roleList, ok := obj.(*rbacv1.RoleList)
	if !ok {
		return nil, errors.New("failed to assert the object as an rbacv1.RoleList")
	}
	return roleList, nil
}

// UpdateRole updates the specified Role using the provided client.
func UpdateRole(role *rbacv1.Role, client cntrlClient.Client) error {
	return resource.UpdateObject(role, client)
}

// DeleteRole deletes the Role with the given name and namespace using the provided client.
func DeleteRole(name, namespace string, client cntrlClient.Client) error {
	role := &rbacv1.Role{}
	return resource.DeleteObject(name, namespace, role, client)
}
