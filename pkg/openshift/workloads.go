package openshift

import (
	"context"
	"fmt"

	oappsv1 "github.com/openshift/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
)

// DeploymentConfigRequest objects contain all the required information to produce a deploymentConfig object in return
type DeploymentConfigRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       oappsv1.DeploymentConfigSpec

	// array of functions to mutate role before returning to requester
	Mutations []mutation.MutateFunc
	Client    cntrlClient.Client
}

// newDeploymentConfig returns a new DeploymentConfig instance for the given ArgoCD.
func newDeploymentConfig(objMeta metav1.ObjectMeta, spec oappsv1.DeploymentConfigSpec) *oappsv1.DeploymentConfig {
	return &oappsv1.DeploymentConfig{
		ObjectMeta: objMeta,
		Spec:       spec,
	}
}

func CreateDeploymentConfig(deploymentConfig *oappsv1.DeploymentConfig, client cntrlClient.Client) error {
	return client.Create(context.TODO(), deploymentConfig)
}

// UpdateDeploymentConfig updates the specified DeploymentConfig using the provided client.
func UpdateDeploymentConfig(deploymentConfig *oappsv1.DeploymentConfig, client cntrlClient.Client) error {
	_, err := GetDeploymentConfig(deploymentConfig.Name, deploymentConfig.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), deploymentConfig); err != nil {
		return err
	}
	return nil
}

func DeleteDeploymentConfig(name, namespace string, client cntrlClient.Client) error {
	existingDeploymentConfig, err := GetDeploymentConfig(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingDeploymentConfig); err != nil {
		return err
	}
	return nil
}

func GetDeploymentConfig(name, namespace string, client cntrlClient.Client) (*oappsv1.DeploymentConfig, error) {
	existingDeploymentConfig := &oappsv1.DeploymentConfig{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingDeploymentConfig)
	if err != nil {
		return nil, err
	}
	return existingDeploymentConfig, nil
}

func ListDeploymentConfigs(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*oappsv1.DeploymentConfigList, error) {
	existingDeploymentConfigs := &oappsv1.DeploymentConfigList{}
	err := client.List(context.TODO(), existingDeploymentConfigs, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingDeploymentConfigs, nil
}

func RequestDeploymentConfig(request DeploymentConfigRequest) (*oappsv1.DeploymentConfig, error) {
	var (
		mutationErr error
	)
	deploymentConfig := newDeploymentConfig(request.ObjectMeta, request.Spec)
	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, deploymentConfig, request.Client)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return deploymentConfig, fmt.Errorf("RequestDeploymentConfig: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return deploymentConfig, nil
}
