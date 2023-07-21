package permissions

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterRoleRequest struct {
	Name              string
	InstanceName      string
	InstanceNamespace string
	Labels            map[string]string
	Annotations       map[string]string
	Component         string
	Rules             []rbacv1.PolicyRule

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    ctrlClient.Client
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
			Labels:      argoutil.MergeMaps(argoutil.LabelsForCluster(instanceName, component), labels),
			Annotations: argoutil.MergeMaps(argoutil.AnnotationsForCluster(instanceName, instanceNamespace), annotations),
		},
		Rules: rules,
	}
}

func RequestClusterRole(request ClusterRoleRequest) (*rbacv1.ClusterRole, error) {
	var errCount int
	clusterRole := newClusterRole(request.Name, request.InstanceName, request.InstanceNamespace, request.Component, request.Labels, request.Annotations, request.Rules)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, clusterRole, request.Client)
			if err != nil {
				errCount++
			}
		}
		if errCount > 0 {
			return clusterRole, fmt.Errorf("RequestClusterRole: one or more mutation functions could not be applied")
		}
	}
	return clusterRole, nil
}

func CreateClusterRole(role *rbacv1.ClusterRole, client ctrlClient.Client) error {
	return client.Create(context.TODO(), role)
}

func GetClusterRole(name string, client ctrlClient.Client) (*rbacv1.ClusterRole, error) {
	existingRole := &rbacv1.ClusterRole{}
	err := client.Get(context.TODO(), ctrlClient.ObjectKey{Name: name}, existingRole)
	if err != nil {
		return nil, err
	}
	return existingRole, nil
}

func ListClusterRoles(client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*rbacv1.ClusterRoleList, error) {
	existingRoles := &rbacv1.ClusterRoleList{}
	err := client.List(context.TODO(), existingRoles, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingRoles, nil
}

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
