package server

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func TestServerReconciler_createAndDeleteServiceAccount(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)

	setTestResourceNameAndLabels(sr)

	// create service account
	err := sr.reconcileServiceAccount()
	assert.NoError(t, err)

	// service account should be created
	currentServiceAccount := &corev1.ServiceAccount{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: argocdcommon.TestNamespace}, currentServiceAccount)
	assert.NoError(t, err)

	// delete service account
	err = sr.deleteServiceAccount(resourceName, sr.Instance.Namespace)
	assert.NoError(t, err)

	// sa should not exist
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: argocdcommon.TestNamespace}, currentServiceAccount)
	assert.Equal(t, true, errors.IsNotFound(err))
}
