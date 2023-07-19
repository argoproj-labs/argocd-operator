package permissions

import (
	"context"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterRoleBindingRequest struct {
	Name                string
	InstanceName        string
	InstanceNamespace   string
	InstanceAnnotations map[string]string
	Component           string
	Client              *ctrlClient.Client
}

// newClusterclusterRoleBinding returns a new clusterclusterRoleBinding instance.
func newClusterRoleBinding(name, instanceName, instanceNamespace, component string, instanceAnnotations map[string]string) *rbacv1.ClusterRoleBinding {
	crbName := argoutil.GenerateUniqueResourceName(instanceName, instanceNamespace, component)
	if name != "" {
		crbName = name
	}

	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        crbName,
			Labels:      argoutil.LabelsForCluster(instanceName, component),
			Annotations: argoutil.AnnotationsForCluster(instanceName, instanceNamespace, instanceAnnotations),
		},
	}
}

func RequestClusterRoleBinding(request ClusterRoleBindingRequest) *rbacv1.ClusterRoleBinding {
	return newClusterRoleBinding(request.Name, request.InstanceName, request.InstanceNamespace, request.Component, request.InstanceAnnotations)
}

func CreateClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, client ctrlClient.Client) error {
	return client.Create(context.TODO(), crb)
}

func GetClusterRoleBinding(name string, client ctrlClient.Client) (*rbacv1.ClusterRoleBinding, error) {
	existingCRB := &rbacv1.ClusterRoleBinding{}
	err := client.Get(context.TODO(), ctrlClient.ObjectKey{Name: name}, existingCRB)
	if err != nil {
		return nil, err
	}
	return existingCRB, nil
}

func ListClusterRoleBindings(client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*rbacv1.ClusterRoleBindingList, error) {
	existingCRBs := &rbacv1.ClusterRoleBindingList{}
	err := client.List(context.TODO(), existingCRBs, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingCRBs, nil
}

func UpdateClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, client ctrlClient.Client) error {
	_, err := GetClusterRoleBinding(crb.Name, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), crb); err != nil {
		return err
	}

	return nil
}

func DeleteClusterRoleBinding(name string, client ctrlClient.Client) error {
	existingCRB, err := GetClusterRoleBinding(name, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingCRB); err != nil {
		return err
	}
	return nil
}
