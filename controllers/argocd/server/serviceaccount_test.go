package server

import (
	"context"
	"fmt"
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

	expectedSAName := fmt.Sprint(argocdcommon.TestArgoCDName + "argocd-server")
	expectedSALabels := map[string]string{
		"app.kubernetes.io/name":      expectedSAName,
		"app.kubernetes.io/instance":  argocdcommon.TestArgoCDName,
		"app.kubernetes.io/part-of":   "argocd",
		"app.kubernetes.io/managed-by": "argocd-operator",
	}

	// create service account
	err := sr.reconcileServiceAccount()
	assert.NoError(t, err)

	// service account should be created
	currentServiceAccount := &corev1.ServiceAccount{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: expectedSAName, Namespace: argocdcommon.TestNamespace}, currentServiceAccount)
	assert.NoError(t, err)
	assert.Equal(t, expectedSALabels, currentServiceAccount.Labels)

	// delete service account
	err = sr.deleteServiceAccount(expectedSAName, sr.Instance.Namespace)
	assert.NoError(t, err)

	// sa should not exist
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: expectedSAName, Namespace: argocdcommon.TestNamespace}, currentServiceAccount)
	assert.Equal(t, true, errors.IsNotFound(err))
}
