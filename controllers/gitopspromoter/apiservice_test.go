// Copyright 2026 ArgoCD Operator Developers
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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/ptr"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

func makeExistingAPIService(cr *argoproj.ArgoCD) *apiregistrationv1.APIService {
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "v1alpha1.view.promoter.argoproj.io",
			Labels: buildLabelsForPromoterResources(testCompName, cr),
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Group:                "view.promoter.argoproj.io",
			Version:              "v1alpha1",
			GroupPriorityMinimum: APIServerAPIServiceGroupPriorityMinimum,
			VersionPriority:      APIServerAPIServiceVersionPriority,
			Service: &apiregistrationv1.ServiceReference{
				Name:      generatePromoterResourceName(testCompName, cr),
				Namespace: cr.Namespace,
				Port:      ptr.To(int32(APIServerPort)),
			},
		},
	}
}

func makeCABundleSecret(key string, cr *argoproj.ArgoCD) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testCACertName,
			Namespace: cr.Namespace,
		},
		Data: map[string][]byte{
			key: []byte(testCACertData),
		},
	}
}

func TestReconcilePromoterAPIServerAPIService_DoesNotExist_PromoterDisabled_APIServerNotSet(t *testing.T) {
	// Test case: APIService does not exist and the promoter is fully disabled
	// Expected behavior: Should not create the API Service

	cr := makeTestArgoCD(withPromoterEnabled(false))
	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterAPIServerAPIService_DoesNotExist_PromoterEnabled_APIServerNotSet(t *testing.T) {
	// Test case: APIService does not exist and the promoter is enabled with the API server not set
	// Expected behavior: Should create the API Service, due to API Server being enabled by default

	cr := makeTestArgoCD(withPromoterEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.NoError(t, err)

	assert.Equal(t, "v1alpha1.view.promoter.argoproj.io", retrievedAPIService.Name)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedAPIService.Labels)

	assert.Equal(t, "view.promoter.argoproj.io", retrievedAPIService.Spec.Group)
	assert.Equal(t, "v1alpha1", retrievedAPIService.Spec.Version)
	assert.Equal(t, int32(APIServerAPIServiceGroupPriorityMinimum), retrievedAPIService.Spec.GroupPriorityMinimum)
	assert.Equal(t, int32(APIServerAPIServiceVersionPriority), retrievedAPIService.Spec.VersionPriority)
	assert.Equal(t, generatePromoterResourceName(testCompName, cr), retrievedAPIService.Spec.Service.Name)
	assert.Equal(t, cr.Namespace, retrievedAPIService.Spec.Service.Namespace)
	assert.Equal(t, ptr.To(int32(APIServerPort)), retrievedAPIService.Spec.Service.Port)
}

func TestReconcilePromoterAPIServerAPIService_DoesNotExist_PromoterNotSet_APIServerNotSet(t *testing.T) {
	// Test case: APIService does not exist and the promoter is not set
	// Expected behavior: Should not create the API Service

	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterAPIServerAPIService_Exists_PromoterNotSet_APIServerNotSet(t *testing.T) {
	// Test case: APIService does not exist and the promoter is not set
	// Expected behavior: Should get deleted

	cr := makeTestArgoCD()

	existingAPIService := makeExistingAPIService(cr)

	resObjs := []client.Object{cr, existingAPIService}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterAPIServerAPIService_DoesNotExist_PromoterEnabled_APIServerEnabled(t *testing.T) {
	// Test case: APIService does not exist and both promoter and api server are enabled
	// Expected behavior: Should create the API Service

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.NoError(t, err)

	assert.Equal(t, "v1alpha1.view.promoter.argoproj.io", retrievedAPIService.Name)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedAPIService.Labels)

	assert.Equal(t, "view.promoter.argoproj.io", retrievedAPIService.Spec.Group)
	assert.Equal(t, "v1alpha1", retrievedAPIService.Spec.Version)
	assert.Equal(t, int32(APIServerAPIServiceGroupPriorityMinimum), retrievedAPIService.Spec.GroupPriorityMinimum)
	assert.Equal(t, int32(APIServerAPIServiceVersionPriority), retrievedAPIService.Spec.VersionPriority)
	assert.Equal(t, generatePromoterResourceName(testCompName, cr), retrievedAPIService.Spec.Service.Name)
	assert.Equal(t, cr.Namespace, retrievedAPIService.Spec.Service.Namespace)
	assert.Equal(t, ptr.To(int32(APIServerPort)), retrievedAPIService.Spec.Service.Port)
}

func TestReconcilePromoterAPIServerAPIService_Exists_PromoterEnabled_APIServerDisabled(t *testing.T) {
	// Test case: APIService exists and the promoter is enabled but the API server is disabled
	// Expected behavior: API service should get deleted

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(false))

	existingAPIService := makeExistingAPIService(cr)

	resObjs := []client.Object{cr, existingAPIService}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterAPIServerAPIService_DoesNotExists_PromoterDisabled_APIServerEnabled(t *testing.T) {
	// Test case: APIService does not exist and the promoter is disabled but the API server is enabled
	// Expected behavior: API service should not get created

	cr := makeTestArgoCD(withPromoterEnabled(false), withPromoterAPIServerEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterAPIServerAPIService_Exists_PromoterDisabled_APIServerEnabled(t *testing.T) {
	// Test case: APIService exists and the promoter is disabled but the API server is enabled
	// Expected behavior: API service should get deleted

	cr := makeTestArgoCD(withPromoterEnabled(false), withPromoterAPIServerEnabled(true))

	existingAPIService := makeExistingAPIService(cr)

	resObjs := []client.Object{cr, existingAPIService}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterAPIServerAPIService_CABundle_SecretExists(t *testing.T) {
	// Test case: APIService does not exist and the Promoter is enabled with the API server with TLS settings
	// A secret that contains the CA bundle already exists
	// Expected behavior: TLS CA bundle should be passed to the API Service's .Spec.CABundle field

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(true), withPromoterAPIServerTLS("ca.crt"))

	secret := makeCABundleSecret("ca.crt", cr)

	resObjs := []client.Object{cr, secret}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.NoError(t, err)

	assert.Equal(t, []byte(testCACertData), retrievedAPIService.Spec.CABundle)
}

func TestReconcilePromoterAPIServerAPIService_CABundle_SecretDoesNotExists(t *testing.T) {
	// Test case: APIService does not exist and the Promoter is enabled with the API server with TLS settings
	// The secret with the CA bundle does not exist
	// Expected behavior: The reconcilation function should fail with a not found error

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(true), withPromoterAPIServerTLS("ca.crt"))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	_, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterAPIServerAPIService_CABundle_SecretKeyExists(t *testing.T) {
	// Test case: APIService does not exist and the Promoter is enabled with the API server with TLS settings
	// A secret that contains the CA bundle exists with a custom key that is set
	// Expected behavior: TLS CA bundle should be passed to the .Spec.CABundle field of the API server

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(true), withPromoterAPIServerTLS(testCACertKey))

	secret := makeCABundleSecret(testCACertKey, cr)

	resObjs := []client.Object{cr, secret}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.NoError(t, err)

	assert.Equal(t, []byte(testCACertData), retrievedAPIService.Spec.CABundle)
}

func TestReconcilePromoterAPIServerAPIService_CABundle_SecretKeyDoesNotExist(t *testing.T) {
	// Test case: APIService does not exist and the Promoter is enabled with the API server with TLS settings
	// A secret that contains the CA bundle exists with the wrong key
	// Expected behavior: TLS CA bundle should be empty

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(true), withPromoterAPIServerTLS(testCACertKey))

	secret := makeCABundleSecret("ca.crt", cr)

	resObjs := []client.Object{cr, secret}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	_, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.Error(t, err)
}

func TestReconcilePromoterAPIServerAPIService_No_CABundle(t *testing.T) {
	// Test case: APIService does not exist and the Promoter is enabled with the API server with no TLS settings
	// Expected behavior: API Service still gets reconciled

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.NoError(t, err)
}

func TestReconcilePromoterAPIServerAPIService_Exists_Update(t *testing.T) {
	// Test case: APIService exists and the service it is pointing to has been updated
	// Expected behavior: the API service is updated to have the correct service reference

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(true))

	existingAPIService := makeExistingAPIService(cr)
	existingAPIService.Spec.Service.Name = "not-a-real-service"
	existingAPIService.Spec.Service.Namespace = "not-a-real-namespace"
	existingAPIService.Spec.Service.Port = ptr.To(int32(25565))

	resObjs := []client.Object{cr, existingAPIService}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	apiService, err := ReconcilePromoterAPIServerAPIService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, apiService)

	retrievedAPIService := &apiregistrationv1.APIService{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name: "v1alpha1.view.promoter.argoproj.io",
	}, retrievedAPIService)
	assert.NoError(t, err)

	assert.Equal(t, generatePromoterResourceName(testCompName, cr), retrievedAPIService.Spec.Service.Name)
	assert.Equal(t, cr.Namespace, retrievedAPIService.Spec.Service.Namespace)
	assert.Equal(t, ptr.To(int32(APIServerPort)), retrievedAPIService.Spec.Service.Port)
}
