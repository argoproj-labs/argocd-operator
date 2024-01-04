package permissions

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
)

// RoleRequest objects contain all the required information to produce a role object in return
type RoleRequest struct {
	ObjectMeta metav1.ObjectMeta
	Rules      []rbacv1.PolicyRule

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    cntrlClient.Client
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
			err := mutation(nil, role, request.Client)
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
	return client.Create(context.TODO(), role)
}

// GetRole retrieves the Role with the given name and namespace using the provided client.
func GetRole(name, namespace string, client cntrlClient.Client) (*rbacv1.Role, error) {
	existingRole := &rbacv1.Role{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingRole)
	if err != nil {
		return nil, err
	}
	return existingRole, nil
}

// ListRoles returns a list of Role objects in the specified namespace using the provided client and list options.
func ListRoles(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*rbacv1.RoleList, error) {
	existingRoles := &rbacv1.RoleList{}
	err := client.List(context.TODO(), existingRoles, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingRoles, nil
}

// UpdateRole updates the specified Role using the provided client.
func UpdateRole(role *rbacv1.Role, client cntrlClient.Client) error {
	_, err := GetRole(role.Name, role.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), role); err != nil {
		return err
	}

	return nil
}

// DeleteRole deletes the Role with the given name and namespace using the provided client.
// It ignores the "not found" error if the Role does not exist.
func DeleteRole(name, namespace string, client cntrlClient.Client) error {
	existingRole, err := GetRole(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingRole); err != nil {
		return err
	}
	return nil
}
