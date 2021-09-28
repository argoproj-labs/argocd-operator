// Copyright 2021 ArgoCD Operator Developers
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

package argocd

import (
	"context"
	"testing"

	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"gotest.tools/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	argov1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
)

func makeFakeReconciler(t *testing.T, acd *argov1alpha1.ArgoCD, objs ...runtime.Object) *ReconcileArgoCD {
	t.Helper()
	s := scheme.Scheme
	// Register template scheme
	s.AddKnownTypes(templatev1.SchemeGroupVersion, objs...)
	s.AddKnownTypes(oappsv1.SchemeGroupVersion, objs...)
	assert.NilError(t, argov1alpha1.AddToScheme(s))
	templatev1.Install(s)
	oappsv1.Install(s)
	routev1.Install(s)

	cl := fake.NewFakeClientWithScheme(s, objs...)
	return &ReconcileArgoCD{
		Client: cl,
		Scheme: s,
	}
}

func TestReconcile_testKeycloakTemplateInstance(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloak()

	templateAPIFound = true
	r := makeFakeReconciler(t, a)

	assert.NilError(t, r.reconcileSSO(a))

	templateInstance := &templatev1.TemplateInstance{}
	assert.NilError(t, r.Client.Get(
		context.TODO(),
		types.NamespacedName{
			Name:      "rhsso",
			Namespace: a.Namespace,
		},
		templateInstance))
}
func TestReconcile_testKeycloakTemplateWithDexInstance(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloakWithDex()
	r := makeFakeReconciler(t, a)
	assert.Error(t, r.reconcileSSO(a), "multiple SSO configuration")
}

func TestReconcile_noTemplateInstance(t *testing.T) {
	logf.SetLogger(ZapLogger(true))
	a := makeTestArgoCDForKeycloak()
	r := makeFakeReconciler(t, a)

	assert.NilError(t, r.reconcileSSO(a))
}
