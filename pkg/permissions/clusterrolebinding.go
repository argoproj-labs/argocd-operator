package permissions

import (
	"errors"

	"github.com/argoproj-labs/argocd-operator/pkg/resource"
	rbacv1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterRoleBindingRequest struct {
	ObjectMeta metav1.ObjectMeta
	RoleRef    rbacv1.RoleRef
	Subjects   []rbacv1.Subject
}

// newClusterclusterRoleBinding returns a new clusterclusterRoleBinding instance.
func newClusterRoleBinding(objMeta metav1.ObjectMeta, roleRef rbacv1.RoleRef, subjects []rbacv1.Subject) *rbacv1.ClusterRoleBinding {

	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: objMeta,
		RoleRef:    roleRef,
		Subjects:   subjects,
	}
}

// RequestClusterRoleBinding creates a ClusterRoleBinding object based on the provided ClusterRoleBindingRequest.
func RequestClusterRoleBinding(request ClusterRoleBindingRequest) *rbacv1.ClusterRoleBinding {
	return newClusterRoleBinding(request.ObjectMeta, request.RoleRef, request.Subjects)
}

// CreateClusterRoleBinding creates the specified ClusterRoleBinding using the provided client.
func CreateClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, client cntrlClient.Client) error {
	return resource.CreateClusterObject(crb, client)
}

// GetClusterRoleBinding retrieves the ClusterRoleBinding with the given name using the provided client.
func GetClusterRoleBinding(name string, client cntrlClient.Client) (*rbacv1.ClusterRoleBinding, error) {
	crb := &rbacv1.ClusterRoleBinding{}
	obj, err := resource.GetClusterObject(name, crb, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as an rbacv1.ClusterRoleBinding
	clusterRoleBinding, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok {
		return nil, errors.New("failed to assert the object as an rbacv1.ClusterRoleBinding")
	}
	return clusterRoleBinding, nil
}

// ListClusterRoleBindings returns a list of ClusterRoleBinding objects using the provided client and list options.
func ListClusterRoleBindings(client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*rbacv1.ClusterRoleBindingList, error) {
	clusterRoleBindingList := &rbacv1.ClusterRoleBindingList{}
	obj, err := resource.ListClusterObjects(clusterRoleBindingList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as an rbacv1.ClusterRoleBindingList
	clusterRoleBindingList, ok := obj.(*rbacv1.ClusterRoleBindingList)
	if !ok {
		return nil, errors.New("failed to assert the object as an rbacv1.ClusterRoleBindingList")
	}
	return clusterRoleBindingList, nil
}

// UpdateClusterRoleBinding updates the specified ClusterRoleBinding using the provided client.
func UpdateClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, client cntrlClient.Client) error {
	return resource.UpdateClusterObject(crb, client)
}

// DeleteClusterRoleBinding deletes the ClusterRoleBinding with the given name using the provided client.
func DeleteClusterRoleBinding(name string, client cntrlClient.Client) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	return resource.DeleteClusterObject(name, clusterRoleBinding, client)
}
