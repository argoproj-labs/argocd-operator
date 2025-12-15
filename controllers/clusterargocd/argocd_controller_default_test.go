package clusterargocd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
)

func TestReconcileClusterArgoCD_DefaultsControlPlaneNamespace(t *testing.T) {
	// Register the scheme
	s := runtime.NewScheme()
	assert.NoError(t, argoproj.AddToScheme(s))

	// Create a ClusterArgoCD object with empty ControlPlaneNamespace
	cr := &argoproj.ClusterArgoCD{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-argocd",
		},
		Spec: argoproj.ClusterArgoCDSpec{
			ControlPlaneNamespace: "",
		},
	}

	// Create a fake client with the object
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(cr).Build()

	// Create the Reconciler
	r := &ReconcileClusterArgoCD{
		Client: cl,
		Scheme: s,
	}

	// Create the Reconcile Request
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: "test-argocd",
		},
	}

	// Call internalReconcile directly as we modified it to handle the defaulting
	// We expect it to update the object and return Requeue: true
	res, _, _, err := r.internalReconcile(context.TODO(), req)
	assert.NoError(t, err)
	assert.True(t, res.Requeue)

	// Fetch the object again to verify the update
	updatedCR := &argoproj.ClusterArgoCD{}
	err = cl.Get(context.TODO(), req.NamespacedName, updatedCR)
	assert.NoError(t, err)
	assert.Equal(t, "test-argocd", updatedCR.Spec.ControlPlaneNamespace)
}
