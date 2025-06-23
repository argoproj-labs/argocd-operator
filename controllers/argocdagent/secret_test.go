// Copyright 2025 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argocdagent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

func TestReconcileRedisSecret_SecretDoesNotExist_ShouldCreateSecret(t *testing.T) {
	// Test case: Redis secret doesn't exist
	// Expected behavior: Should create the secret with expected auth data

	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileRedisSecret(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Secret was created
	secret := &corev1.Secret{}
	expectedSecretName := cr.Name + "-redis"
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      expectedSecretName,
		Namespace: cr.Namespace,
	}, secret)
	assert.NoError(t, err)

	// Verify Secret has expected metadata
	assert.Equal(t, expectedSecretName, secret.Name)
	assert.Equal(t, cr.Namespace, secret.Namespace)

	// Verify Secret has expected labels (NewSecretWithSuffix sets name label to the secret name)
	expectedLabels := argoutil.LabelsForCluster(cr)
	expectedLabels["app.kubernetes.io/name"] = expectedSecretName
	assert.Equal(t, expectedLabels, secret.Labels)

	// Verify Secret has expected data
	assert.NotNil(t, secret.Data)
	assert.Contains(t, secret.Data, "auth")
	assert.Equal(t, []byte("kpQyNY-jche7EBuW"), secret.Data["auth"])

	// Verify Secret type
	assert.Equal(t, corev1.SecretTypeOpaque, secret.Type)
}

func TestReconcileRedisSecret_SecretExists_ShouldDoNothing(t *testing.T) {
	// Test case: Redis secret already exists
	// Expected behavior: Should do nothing (no update, no error)

	cr := makeTestArgoCD()

	// Create existing Secret
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-redis",
			Namespace: cr.Namespace,
			Labels:    argoutil.LabelsForCluster(cr),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"auth": []byte("existing-password"),
		},
	}

	resObjs := []client.Object{cr, existingSecret}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileRedisSecret(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Secret still exists with original data
	secret := &corev1.Secret{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      cr.Name + "-redis",
		Namespace: cr.Namespace,
	}, secret)
	assert.NoError(t, err)

	// Verify the existing data is unchanged
	assert.Equal(t, []byte("existing-password"), secret.Data["auth"])
}

func TestReconcileRedisSecret_SecretNameMatchesExpectedFormat(t *testing.T) {
	// Test case: Verify that the secret name follows the expected format
	// Expected behavior: Secret name should be "{argocd-name}-redis"

	cr := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-argocd",
			Namespace: testNamespace,
		},
	}

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	cl := makeTestReconcilerClient(sch, resObjs)

	err := ReconcileRedisSecret(cl, testCompName, cr, sch)
	assert.NoError(t, err)

	// Verify Secret was created with expected name format
	expectedSecretName := "custom-argocd-redis"
	secret := &corev1.Secret{}
	err = cl.Get(context.TODO(), types.NamespacedName{
		Name:      expectedSecretName,
		Namespace: cr.Namespace,
	}, secret)
	assert.NoError(t, err)
	assert.Equal(t, expectedSecretName, secret.Name)
}
