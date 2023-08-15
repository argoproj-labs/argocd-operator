package workloads

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretRequest objects contain all the required information to produce a secret object in return
type SecretRequest struct {
	Name         string
	InstanceName string
	Namespace    string
	Component    string
	Labels       map[string]string
	Annotations  map[string]string

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    interface{}
}

// newSecret returns a new Secret instance for the given ArgoCD.
func newSecret(name, instanceName, namespace, component string, labels, annotations map[string]string) *corev1.Secret {
	var secretName string
	if name != "" {
		secretName = name
	} else {
		secretName = argoutil.GenerateResourceName(instanceName, component)

	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Namespace:   namespace,
			Labels:      argoutil.MergeMaps(argoutil.LabelsForCluster(instanceName, component), labels),
			Annotations: annotations,
		},
	}
}

func CreateSecret(secret *corev1.Secret, client ctrlClient.Client) error {
	return client.Create(context.TODO(), secret)
}

// UpdateSecret updates the specified Secret using the provided client.
func UpdateSecret(secret *corev1.Secret, client ctrlClient.Client) error {
	_, err := GetSecret(secret.Name, secret.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), secret); err != nil {
		return err
	}
	return nil
}

func DeleteSecret(name, namespace string, client ctrlClient.Client) error {
	existingSecret, err := GetSecret(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingSecret); err != nil {
		return err
	}
	return nil
}

func GetSecret(name, namespace string, client ctrlClient.Client) (*corev1.Secret, error) {
	existingSecret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingSecret)
	if err != nil {
		return nil, err
	}
	return existingSecret, nil
}

func ListSecrets(namespace string, client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*corev1.SecretList, error) {
	existingSecrets := &corev1.SecretList{}
	err := client.List(context.TODO(), existingSecrets, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingSecrets, nil
}

func RequestSecret(request SecretRequest) (*corev1.Secret, error) {
	var (
		mutationErr error
	)
	secret := newSecret(request.Name, request.InstanceName, request.Namespace, request.Component, request.Labels, request.Annotations)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, secret, request.Client)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return secret, fmt.Errorf("RequestSecret: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return secret, nil
}
