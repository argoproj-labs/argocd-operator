// Copyright 2026 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gitopspromoter handles reconcilation related to the GitOps Promoter
package gitopspromoter

import (
	"context"
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/ptr"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

const (
	// APIServerAPIServiceGroupPriorityMinimum defines the GroupPriorityMinimum for the API Service
	APIServerAPIServiceGroupPriorityMinimum = 1000
	// APIServerAPIServiceVersionPriority defines the VersionPriority for the API Service
	APIServerAPIServiceVersionPriority = 15
)

// ReconcilePromoterAPIServerAPIService reconciles the API Server's APIService, handles all creation, updates, and deletion
func ReconcilePromoterAPIServerAPIService(client client.Client, compName string, cr *argoproj.ArgoCD) (*apiregistrationv1.APIService, error) {
	apiSvc := buildAPIService(compName, cr)
	enabled := cr.Spec.Promoter == nil || cr.Spec.Promoter.APIServer.IsEnabled()

	allowed := argoutil.IsNamespaceClusterConfigNamespace(cr.Namespace)

	exists := true
	if err := client.Get(context.Background(), types.NamespacedName{Name: apiSvc.Name}, apiSvc); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get existing promoter apiservice %s: %v", apiSvc.Name, err)
		}
		exists = false
	}

	if exists {
		if !cr.Spec.Promoter.IsEnabled() || !enabled || !allowed {
			argoutil.LogResourceDeletion(log, apiSvc, fmt.Sprintf("promoter apiservice for component %s is being deleted due to being disabled", compName))
			if err := client.Delete(context.Background(), apiSvc); err != nil {
				return nil, fmt.Errorf("failed to delete promoter service %s: %v", apiSvc.Name, err)
			}
			return apiSvc, nil
		}

		expectedSpec, err := buildAPIServiceSpec(client, compName, cr)
		if err != nil {
			return nil, err
		}

		if !reflect.DeepEqual(apiSvc.Spec.Service, expectedSpec.Service) ||
			!reflect.DeepEqual(apiSvc.Spec.CABundle, expectedSpec.CABundle) {

			apiSvc.Spec = expectedSpec

			argoutil.LogResourceUpdate(log, apiSvc)
			if err := client.Update(context.Background(), apiSvc); err != nil {
				return nil, err
			}
			return apiSvc, nil
		}

		return apiSvc, nil
	}

	expectedSpec, err := buildAPIServiceSpec(client, compName, cr)
	if err != nil {
		return nil, err
	}

	if !cr.Spec.Promoter.IsEnabled() || !enabled || !allowed {
		return apiSvc, nil
	}

	apiSvc.Spec = expectedSpec
	argoutil.LogResourceCreation(log, apiSvc)
	if err := client.Create(context.Background(), apiSvc); err != nil {
		return nil, fmt.Errorf("failed to create promoter apiserver apiservice %s: %v", apiSvc.Name, err)
	}
	return apiSvc, nil
}

// buildAPIService creates the basic object for the API Service
func buildAPIService(compName string, cr *argoproj.ArgoCD) *apiregistrationv1.APIService {
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "v1alpha1.view.promoter.argoproj.io",
			Labels: buildLabelsForPromoterResources(compName, cr),
		},
	}
}

// buildAPIServiceSpec creates the spec for the API Service object
func buildAPIServiceSpec(client client.Client, compName string, cr *argoproj.ArgoCD) (apiregistrationv1.APIServiceSpec, error) {
	apiSvc := apiregistrationv1.APIServiceSpec{
		Group:                "view.promoter.argoproj.io",
		Version:              "v1alpha1",
		GroupPriorityMinimum: APIServerAPIServiceGroupPriorityMinimum,
		VersionPriority:      APIServerAPIServiceVersionPriority,
		Service: &apiregistrationv1.ServiceReference{
			Name:      generatePromoterResourceName(compName, cr),
			Namespace: cr.Namespace,
			Port:      ptr.To(int32(APIServerPort)),
		},
	}

	if cr.Spec.Promoter != nil && cr.Spec.Promoter.APIServer != nil && cr.Spec.Promoter.APIServer.TLS != nil && cr.Spec.Promoter.APIServer.TLS.CABundleSecretName != "" {
		caSecret := &corev1.Secret{}
		if err := argoutil.FetchObject(client, cr.Namespace, cr.Spec.Promoter.APIServer.TLS.CABundleSecretName, caSecret); err != nil {
			return apiregistrationv1.APIServiceSpec{}, err
		}

		key := "ca.crt"
		if cr.Spec.Promoter != nil && cr.Spec.Promoter.APIServer.TLS != nil && cr.Spec.Promoter.APIServer.TLS.CABundleSecretKey != "" {
			key = cr.Spec.Promoter.APIServer.TLS.CABundleSecretKey
		}

		if val, ok := caSecret.Data[key]; ok {
			apiSvc.CABundle = val
		} else {
			return apiregistrationv1.APIServiceSpec{}, fmt.Errorf("ca bundle not found in secret %s at key %s, API Server may not work correctly", cr.Spec.Promoter.APIServer.TLS.CABundleSecretName, key)
		}
	} else {
		log.Info("Warning: CA Bundle is not set, APIService will not be able to authenticate API Server.")
	}

	return apiSvc, nil
}

// DeleteAPIServices deletes a list of API Services
func DeleteAPIServices(c client.Client, apiSvcList *apiregistrationv1.APIServiceList) error {
	for _, apiSvc := range apiSvcList.Items {
		argoutil.LogResourceDeletion(log, &apiSvc, "cleaning up cluster resources")
		if err := c.Delete(context.TODO(), &apiSvc); err != nil {
			return fmt.Errorf("failed to delete APIService %s during cleanup: %w", apiSvc.Name, err)
		}
	}
	return nil
}
