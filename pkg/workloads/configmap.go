package workloads

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
)

// ConfigMapRequest objects contain all the required information to produce a configMap object in return
type ConfigMapRequest struct {
	ObjectMeta metav1.ObjectMeta
	Data       map[string]string
	Instance   *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newConfigMap returns a new ConfigMap instance for the given ArgoCD.
func newConfigMap(objMeta metav1.ObjectMeta, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: objMeta,
		Data:       data,
	}
}

func RequestConfigMap(request ConfigMapRequest) (*corev1.ConfigMap, error) {
	var (
		mutationErr error
	)
	configMap := newConfigMap(request.ObjectMeta, request.Data)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, configMap, request.Client, request.MutationArgs)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return configMap, fmt.Errorf("RequestConfigMap: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return configMap, nil
}

// CreateConfigMap creates the specified ConfigMap using the provided client.
func CreateConfigMap(configMap *corev1.ConfigMap, client cntrlClient.Client) error {
	return resource.CreateObject(configMap, client)
}

// UpdateConfigMap updates the specified ConfigMap using the provided client.
func UpdateConfigMap(configMap *corev1.ConfigMap, client cntrlClient.Client) error {
	return resource.UpdateObject(configMap, client)
}

// DeleteConfigMap deletes the ConfigMap with the given name and namespace using the provided client.
func DeleteConfigMap(name, namespace string, client cntrlClient.Client) error {
	configMap := &corev1.ConfigMap{}
	return resource.DeleteObject(name, namespace, configMap, client)
}

// GetConfigMap retrieves the ConfigMap with the given name and namespace using the provided client.
func GetConfigMap(name, namespace string, client cntrlClient.Client) (*corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	obj, err := resource.GetObject(name, namespace, configMap, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as a corev1.ConfigMap
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok {
		return nil, errors.New("failed to assert the object as a corev1.ConfigMap")
	}
	return configMap, nil
}

// ListConfigMaps returns a list of ConfigMap objects in the specified namespace using the provided client and list options.
func ListConfigMaps(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*corev1.ConfigMapList, error) {
	configMapList := &corev1.ConfigMapList{}
	obj, err := resource.ListObjects(namespace, configMapList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as a corev1.ConfigMapList
	configMapList, ok := obj.(*corev1.ConfigMapList)
	if !ok {
		return nil, errors.New("failed to assert the object as a corev1.ConfigMapList")
	}
	return configMapList, nil
}
