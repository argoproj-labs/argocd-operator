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

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	promoter "github.com/argoproj-labs/gitops-promoter/api/v1alpha1"
	"github.com/stretchr/testify/assert"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

func makeExistingControllerConfiguration(cr *argoproj.ArgoCD) *promoter.ControllerConfiguration {
	return buildControllerConfiguration(testCompName, cr)
}

func TestReconcilePromoterControllerConfiguration_DoesNotExist_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and ControllerConfiguration does not exist
	// Expected: ControllerConfiguration should not exist

	cr := makeTestArgoCD(withPromoterEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	conConfig, err := ReconcilePromoterControllerConfiguration(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, conConfig)

	retrievedConConfig := &promoter.ControllerConfiguration{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "promoter-controller-configuration",
		Namespace: cr.Namespace,
	}, retrievedConConfig)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterControllerConfiguration_DoesNotExist_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and ControllerConfiguration does not exist
	// Expected: ControllerConfiguration should exist

	cr := makeTestArgoCD(withPromoterEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	conConfig, err := ReconcilePromoterControllerConfiguration(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, conConfig)

	retrievedConConfig := &promoter.ControllerConfiguration{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "promoter-controller-configuration",
		Namespace: cr.Namespace,
	}, retrievedConConfig)
	assert.NoError(t, err)

	assert.Equal(t, "promoter-controller-configuration", retrievedConConfig.Name)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedConConfig.Labels)
}

func TestReconcilePromoterControllerConfiguration_Exists_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and ControllerConfiguration exists
	// Expected: ControllerConfiguration should get deleted

	cr := makeTestArgoCD(withPromoterEnabled(false))

	existingConConfig := makeExistingControllerConfiguration(cr)

	resObjs := []client.Object{cr, existingConConfig}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	conConfig, err := ReconcilePromoterControllerConfiguration(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, conConfig)

	retrievedConConfig := &promoter.ControllerConfiguration{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "promoter-controller-configuration",
		Namespace: cr.Namespace,
	}, retrievedConConfig)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterControllerConfiguration_Exists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and controller configuration exists
	// Expected: ControllerConfiguration should get deleted

	cr := makeTestArgoCD()

	existingConConfig := makeExistingControllerConfiguration(cr)

	resObjs := []client.Object{cr, existingConConfig}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	conConfig, err := ReconcilePromoterControllerConfiguration(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, conConfig)

	retrievedConConfig := &promoter.ControllerConfiguration{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "promoter-controller-configuration",
		Namespace: cr.Namespace,
	}, retrievedConConfig)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterControllerConfiguration_DoesNotExist_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set and controller configuration does not exists
	// Expected: ControllerConfiguration should not get created

	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	conConfig, err := ReconcilePromoterControllerConfiguration(client, testCompName, cr)
	assert.NoError(t, err)
	assert.NotNil(t, conConfig)

	retrievedConConfig := &promoter.ControllerConfiguration{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      "promoter-controller-configuration",
		Namespace: cr.Namespace,
	}, retrievedConConfig)
	assert.True(t, errors.IsNotFound(err))
}
