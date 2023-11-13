package server

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func TestServerReconciler_createUpdateAndDeleteDeployment(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)

	expectedName := "argocd-server"
	expectedLabels := map[string]string{
		"app.kubernetes.io/name":      expectedName,
		"app.kubernetes.io/instance":  argocdcommon.TestArgoCDName,
		"app.kubernetes.io/component": "server",
		"app.kubernetes.io/part-of":   "argocd",
		"app.kubernetes.io/managed-by": "argocd-operator",
	}

	// reconcile deployment
	err := sr.reconcileDeployment()
	assert.NoError(t, err)

	// deployment should be created
	currentDeployment := &appsv1.Deployment{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: argocdcommon.TestNamespace}, currentDeployment)
	assert.NoError(t, err)
	assert.Equal(t, expectedLabels, currentDeployment.Labels)

	// modify deployment image
	sr.Instance.Spec.Image = "test-argocd"
	sr.Instance.Spec.Version = "latest"

	// update deployment
	err = sr.reconcileDeployment()
	assert.NoError(t, err)

	// image should be updated
	currentDeployment = &appsv1.Deployment{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: argocdcommon.TestNamespace}, currentDeployment)
	assert.NoError(t, err)
	assert.Equal(t, "test-argocd:latest", currentDeployment.Spec.Template.Spec.Containers[0].Image)

	// delete deployment
	err = sr.deleteDeployment(expectedName, sr.Instance.Namespace)
	assert.NoError(t, err)

	// deployment shouldn't exist
	currentDeployment = &appsv1.Deployment{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: expectedName, Namespace: argocdcommon.TestNamespace}, currentDeployment)
	assert.Error(t, err)
	assert.Equal(t, true, errors.IsNotFound(err))

}

// TODO: add extensive unit tests to validate deployment cmd flags, args, vols, etc