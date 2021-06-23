// Copyright 2019 ArgoCD Operator Developers
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
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"time"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	argopass "github.com/argoproj/argo-cd/util/password"
	corev1 "k8s.io/api/core/v1"
)

// NewCASecret creates a new CA secret with the given suffix for the given ArgoCD.
func NewCASecret(cr *argoprojv1a1.ArgoCD) (*corev1.Secret, error) {
	secret := argoutil.NewTLSSecret(cr.ObjectMeta, "ca")

	key, err := argoutil.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cert, err := argoutil.NewSelfSignedCACertificate(key)
	if err != nil {
		return nil, err
	}

	// This puts both ca.crt and tls.crt into the secret.
	secret.Data = map[string][]byte{
		corev1.TLSCertKey:              argoutil.EncodeCertificatePEM(cert),
		corev1.ServiceAccountRootCAKey: argoutil.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey:        argoutil.EncodePrivateKeyPEM(key),
	}

	return secret, nil
}

// NewCertificateSecret creates a new secret using the given name suffix for the given TLS certificate.
func NewCertificateSecret(suffix string, caCert *x509.Certificate, caKey *rsa.PrivateKey, cr *argoprojv1a1.ArgoCD) (*corev1.Secret, error) {
	secret := argoutil.NewTLSSecret(cr.ObjectMeta, suffix)

	key, err := argoutil.NewPrivateKey()
	if err != nil {
		return nil, err
	}

	cfg := &argoutil.CertConfig{
		CertName:     secret.Name,
		CertType:     argoutil.ClientAndServingCert,
		CommonName:   secret.Name,
		Organization: []string{cr.ObjectMeta.Namespace},
	}

	dnsNames := []string{
		cr.ObjectMeta.Name,
		NameWithSuffix("grpc", cr),
		fmt.Sprintf("%s.%s.svc.cluster.local", cr.ObjectMeta.Name, cr.ObjectMeta.Namespace),
	}

	if cr.Spec.Grafana.Enabled {
		dnsNames = append(dnsNames, GetGrafanaHost(cr))
	}
	if cr.Spec.Prometheus.Enabled {
		dnsNames = append(dnsNames, GetPrometheusHost(cr))
	}

	cert, err := argoutil.NewSignedCertificate(cfg, dnsNames, key, caCert, caKey)
	if err != nil {
		return nil, err
	}

	secret.Data = map[string][]byte{
		corev1.TLSCertKey:       argoutil.EncodeCertificatePEM(cert),
		corev1.TLSPrivateKeyKey: argoutil.EncodePrivateKeyPEM(key),
	}

	return secret, nil
}

// HasArgoAdminPasswordChanged will return true if the Argo admin password has changed.
func HasArgoAdminPasswordChanged(actual *corev1.Secret, expected *corev1.Secret) bool {
	actualPwd := string(actual.Data[common.ArgoCDKeyAdminPassword])
	expectedPwd := string(expected.Data[common.ArgoCDKeyAdminPassword])

	validPwd, _ := argopass.VerifyPassword(expectedPwd, actualPwd)
	if !validPwd {
		logr.Info("admin password has changed")
		return true
	}
	return false
}

// NowNano returns a string with the current UTC time as epoch in nanoseconds
func NowNano() string {
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
}

// HasArgoTLSChanged will return true if the Argo TLS certificate or key have changed.
func HasArgoTLSChanged(actual *corev1.Secret, expected *corev1.Secret) bool {
	actualCert := string(actual.Data[common.ArgoCDKeyTLSCert])
	actualKey := string(actual.Data[common.ArgoCDKeyTLSPrivateKey])
	expectedCert := string(expected.Data[common.ArgoCDKeyTLSCert])
	expectedKey := string(expected.Data[common.ArgoCDKeyTLSPrivateKey])

	if actualCert != expectedCert || actualKey != expectedKey {
		logr.Info("tls secret has changed")
		return true
	}
	return false
}

// NowBytes is a shortcut function to return the current date/time in RFC3339 format.
func NowBytes() []byte {
	return []byte(time.Now().UTC().Format(time.RFC3339))
}
