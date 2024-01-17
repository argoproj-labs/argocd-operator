package openshift

import (
	"errors"
	"fmt"

	oappsv1 "github.com/openshift/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cntrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/resource"
)

// DeploymentConfigRequest objects contain all the required information to produce a deploymentConfig object in return
type DeploymentConfigRequest struct {
	ObjectMeta metav1.ObjectMeta
	Spec       oappsv1.DeploymentConfigSpec
	Instance   *argoproj.ArgoCD

	// array of functions to mutate obj before returning to requester
	Mutations []mutation.MutateFunc
	// array of arguments to pass to the mutation funcs
	MutationArgs []interface{}
	Client       cntrlClient.Client
}

// newDeploymentConfig returns a new DeploymentConfig instance for the given ArgoCD.
func newDeploymentConfig(objMeta metav1.ObjectMeta, spec oappsv1.DeploymentConfigSpec) *oappsv1.DeploymentConfig {
	return &oappsv1.DeploymentConfig{
		ObjectMeta: objMeta,
		Spec:       spec,
	}
}

func RequestDeploymentConfig(request DeploymentConfigRequest) (*oappsv1.DeploymentConfig, error) {
	var (
		mutationErr error
	)
	deploymentConfig := newDeploymentConfig(request.ObjectMeta, request.Spec)
	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(request.Instance, deploymentConfig, request.Client, request.MutationArgs)
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

// CreateDeploymentConfig creates the specified DeploymentConfig using the provided client.
func CreateDeploymentConfig(deploymentConfig *oappsv1.DeploymentConfig, client cntrlClient.Client) error {
	return resource.CreateObject(deploymentConfig, client)
}

// UpdateDeploymentConfig updates the specified DeploymentConfig using the provided client.
func UpdateDeploymentConfig(deploymentConfig *oappsv1.DeploymentConfig, client cntrlClient.Client) error {
	return resource.UpdateObject(deploymentConfig, client)
}

// DeleteDeploymentConfig deletes the DeploymentConfig with the given name and namespace using the provided client.
func DeleteDeploymentConfig(name, namespace string, client cntrlClient.Client) error {
	deploymentConfig := &oappsv1.DeploymentConfig{}
	return resource.DeleteObject(name, namespace, deploymentConfig, client)
}

// GetDeploymentConfig retrieves the DeploymentConfig with the given name and namespace using the provided client.
func GetDeploymentConfig(name, namespace string, client cntrlClient.Client) (*oappsv1.DeploymentConfig, error) {
	deploymentConfig := &oappsv1.DeploymentConfig{}
	obj, err := resource.GetObject(name, namespace, deploymentConfig, client)
	if err != nil {
		return nil, err
	}
	// Assert the object as a oappsv1.DeploymentConfig
	deploymentConfig, ok := obj.(*oappsv1.DeploymentConfig)
	if !ok {
		return nil, errors.New("failed to assert the object as a oappsv1.DeploymentConfig")
	}
	return deploymentConfig, nil
}

// ListDeploymentConfigs returns a list of DeploymentConfig objects in the specified namespace using the provided client and list options.
func ListDeploymentConfigs(namespace string, client cntrlClient.Client, listOptions []cntrlClient.ListOption) (*oappsv1.DeploymentConfigList, error) {
	deploymentConfigList := &oappsv1.DeploymentConfigList{}
	obj, err := resource.ListObjects(namespace, deploymentConfigList, client, listOptions)
	if err != nil {
		return nil, err
	}
	// Assert the object as a oappsv1.DeploymentConfigList
	deploymentConfigList, ok := obj.(*oappsv1.DeploymentConfigList)
	if !ok {
		return nil, errors.New("failed to assert the object as a oappsv1.DeploymentConfigList")
	}
	return deploymentConfigList, nil
}
