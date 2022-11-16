package argorollouts

import (
	"context"
	"testing"

	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace          = "rollouts"
	testRolloutsName       = "example-rollouts"
	testRolloutsController = "example-rollouts-argo-rollouts"
)

type argoCDOpt func(*argoprojv1alpha1.ArgoRollouts)

func makeTestArgoRollouts(opts ...argoCDOpt) *argoprojv1alpha1.ArgoRollouts {
	a := &argoprojv1alpha1.ArgoRollouts{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testRolloutsName,
			Namespace: testNamespace,
		},
	}
	for _, o := range opts {
		o(a)
	}
	return a
}

func makeTestReconciler(t *testing.T, objs ...runtime.Object) *ArgoRolloutsReconciler {
	s := scheme.Scheme
	assert.NoError(t, argoprojv1alpha1.AddToScheme(s))

	cl := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
	return &ArgoRolloutsReconciler{
		Client: cl,
		Scheme: s,
	}
}

func createNamespace(r *ArgoRolloutsReconciler, n string) error {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: n}}
	return r.Client.Create(context.TODO(), ns)
}
