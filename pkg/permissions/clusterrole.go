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
func CreateClusterRole(clusterRole *rbacv1.ClusterRole, client cntrlClient.Client) error {
	return resource.CreateClusterObject(clusterRole, client)
}

// GetClusterRole retrieves the ClusterRole with the given name using the provided client.
func GetClusterRole(name string, client cntrlClient.Client) (*rbacv1.ClusterRole, error) {
	clusterRole := &rbacv1.ClusterRole{}
	obj, err := resource.GetClusterObject(name, clusterRole, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as an rbacv1.ClusterRole
	clusterRole, ok := obj.(*rbacv1.ClusterRole)
	if !ok {
		return nil, errors.New("failed to assert the object as an rbacv1.ClusterRole")
	}
	return clusterRole, nil
}

// ListClusterRoles returns a list of ClusterRole objects using the provided client and list options.
func ListClusterRoles(client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*rbacv1.ClusterRoleList, error) {
	clusterRoleList := &rbacv1.ClusterRoleList{}
	obj, err := resource.ListClusterObjects(clusterRoleList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as an rbacv1.ClusterRoleList
	clusterRoleList, ok := obj.(*rbacv1.ClusterRoleList)
	if !ok {
		return nil, errors.New("failed to assert the object as an rbacv1.ClusterRoleList")
	}
	return clusterRoleList, nil
}

// UpdateClusterRole updates the specified ClusterRole using the provided client.
func UpdateClusterRole(clusterRole *rbacv1.ClusterRole, client cntrlClient.Client) error {
	return resource.UpdateClusterObject(clusterRole, client)
}

// DeleteClusterRole deletes the ClusterRole with the given name using the provided client.
func DeleteClusterRole(name string, client cntrlClient.Client) error {
	clusterRole := &rbacv1.ClusterRole{}
	return resource.DeleteClusterObject(name, clusterRole, client)
}
