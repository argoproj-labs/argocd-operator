package workloads

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretRequest objects contain all the required information to produce a secret object in return
type SecretRequest struct {
	ObjectMeta metav1.ObjectMeta
	Data       map[string][]byte
	StringData map[string]string
	Type       corev1.SecretType

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    cntrlClient.Client
}

// newSecret returns a new Secret instance for the given ArgoCD.
func newSecret(objMeta metav1.ObjectMeta, data map[string][]byte, stringData map[string]string, secretType corev1.SecretType) *corev1.Secret {

	return &corev1.Secret{
		ObjectMeta: objMeta,
		Data:       data,
		StringData: stringData,
		Type:       secretType,
	}
}

func CreateSecret(secret *corev1.Secret, client cntrlClient.Client) error {
	return client.Create(context.TODO(), secret)
}

// UpdateSecret updates the specified Secret using the provided client.
func UpdateSecret(secret *corev1.Secret, client cntrlClient.Client) error {
	_, err := GetSecret(secret.Name, secret.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), secret); err != nil {
		return err
	}
	return nil
}

func DeleteSecret(name, namespace string, client cntrlClient.Client) error {
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

func GetSecret(name, namespace string, client cntrlClient.Client) (*corev1.Secret, error) {
	existingSecret := &corev1.Secret{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingSecret)
	if err != nil {
		return nil, err
	}
	return existingSecret, nil
}

func ListSecrets(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*corev1.SecretList, error) {
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
	secret := newSecret(request.ObjectMeta, request.Data, request.StringData, request.Type)

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
