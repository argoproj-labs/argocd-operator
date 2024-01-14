package permissions

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
)

// ClusterRoleRequest objects contain all the required information to produce a clusterRole object in return
type ClusterRoleRequest struct {
	ObjectMeta metav1.ObjectMeta
	Rules      []rbacv1.PolicyRule
	Instance   *argoproj.ArgoCD

	// array of functions to mutate clusterRole before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newClusterRole returns a new clusterRole instance.
func newClusterRole(objMeta metav1.ObjectMeta, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: objMeta,
		Rules:      rules,
	}
}

// RequestClusterRole creates a ClusterRole object based on the provided ClusterRoleRequest.
// It applies any specified mutation functions to the ClusterRole.
func RequestClusterRole(request ClusterRoleRequest) (*rbacv1.ClusterRole, error) {
	var (
		mutationErr error
	)

	clusterRole := newClusterRole(request.ObjectMeta, request.Rules)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, clusterRole, request.Client, request.MutationArgs)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return clusterRole, fmt.Errorf("RequestClusterRole: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}
	return clusterRole, nil
}

// CreateClusterRole creates the specified ClusterRole using the provided client.
func CreateClusterRole(role *rbacv1.ClusterRole, client cntrlClient.Client) error {
	return client.Create(context.TODO(), role)
}

// GetClusterRole retrieves the ClusterRole with the given name using the provided client.
func GetClusterRole(name string, client cntrlClient.Client) (*rbacv1.ClusterRole, error) {
	existingRole := &rbacv1.ClusterRole{}
	err := client.Get(context.TODO(), cntrlClient.ObjectKey{Name: name}, existingRole)
	if err != nil {
		return nil, err
	}
	return existingRole, nil
}

// ListClusterRoles returns a list of ClusterRole objects using the provided client and list options.
func ListClusterRoles(client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*rbacv1.ClusterRoleList, error) {
	existingRoles := &rbacv1.ClusterRoleList{}
	err := client.List(context.TODO(), existingRoles, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingRoles, nil
}

// UpdateClusterRole updates the specified ClusterRole using the provided client.
func UpdateClusterRole(role *rbacv1.ClusterRole, client cntrlClient.Client) error {
	_, err := GetClusterRole(role.Name, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), role); err != nil {
		return err
	}

	return nil
}

// DeleteClusterRole deletes the ClusterRole with the given name using the provided client.
// It ignores the "not found" error if the ClusterRole does not exist.
func DeleteClusterRole(name string, client cntrlClient.Client) error {
	existingRole, err := GetClusterRole(name, client)
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
