// Copyright 2020 ArgoCD Operator Developers
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
	"sort"
	"testing"

	"github.com/argoproj-labs/argocd-operator/pkg/apis"
	"github.com/argoproj-labs/argocd-operator/pkg/controller/argoutil"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/pkg/apis/argoproj/v1alpha1"
)

const (
	testNamespace  = "argocd"
	testArgoCDName = "argocd"
)

func makeTestReconciler(t *testing.T, objs ...runtime.Object) *ReconcileArgoCD {
	s := scheme.Scheme
	assert.NilError(t, apis.AddToScheme(s))

	cl := fake.NewFakeClientWithScheme(s, objs...)
	return &ReconcileArgoCD{
		client: cl,
		scheme: s,
	}
}

type argoCDOpt func(*argoprojv1alpha1.ArgoCD)

func makeTestArgoCD(opts ...argoCDOpt) *argoprojv1alpha1.ArgoCD {
	a := &argoprojv1alpha1.ArgoCD{
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

func initialCerts(t *testing.T, host string) argoCDOpt {
	t.Helper()
	return func(a *argoprojv1alpha1.ArgoCD) {
		key, err := argoutil.NewPrivateKey()
		assert.NilError(t, err)
		cert, err := argoutil.NewSelfSignedCACertificate(key)
		assert.NilError(t, err)
		encoded := argoutil.EncodeCertificatePEM(cert)

		a.Spec.TLS.InitialCerts = map[string]string{
			host: string(encoded),
		}
	}
}

func stringMapKeys(m map[string]string) []string {
	r := []string{}
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
