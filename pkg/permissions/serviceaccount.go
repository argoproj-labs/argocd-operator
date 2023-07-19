package permissions

import (
	"context"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceaccountRequest struct {
	Name         string
	InstanceName string
	Namespace    string
	Component    string
	Client       *ctrlClient.Client
}

// newServiceAccount returns a new ServiceAccount instance.
func newServiceAccount(name, instanceName, namespace, component string) *corev1.ServiceAccount {
	saName := argoutil.GenerateResourceName(instanceName, component)
	if name != "" {
		saName = name
	}
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: namespace,
			Labels:    argoutil.LabelsForCluster(instanceName, component),
		},
	}
}
func RequestServiceaccount(request ServiceaccountRequest) *corev1.ServiceAccount {
	return newServiceAccount(request.Name, request.InstanceName, request.Namespace, request.Component)
}

func CreateServiceAccount(sa *corev1.ServiceAccount, client ctrlClient.Client) error {
	return client.Create(context.TODO(), sa)
}

func GetServiceAccount(name, namespace string, client ctrlClient.Client) (*corev1.ServiceAccount, error) {
	existingSA := &corev1.ServiceAccount{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingSA)
	if err != nil {
		return nil, err
	}
	return existingSA, nil
}

func ListServiceAccounts(namespace string, client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*corev1.ServiceAccountList, error) {
	existingSAs := &corev1.ServiceAccountList{}
	err := client.List(context.TODO(), existingSAs, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingSAs, nil
}

func UpdateServiceAccount(sa *corev1.ServiceAccount, client ctrlClient.Client) error {
	_, err := GetServiceAccount(sa.Name, sa.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), sa); err != nil {
		return err
	}

	return nil
}

func DeleteServiceAccount(name, namespace string, client ctrlClient.Client) error {
	existingSA, err := GetServiceAccount(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingSA); err != nil {
		return err
	}
	return nil
}
