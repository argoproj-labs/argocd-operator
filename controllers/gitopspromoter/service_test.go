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

package gitopspromoter

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/stretchr/testify/assert"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
)

func makeTestServiceSpec() corev1.ServiceSpec {
	return corev1.ServiceSpec{
		Selector: map[string]string{
			common.ArgoCDKeyComponent: testCompName,
		},
		Ports: []corev1.ServicePort{
			{
				Port:       80,
				TargetPort: intstr.FromInt(80),
				Protocol:   corev1.ProtocolTCP,
			},
		},
	}
}

func makeExistingService(cr *argoproj.ArgoCD) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatePromoterResourceName(testCompName, cr),
			Namespace: cr.Namespace,
			Labels:    buildLabelsForPromoterResources(testCompName, cr),
		},
		Spec: makeTestServiceSpec(),
	}
}

func TestReconcilePromoterService_DoesNotExist_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and Service does not exist
	// Expected: Service should not exist

	cr := makeTestArgoCD(withPromoterEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterService(client, testCompName, cr, makeTestServiceSpec(), true)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterService_DoesNotExist_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and Service does not exist
	// Expected: Service should exist

	cr := makeTestArgoCD(withPromoterEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterService(client, testCompName, cr, makeTestServiceSpec(), true)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.NoError(t, err)

	assert.Equal(t, generatePromoterResourceName(testCompName, cr), retrievedService.Name)
	assert.Equal(t, cr.Namespace, retrievedService.Namespace)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedService.Labels)

	expectedSpec := makeTestServiceSpec()
	assert.Equal(t, expectedSpec.Selector, retrievedService.Spec.Selector)
	assert.Equal(t, expectedSpec.Ports[0].Port, retrievedService.Spec.Ports[0].Port)
	assert.Equal(t, expectedSpec.Ports[0].TargetPort, retrievedService.Spec.Ports[0].TargetPort)
	assert.Equal(t, expectedSpec.Ports[0].Protocol, retrievedService.Spec.Ports[0].Protocol)
}

func TestReconcilePromoterService_Exists_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and Service exists
	// Expected: Service should be deleted

	cr := makeTestArgoCD(withPromoterEnabled(false))

	existingSvc := makeExistingService(cr)

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterService(client, testCompName, cr, makeTestServiceSpec(), true)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterService_Exists_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and Service exists
	// Expected: Service should exist and not be changed

	cr := makeTestArgoCD(withPromoterEnabled(true))

	existingService := makeExistingService(cr)

	resObjs := []client.Object{cr, existingService}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterService(client, testCompName, cr, makeTestServiceSpec(), true)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.NoError(t, err)

	assert.Equal(t, generatePromoterResourceName(testCompName, cr), retrievedService.Name)
	assert.Equal(t, cr.Namespace, retrievedService.Namespace)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedService.Labels)

	expectedSpec := makeTestServiceSpec()
	assert.Equal(t, expectedSpec.Selector, retrievedService.Spec.Selector)
	assert.Equal(t, expectedSpec.Ports[0].Port, retrievedService.Spec.Ports[0].Port)
	assert.Equal(t, expectedSpec.Ports[0].TargetPort, retrievedService.Spec.Ports[0].TargetPort)
	assert.Equal(t, expectedSpec.Ports[0].Protocol, retrievedService.Spec.Ports[0].Protocol)
}

func TestReconcilePromoterService_Exists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and Service exists
	// Expected: Service should be deleted

	cr := makeTestArgoCD()

	existingSvc := makeExistingService(cr)

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterService(client, testCompName, cr, makeTestServiceSpec(), true)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterService_DoesNotExist_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and Service does not exist
	// Expected: Service should be created

	cr := makeTestArgoCD()

	existingSvc := makeExistingService(cr)

	resObjs := []client.Object{cr, existingSvc}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterService(client, testCompName, cr, makeTestServiceSpec(), true)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterService_Exists_Update(t *testing.T) {
	// Test Case: Promoter is enabled and service exists but has been changed
	// Expected: Service should be updated to match what is expected

	cr := makeTestArgoCD(withPromoterEnabled(true))

	existingService := makeExistingService(cr)
	existingService.Spec.Selector[common.ArgoCDKeyComponent] = "random-not-correct-value"
	existingService.Spec.Ports[0].Port = 25565
	existingService.Spec.Ports[0].TargetPort = intstr.FromInt(25565)
	existingService.Spec.Ports[0].Protocol = corev1.ProtocolUDP

	resObjs := []client.Object{cr, existingService}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterService(client, testCompName, cr, makeTestServiceSpec(), true)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.NoError(t, err)

	assert.Equal(t, generatePromoterResourceName(testCompName, cr), retrievedService.Name)
	assert.Equal(t, cr.Namespace, retrievedService.Namespace)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedService.Labels)

	expectedSpec := makeTestServiceSpec()
	assert.Equal(t, expectedSpec.Selector, retrievedService.Spec.Selector)
	assert.Equal(t, expectedSpec.Ports[0].Port, retrievedService.Spec.Ports[0].Port)
	assert.Equal(t, expectedSpec.Ports[0].TargetPort, retrievedService.Spec.Ports[0].TargetPort)
	assert.Equal(t, expectedSpec.Ports[0].Protocol, retrievedService.Spec.Ports[0].Protocol)
}

func TestReconcilePromoterControllerWebhookService_PromoterEnabled_WebhookDisabled(t *testing.T) {
	// Test case: Promoter is enabled and webhook is disabled
	// Expected behavior: Webhook Service should not be reconciled

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterControllerWebhook(false, ""))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterControllerWebhookService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterControllerWebhookService_PromoterEnabled_WebhookEnabled(t *testing.T) {
	// Test case: Promoter is enabled and webhook is enabled
	// Expected behavior: Webhook Service should be reconciled

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterControllerWebhook(true, ""))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterControllerWebhookService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.NoError(t, err)

	assert.Equal(t, generatePromoterResourceName(testCompName, cr), retrievedService.Name)
	assert.Equal(t, cr.Namespace, retrievedService.Namespace)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedService.Labels)

	expectedSpec := makeTestServiceSpec()
	assert.Equal(t, expectedSpec.Selector, retrievedService.Spec.Selector)
	assert.Equal(t, int32(ControllerWebhookPort), retrievedService.Spec.Ports[0].Port)
	assert.Equal(t, intstr.FromInt(ControllerWebhookPort), retrievedService.Spec.Ports[0].TargetPort)
	assert.Equal(t, ControllerWebhookProtocol, retrievedService.Spec.Ports[0].Protocol)
	assert.Equal(t, corev1.ServiceTypeClusterIP, retrievedService.Spec.Type)
}

func TestReconcilePromoterControllerWebhookService_PromoterEnabled_WebhookEnabledWithType(t *testing.T) {
	// Test case: Promoter is enabled and webhook is enabled with none default type
	// Expected behavior: Webhook Service should be reconciled with the non default type selected

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterControllerWebhook(true, "NodePort"))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterControllerWebhookService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.NoError(t, err)

	assert.Equal(t, generatePromoterResourceName(testCompName, cr), retrievedService.Name)
	assert.Equal(t, cr.Namespace, retrievedService.Namespace)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedService.Labels)

	expectedSpec := makeTestServiceSpec()
	assert.Equal(t, expectedSpec.Selector, retrievedService.Spec.Selector)
	assert.Equal(t, int32(ControllerWebhookPort), retrievedService.Spec.Ports[0].Port)
	assert.Equal(t, intstr.FromInt(ControllerWebhookPort), retrievedService.Spec.Ports[0].TargetPort)
	assert.Equal(t, ControllerWebhookProtocol, retrievedService.Spec.Ports[0].Protocol)
	assert.Equal(t, corev1.ServiceTypeNodePort, retrievedService.Spec.Type)
}

func TestReconcilePromoterAPIServerService_PromoterEnabled_APIServerNotSet(t *testing.T) {
	// Test case: Promoter is enabled and API server is not set
	// Expected behavior: API Server Service should be reconciled due to being enabled by default

	cr := makeTestArgoCD(withPromoterEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterAPIServerService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.NoError(t, err)

	assert.Equal(t, generatePromoterResourceName(testCompName, cr), retrievedService.Name)
	assert.Equal(t, cr.Namespace, retrievedService.Namespace)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedService.Labels)

	expectedSpec := makeTestServiceSpec()
	assert.Equal(t, expectedSpec.Selector, retrievedService.Spec.Selector)
	assert.Equal(t, int32(APIServerPort), retrievedService.Spec.Ports[0].Port)
	assert.Equal(t, intstr.FromString(APIServerTargetPort), retrievedService.Spec.Ports[0].TargetPort)
	assert.Equal(t, APIServerProtocol, retrievedService.Spec.Ports[0].Protocol)
	assert.Equal(t, APIServerPortName, retrievedService.Spec.Ports[0].Name)
}

func TestReconcilePromoterAPIServerService_PromoterEnabled_APIServerDisabled(t *testing.T) {
	// Test case: Promoter is enabled and API server is disabled
	// Expected behavior: API Server Service should not exist

	cr := makeTestArgoCD(withPromoterEnabled(true), withPromoterAPIServerEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	service, err := ReconcilePromoterAPIServerService(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, service)

	retrievedService := &corev1.Service{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: cr.Namespace,
	}, retrievedService)
	assert.True(t, errors.IsNotFound(err))
}
