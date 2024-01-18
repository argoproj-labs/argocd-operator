package workloads

import (
	"errors"
	"fmt"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretRequest objects contain all the required information to produce a secret object in return
type SecretRequest struct {
	ObjectMeta metav1.ObjectMeta
	Data       map[string][]byte
	StringData map[string]string
	Type       corev1.SecretType
	Instance   *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
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

func RequestSecret(request SecretRequest) (*corev1.Secret, error) {
	var (
		mutationErr error
	)
	secret := newSecret(request.ObjectMeta, request.Data, request.StringData, request.Type)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, secret, request.Client, request.MutationArgs)
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

// CreateSecret creates the specified Secret using the provided client.
func CreateSecret(secret *corev1.Secret, client cntrlClient.Client) error {
	return resource.CreateObject(secret, client)
}

// UpdateSecret updates the specified Secret using the provided client.
func UpdateSecret(secret *corev1.Secret, client cntrlClient.Client) error {
	return resource.UpdateObject(secret, client)
}

// DeleteSecret deletes the Secret with the given name and namespace using the provided client.
func DeleteSecret(name, namespace string, client cntrlClient.Client) error {
	secret := &corev1.Secret{}
	return resource.DeleteObject(name, namespace, secret, client)
}

// GetSecret retrieves the Secret with the given name and namespace using the provided client.
func GetSecret(name, namespace string, client cntrlClient.Client) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	obj, err := resource.GetObject(name, namespace, secret, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as a corev1.Secret
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil, errors.New("failed to assert the object as a corev1.Secret")
	}
	return secret, nil
}

// ListSecrets returns a list of Secret objects in the specified namespace using the provided client and list options.
func ListSecrets(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*corev1.SecretList, error) {
	secretList := &corev1.SecretList{}
	obj, err := resource.ListObjects(namespace, secretList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as a corev1.SecretList
	secretList, ok := obj.(*corev1.SecretList)
	if !ok {
		return nil, errors.New("failed to assert the object as a corev1.SecretList")
	}
	return secretList, nil
}
