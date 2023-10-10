package secret

import (
	"github.com/argoproj-labs/argocd-operator/common"
	argopass "github.com/argoproj/argo-cd/v2/util/password"
	corev1 "k8s.io/api/core/v1"
)

// ArgoAdminPasswordChanged will return true if the Argo admin password has changed.
func ArgoAdminPasswordChanged(actual *corev1.Secret, expected *corev1.Secret) bool {
	actualPwd := string(actual.Data[common.ArgoCDKeyAdminPassword])
	expectedPwd := string(expected.Data[common.ArgoCDKeyAdminPassword])

	validPwd, _ := argopass.VerifyPassword(expectedPwd, actualPwd)
	if !validPwd {
		return true
	}
	return false
}

// ArgoTLSChanged will return true if the Argo TLS certificate or key have changed.
func ArgoTLSChanged(actual *corev1.Secret, expected *corev1.Secret) bool {
	actualCert := string(actual.Data[corev1.TLSCertKey])
	actualKey := string(actual.Data[corev1.TLSPrivateKeyKey])
	expectedCert := string(expected.Data[corev1.TLSCertKey])
	expectedKey := string(expected.Data[corev1.TLSPrivateKeyKey])

	if actualCert != expectedCert || actualKey != expectedKey {
		return true
	}
	return false
}
