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
	"context"
	"sort"
	"testing"

	"github.com/go-logr/logr"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/pkg/util"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

const (
	testNamespace             = "argocd"
	testArgoCDName            = "argocd"
	testApplicationController = "argocd-application-controller"
)

func ZapLogger(development bool) logr.Logger {
	return zap.New(zap.UseDevMode(development))
}

type namespaceOpt func(*corev1.Namespace)

func makeTestNs(opts ...namespaceOpt) *corev1.Namespace {
	a := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test-ns",
			Labels: make(map[string]string),
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func makeTestReconciler(t *testing.T, objs ...runtime.Object) *ArgoCDReconciler {
	s := scheme.Scheme
	assert.NoError(t, argoproj.AddToScheme(s))
	assert.NoError(t, argoproj.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	logger := ctrl.Log.WithName("test-logger")

	return &ArgoCDReconciler{
		Client: cl,
		Scheme: s,
		Logger: logger,
	}
}

type argoCDOpt func(*argoproj.ArgoCD)

func makeTestArgoCD(opts ...argoCDOpt) *argoproj.ArgoCD {
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

func makeTestArgoCDForKeycloak() *argoproj.ArgoCD {
	a := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
		Spec: argoproj.ArgoCDSpec{
			SSO: &argoproj.ArgoCDSSOSpec{
				Provider: "keycloak",
			},
			Server: argoproj.ArgoCDServerSpec{
				Route: argoproj.ArgoCDRouteSpec{
					Enabled: true,
				},
			},
		},
	}
	return a
}

func makeTestArgoCDWithResources(opts ...argoCDOpt) *argoproj.ArgoCD {
	a := &argoproj.ArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testArgoCDName,
			Namespace: testNamespace,
		},
		Spec: argoproj.ArgoCDSpec{
			ApplicationSet: &argoproj.ArgoCDApplicationSet{
				Resources: makeTestApplicationSetResources(),
			},
			HA: argoproj.ArgoCDHASpec{
				Resources: makeTestHAResources(),
			},
			SSO: &argoproj.ArgoCDSSOSpec{
				Provider: "dex",
				Dex: &argoproj.ArgoCDDexSpec{
					Resources: makeTestDexResources(),
				},
			},
			Controller: argoproj.ArgoCDApplicationControllerSpec{
				Resources: makeTestControllerResources(),
			},
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func makeTestClusterRole() *v1.ClusterRole {
	return &v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testApplicationController,
			Namespace: testNamespace,
		},
		Rules: makeTestPolicyRules(),
	}
}

func makeTestDeployment() *appsv1.Deployment {
	var replicas int32 = 1
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testApplicationController,
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "name",
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Command: []string{"testing"},
							Image:   "test-image",
						},
					},
				},
			},
		},
	}
}

func makeTestPolicyRules() []v1.PolicyRule {
	return []v1.PolicyRule{
		{
			APIGroups: []string{
				"foo.example.com",
			},
			Resources: []string{
				"*",
			},
			Verbs: []string{
				"*",
			},
		},
	}
}

func initialCerts(t *testing.T, host string) argoCDOpt {
	t.Helper()
	return func(a *argoproj.ArgoCD) {
		key, err := util.NewPrivateKey()
		assert.NoError(t, err)
		cert, err := util.NewSelfSignedCACertificate(a.Name, key)
		assert.NoError(t, err)
		encoded := util.EncodeCertificatePEM(cert)

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

func makeTestControllerResources() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("1024Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("1000m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("2048Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("2000m"),
		},
	}
}

func makeTestApplicationSetResources() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("1024Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("1"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("2048Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("2"),
		},
	}
}

func makeTestHAResources() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("250m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("256Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("500m"),
		},
	}
}

func makeTestDexResources() *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("250m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("256Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("500m"),
		},
	}
}

func createNamespace(r *ArgoCDReconciler, n string, managedBy string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}
	if managedBy != "" {
		ns.Labels = map[string]string{common.ArgoCDArgoprojKeyManagedBy: managedBy}
	}

	if r.ResourceManagedNamespaces == nil {
		r.ResourceManagedNamespaces = make(map[string]string)
	}
	r.ResourceManagedNamespaces[ns.Name] = ""

	return r.Client.Create(context.TODO(), ns)
}

func createNamespaceManagedByClusterArgoCDLabel(r *ArgoCDReconciler, n string, managedBy string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}
	if managedBy != "" {
		ns.Labels = map[string]string{common.ArgoCDArgoprojKeyManagedByClusterArgoCD: managedBy}
	}

	if r.AppManagedNamespaces == nil {
		r.AppManagedNamespaces = make(map[string]string)
	}
	r.AppManagedNamespaces[ns.Name] = ""

	return r.Client.Create(context.TODO(), ns)
}

func merge(base map[string]string, diff map[string]string) map[string]string {
	result := make(map[string]string)

	for k, v := range base {
		result[k] = v
	}
	for k, v := range diff {
		result[k] = v
	}

	return result
}
