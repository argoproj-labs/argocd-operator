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

// ConfigMapRequest objects contain all the required information to produce a configMap object in return
type ConfigMapRequest struct {
	Name              string
	InstanceName      string
	InstanceNamespace string
	Component         string
	Labels            map[string]string
	Annotations       map[string]string

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    interface{}
}

// newConfigMap returns a new ConfigMap instance for the given ArgoCD.
func newConfigMap(name, instanceName, instanceNamespace, component string, labels, annotations map[string]string) *corev1.ConfigMap {
	var configMapName string
	if name != "" {
		configMapName = name
	} else {
		configMapName = argoutil.GenerateResourceName(instanceName, component)

	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        configMapName,
			Namespace:   instanceNamespace,
			Labels:      argoutil.MergeMaps(argoutil.LabelsForCluster(instanceName, component), labels),
			Annotations: argoutil.MergeMaps(argoutil.AnnotationsForCluster(instanceName, instanceNamespace), annotations),
		},
	}
}

func CreateConfigMap(configMap *corev1.ConfigMap, client ctrlClient.Client) error {
	return client.Create(context.TODO(), configMap)
}

// UpdateConfigMap updates the specified ConfigMap using the provided client.
func UpdateConfigMap(configMap *corev1.ConfigMap, client ctrlClient.Client) error {
	_, err := GetConfigMap(configMap.Name, configMap.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), configMap); err != nil {
		return err
	}
	return nil
}

func DeleteConfigMap(name, namespace string, client ctrlClient.Client) error {
	existingConfigMap, err := GetConfigMap(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingConfigMap); err != nil {
		return err
	}
	return nil
}

func GetConfigMap(name, namespace string, client ctrlClient.Client) (*corev1.ConfigMap, error) {
	existingConfigMap := &corev1.ConfigMap{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingConfigMap)
	if err != nil {
		return nil, err
	}
	return existingConfigMap, nil
}

func ListConfigMaps(namespace string, client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*corev1.ConfigMapList, error) {
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
	configMap := newConfigMap(request.Name, request.InstanceName, request.InstanceNamespace, request.Component, request.Labels, request.Annotations)

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
