package permissions

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterRoleRequest objects contain all the required information to produce a clusterRole object in return
type ClusterRoleRequest struct {
	Name              string
	InstanceName      string
	InstanceNamespace string
	Labels            map[string]string
	Annotations       map[string]string
	Component         string
	Rules             []rbacv1.PolicyRule

	// array of functions to mutate clusterRole before returning to requester
	Mutations []mutation.MutateFunc
	Client    interface{}
}

// newClusterRole returns a new clusterRole instance.
func newClusterRole(name, instanceName, instanceNamespace, component string, labels, annotations map[string]string, rules []rbacv1.PolicyRule) *rbacv1.ClusterRole {
	crName := argoutil.GenerateUniqueResourceName(instanceName, instanceNamespace, component)
	if name != "" {
		crName = name
	}

	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:        crName,
			Labels:      argoutil.MergeMaps(common.DefaultLabels(crName, instanceName, component), labels),
			Annotations: argoutil.MergeMaps(common.DefaultAnnotations(instanceName, instanceNamespace), annotations),
		},
		Rules: rules,
	}
}

// RequestClusterRole creates a ClusterRole object based on the provided ClusterRoleRequest.
// It applies any specified mutation functions to the ClusterRole.
func RequestClusterRole(request ClusterRoleRequest) (*rbacv1.ClusterRole, error) {
	var (
		mutationErr error
	)

	clusterRole := newClusterRole(request.Name, request.InstanceName, request.InstanceNamespace, request.Component, request.Labels, request.Annotations, request.Rules)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, clusterRole, request.Client)
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
func CreateClusterRole(role *rbacv1.ClusterRole, client ctrlClient.Client) error {
	return client.Create(context.TODO(), role)
}

// GetClusterRole retrieves the ClusterRole with the given name using the provided client.
func GetClusterRole(name string, client ctrlClient.Client) (*rbacv1.ClusterRole, error) {
	existingRole := &rbacv1.ClusterRole{}
	err := client.Get(context.TODO(), ctrlClient.ObjectKey{Name: name}, existingRole)
	if err != nil {
		return nil, err
	}
	return existingRole, nil
}

// ListClusterRoles returns a list of ClusterRole objects using the provided client and list options.
func ListClusterRoles(client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*rbacv1.ClusterRoleList, error) {
	existingRoles := &rbacv1.ClusterRoleList{}
	err := client.List(context.TODO(), existingRoles, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingRoles, nil
}

// UpdateClusterRole updates the specified ClusterRole using the provided client.
func UpdateClusterRole(role *rbacv1.ClusterRole, client ctrlClient.Client) error {
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
func DeleteClusterRole(name string, client ctrlClient.Client) error {
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
