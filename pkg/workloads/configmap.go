package workloads

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
)

// ConfigMapRequest objects contain all the required information to produce a configMap object in return
type ConfigMapRequest struct {
	ObjectMeta metav1.ObjectMeta
	Data       map[string]string

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    cntrlClient.Client
}

// newConfigMap returns a new ConfigMap instance for the given ArgoCD.
func newConfigMap(objMeta metav1.ObjectMeta, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: objMeta,
		Data:       data,
	}
}

func CreateConfigMap(configMap *corev1.ConfigMap, client cntrlClient.Client) error {
	return client.Create(context.TODO(), configMap)
}

// UpdateConfigMap updates the specified ConfigMap using the provided client.
func UpdateConfigMap(configMap *corev1.ConfigMap, client cntrlClient.Client) error {
	_, err := GetConfigMap(configMap.Name, configMap.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), configMap); err != nil {
		return err
	}
	return nil
}

func DeleteConfigMap(name, namespace string, client cntrlClient.Client) error {
	existingConfigMap, err := GetConfigMap(name, namespace, client)
	if err != nil {
		return err
	}

	if err := client.Delete(context.TODO(), existingConfigMap); err != nil {
		return err
	}
	return nil
}

func GetConfigMap(name, namespace string, client cntrlClient.Client) (*corev1.ConfigMap, error) {
	existingConfigMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingConfigMap)
	if err != nil {
		return nil, err
	}
	return existingConfigMap, nil
}

func ListConfigMaps(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*corev1.ConfigMapList, error) {
	existingConfigMaps := &corev1.ConfigMapList{}
	err := client.List(context.TODO(), existingConfigMaps, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingConfigMaps, nil
}

func RequestConfigMap(request ConfigMapRequest) (*corev1.ConfigMap, error) {
	var (
		mutationErr error
	)
	configMap := newConfigMap(request.ObjectMeta, request.Data)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, configMap, request.Client)
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
