package workloads

import (
	"context"
	"fmt"

	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
)

// DeploymentRequest objects contain all the required information to produce a deployment object in return
type DeploymentRequest struct {
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

// newDeployment returns a new Deployment instance for the given ArgoCD.
func newDeployment(name, instanceName, instanceNamespace, component string, labels, annotations map[string]string) *appsv1.Deployment {
	var deploymentName string
	if name != "" {
		deploymentName = name
	} else {
		deploymentName = argoutil.GenerateResourceName(instanceName, component)

	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        deploymentName,
			Namespace:   instanceNamespace,
			Labels:      argoutil.MergeMaps(argoutil.LabelsForCluster(instanceName, component), labels),
			Annotations: argoutil.MergeMaps(argoutil.AnnotationsForCluster(instanceName, instanceNamespace), annotations),
		},
	}
}

func CreateDeployment(deployment *appsv1.Deployment, client ctrlClient.Client) error {
	return client.Create(context.TODO(), deployment)
}

// UpdateDeployment updates the specified Deployment using the provided client.
func UpdateDeployment(deployment *appsv1.Deployment, client ctrlClient.Client) error {
	_, err := GetDeployment(deployment.Name, deployment.Namespace, client)
	if err != nil {
		return err
	}

	if err = client.Update(context.TODO(), deployment); err != nil {
		return err
	}
	return nil
}

func DeleteDeployment(name, namespace string, client ctrlClient.Client) error {
	existingDeployment, err := GetDeployment(name, namespace, client)
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if err := client.Delete(context.TODO(), existingDeployment); err != nil {
		return err
	}
	return nil
}

func GetDeployment(name, namespace string, client ctrlClient.Client) (*appsv1.Deployment, error) {
	existingDeployment := &appsv1.Deployment{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: name, Namespace: namespace}, existingDeployment)
	if err != nil {
		return nil, err
	}
	return existingDeployment, nil
}

func ListDeployments(namespace string, client ctrlClient.Client, listOptions []ctrlClient.ListOption) (*appsv1.DeploymentList, error) {
	existingDeployments := &appsv1.DeploymentList{}
	err := client.List(context.TODO(), existingDeployments, listOptions...)
	if err != nil {
		return nil, err
	}
	return existingDeployments, nil
}

func RequestDeployment(request DeploymentRequest) (*appsv1.Deployment, error) {
	var (
		mutationErr error
	)
	deployment := newDeployment(request.Name, request.InstanceName, request.InstanceNamespace, request.Component, request.Labels, request.Annotations)

	if len(request.Mutations) > 0 {
		for _, mutation := range request.Mutations {
			err := mutation(nil, deployment, request.Client)
			if err != nil {
				mutationErr = err
			}
		}
		if mutationErr != nil {
			return deployment, fmt.Errorf("RequestDeployment: one or more mutation functions could not be applied: %s", mutationErr)
		}
	}

	return deployment, nil
}
