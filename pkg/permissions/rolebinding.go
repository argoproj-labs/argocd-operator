package permissions

import (
	"context"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type RoleBindingRequest struct {
	Name         string
	InstanceName string
	Namespace    string
	Component    string
	Client       *ctrlClient.Client
}

// newRoleBinding returns a new RoleBinding instance.
func newRoleBinding(name, instanceName, namespace, component string) *rbacv1.RoleBinding {
	rbName := argoutil.GenerateResourceName(instanceName, component)
	if name != "" {
		rbName = name
	}
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rbName,
			Namespace: namespace,
			Labels:    argoutil.LabelsForCluster(instanceName, component),
		},
	}
}

func RequestRoleBinding(request RoleBindingRequest) *rbacv1.RoleBinding {
	return newRoleBinding(request.Name, request.InstanceName, request.Namespace, request.Component)
}

func CreateRoleBinding(rb *rbacv1.RoleBinding, client ctrlClient.Client) error {
	return client.Create(context.TODO(), rb)
}

func GetRoleBinding(name, namespace string, client ctrlClient.Client) (*rbacv1.RoleBinding, error) {
	existingRB := &rbacv1.RoleBinding{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingRB)
	if err != nil {
		return nil, err
	}
	return existingRB, nil
}

func ListRoleBindings(namespace string, client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*rbacv1.RoleBindingList, error) {
	existingRBs := &rbacv1.RoleBindingList{}
	err := client.List(context.TODO(), existingRBs, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingRBs, nil
}

func UpdateRoleBinding(rb *rbacv1.RoleBinding, client ctrlClient.Client) error {
	_, err := GetRoleBinding(rb.Name, rb.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), rb); err != nil {
		return err
	}

	return nil
}

func DeleteRoleBinding(name, namespace string, client ctrlClient.Client) error {
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
