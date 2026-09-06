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
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	promoter "github.com/argoproj-labs/gitops-promoter/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

const (
	testCompName   = "promoter"
	testArgoCDName = "gitops-promoter-tests"
	testNamespace  = "argo-cd"
	testCertName   = "test-crt"
	testCACertName = "test-ca"
	testCACertKey  = "ca.tls"
	testCACertData = "FAKE_CERT_DATA"
)

type argoCDOpt func(*argoproj.ArgoCD)

func makeTestArgoCD(opts ...argoCDOpt) *argoproj.ArgoCD {
	os.Setenv("ARGOCD_CLUSTER_CONFIG_NAMESPACES", testNamespace)

	a := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
	}

	for _, o := range opts {
		o(a)
	}

	return a
}

func makeTestReconcilerScheme() *runtime.Scheme {
	s := scheme.Scheme
	_ = argoproj.AddToScheme(s)
	_ = promoter.AddToScheme(s)
	_ = apiregistrationv1.AddToScheme(s)
	return s
}

func makeTestReconcilerClient(sch *runtime.Scheme, resObjs []client.Object) client.Client {
	client := fake.NewClientBuilder().WithScheme(sch)
	if len(resObjs) > 0 {
		client = client.WithObjects(resObjs...)
	}
	return client.Build()
}

func withPromoterEnabled(enabled bool) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		if a.Spec.Promoter == nil {
			a.Spec.Promoter = &argoproj.PromoterSpec{}
		}
		a.Spec.Promoter.Enabled = &enabled
	}
}

func withPromoterAPIServerEnabled(enabled bool) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		if a.Spec.Promoter == nil {
			a.Spec.Promoter = &argoproj.PromoterSpec{}
		}
		if a.Spec.Promoter.APIServer == nil {
			a.Spec.Promoter.APIServer = &argoproj.PromoterAPIServerSpec{}
		}
		a.Spec.Promoter.APIServer.Enabled = &enabled
	}
}

func withPromoterAPIServerTLS(caBundleKey string) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		if a.Spec.Promoter == nil {
			a.Spec.Promoter = &argoproj.PromoterSpec{}
		}
		if a.Spec.Promoter.APIServer == nil {
			a.Spec.Promoter.APIServer = &argoproj.PromoterAPIServerSpec{}
		}
		if a.Spec.Promoter.APIServer.TLS == nil {
			a.Spec.Promoter.APIServer.TLS = &argoproj.PromoterAPIServerTLSSpec{}
		}
		a.Spec.Promoter.APIServer.TLS.CertSecretName = testCertName
		a.Spec.Promoter.APIServer.TLS.CABundleSecretName = testCACertName
		a.Spec.Promoter.APIServer.TLS.CABundleSecretKey = caBundleKey
	}
}

func withPromoterControllerWebhook(enabled bool, serviceType string) argoCDOpt {
	return func(a *argoproj.ArgoCD) {
		if a.Spec.Promoter == nil {
			a.Spec.Promoter = &argoproj.PromoterSpec{}
		}
		a.Spec.Promoter.Webhook = &argoproj.PromoterControllerWebhookSpec{
			Enabled:     ptr.To(enabled),
			ServiceType: serviceType,
		}
	}
}

func makeExistingServiceAccount(cr *argoproj.ArgoCD) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatePromoterResourceName(testCompName, cr),
			Namespace: testNamespace,
			Labels:    buildLabelsForPromoterResources(testCompName, cr),
		},
	}
}

func TestReconcilePromoterServiceAccount_DoesNotExist_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and ServiceAccount does not exist
	// Expected: ServiceAccount should not exist

	cr := makeTestArgoCD(withPromoterEnabled(false))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePromoterServiceAccount(client, testCompName, cr, sch, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrievedSA := &corev1.ServiceAccount{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: testNamespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterServiceAccount_DoesNotExist_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and ServiceAccount does not exist
	// Expected: ServiceAccount should be created

	cr := makeTestArgoCD(withPromoterEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePromoterServiceAccount(client, testCompName, cr, sch, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrievedSA := &corev1.ServiceAccount{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: testNamespace,
	}, retrievedSA)
	assert.NoError(t, err)

	assert.Equal(t, generatePromoterResourceName(testCompName, cr), retrievedSA.Name)
	assert.Equal(t, testNamespace, retrievedSA.Namespace)
	assert.Equal(t, buildLabelsForPromoterResources(testCompName, cr), retrievedSA.Labels)

	assert.Len(t, retrievedSA.OwnerReferences, 1)
	assert.Equal(t, cr.Name, retrievedSA.OwnerReferences[0].Name)
	assert.Equal(t, "ArgoCD", retrievedSA.OwnerReferences[0].Kind)
}

func TestReconcilePromoterServiceAccount_Exists_PromoterDisabled(t *testing.T) {
	// Test Case: Promoter is disabled and ServiceAccount exists
	// Expected: ServiceAccount should be deleted

	cr := makeTestArgoCD(withPromoterEnabled(false))

	existingSA := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePromoterServiceAccount(client, testCompName, cr, sch, false, nil)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrievedSA := &corev1.ServiceAccount{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: testNamespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterServiceAccount_DoesNotExist_SetsImagePullSecrets(t *testing.T) {
	// Test Case: Promoter enabled, SA created with the provided imagePullSecrets

	cr := makeTestArgoCD(withPromoterEnabled(true))

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	refs := []corev1.LocalObjectReference{{Name: "my-pull-secret"}}
	_, err := ReconcilePromoterServiceAccount(client, testCompName, cr, sch, true, refs)
	assert.NoError(t, err)

	retrievedSA := &corev1.ServiceAccount{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: testNamespace,
	}, retrievedSA)
	assert.NoError(t, err)
	assert.Equal(t, refs, retrievedSA.ImagePullSecrets)
}

func TestReconcilePromoterServiceAccount_Exists_UpdatesImagePullSecrets(t *testing.T) {
	// Test Case: existing SA has a stale pull secret; reconcile with a new set
	// updates it, and reconcile with nil clears it (label-removal cleanup).

	cr := makeTestArgoCD(withPromoterEnabled(true))

	existingSA := makeExistingServiceAccount(cr)
	existingSA.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "stale"}}

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	refs := []corev1.LocalObjectReference{{Name: "my-pull-secret"}}
	_, err := ReconcilePromoterServiceAccount(client, testCompName, cr, sch, true, refs)
	assert.NoError(t, err)

	retrievedSA := &corev1.ServiceAccount{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: testNamespace,
	}, retrievedSA)
	assert.NoError(t, err)
	assert.Equal(t, refs, retrievedSA.ImagePullSecrets)

	// empty (non-nil) refs (label removed) should clear the imagePullSecrets.
	// nil refs means "don't touch" (used on OpenShift).
	_, err = ReconcilePromoterServiceAccount(client, testCompName, cr, sch, true, []corev1.LocalObjectReference{})
	assert.NoError(t, err)

	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: testNamespace,
	}, retrievedSA)
	assert.NoError(t, err)
	assert.Empty(t, retrievedSA.ImagePullSecrets)
}

func TestReconcilePromoterServiceAccount_Exists_PromoterEnabled(t *testing.T) {
	// Test Case: Promoter is enabled and ServiceAccount exists
	// Expected: ServiceAccount should stay the same

	cr := makeTestArgoCD(withPromoterEnabled(true))

	existingSA := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePromoterServiceAccount(client, testCompName, cr, sch, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrievedSA := &corev1.ServiceAccount{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: testNamespace,
	}, retrievedSA)
	assert.NoError(t, err)
}

func TestReconcilePromoterServiceAccount_DoesNotExist_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set (nil) and ServiceAccount does not exist
	// Expected: ServiceAccount should not be created

	cr := makeTestArgoCD()

	resObjs := []client.Object{cr}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePromoterServiceAccount(client, testCompName, cr, sch, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrievedSA := &corev1.ServiceAccount{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: testNamespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}

func TestReconcilePromoterServiceAccount_Exists_PromoterNotSet(t *testing.T) {
	// Test Case: Promoter is not set (nil) and ServiceAccount exists
	// Expected: ServiceAccount should be deleted

	cr := makeTestArgoCD()

	existingSA := makeExistingServiceAccount(cr)

	resObjs := []client.Object{cr, existingSA}
	sch := makeTestReconcilerScheme()
	client := makeTestReconcilerClient(sch, resObjs)

	sa, err := ReconcilePromoterServiceAccount(client, testCompName, cr, sch, true, nil)
	assert.NoError(t, err)
	assert.NotNil(t, sa)

	retrievedSA := &corev1.ServiceAccount{}
	err = client.Get(context.Background(), types.NamespacedName{
		Name:      generatePromoterResourceName(testCompName, cr),
		Namespace: testNamespace,
	}, retrievedSA)
	assert.True(t, errors.IsNotFound(err))
}
