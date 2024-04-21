package argocd

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/test"
	argopass "github.com/argoproj/argo-cd/v2/util/password"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Test_ReconcileArgoCD_ReconcileShouldNotChangeWhenUpdatedAdminPass(t *testing.T) {

	resources := []client.Object{
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-cluster"
				s.Data = map[string][]byte{
					common.ArgoCDKeyAdminPassword: []byte("something"),
				}
			},
		),
		test.MakeTestSecret(nil,
			func(s *corev1.Secret) {
				s.Name = "test-argocd-tls"
			},
		),
	}

	r := makeTestArgoCDReconciler(
		test.MakeTestArgoCD(nil),
		resources...,
	)

	r.secretVarSetter()

	err := r.reconcileArgoCDSecret()
	assert.NoError(t, err)

	testSecret := &corev1.Secret{}
	secretErr := r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: "test-ns"}, testSecret)
	assert.NoError(t, secretErr)

	// simulating update of argo-cd Admin password from cli or argocd dashboard
	hashedPassword, _ := argopass.HashPassword("updated_password")
	testSecret.Data[common.ArgoCDKeyAdminPassword] = []byte(hashedPassword)
	mTime := nowBytes()
	testSecret.Data[common.ArgoCDKeyAdminPasswordMTime] = mTime
	r.Client.Update(context.TODO(), testSecret)

	_ = r.reconcileArgoCDSecret()
	_ = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: "test-ns"}, testSecret)

	// checking if reconciliation updates the ArgoCDKeyAdminPassword and ArgoCDKeyAdminPasswordMTime
	if string(testSecret.Data[common.ArgoCDKeyAdminPassword]) != hashedPassword {
		t.Errorf("Expected hashedPassword to reamin unchanged but got updated")
	}
	if string(testSecret.Data[common.ArgoCDKeyAdminPasswordMTime]) != string(mTime) {
		t.Errorf("Expected ArgoCDKeyAdminPasswordMTime to reamin unchanged but got updated")
	}

	// if you remove the secret.Data it should come back, including the secretKey
	testSecret.Data = nil
	r.Client.Update(context.TODO(), testSecret)

	_ = r.reconcileArgoCDSecret()
	_ = r.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-secret", Namespace: "test-ns"}, testSecret)

	if testSecret.Data == nil {
		t.Errorf("Expected data for data.server but got nothing")
	}

	if testSecret.Data[common.ArgoCDKeyServerSecretKey] == nil {
		t.Errorf("Expected data for data.server.secretKey but got nothing")
	}
}
