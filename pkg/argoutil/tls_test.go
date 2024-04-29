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

package argoutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"math/big"
	"testing"
	"time"
)

func TestNewPrivateKey(t *testing.T) {
	key, err := NewPrivateKey()
	assert.NoError(t, err)
	assert.NotNil(t, key)
}

func TestEncodePrivateKeyPEM(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemKey := EncodePrivateKeyPEM(rsaKey)
	block, _ := pem.Decode(pemKey)
	assert.NotNil(t, block)
	assert.Equal(t, "RSA PRIVATE KEY", block.Type)
}

func TestEncodeCertificatePEM(t *testing.T) {
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "test-certificate.com",
			Organization: []string{"Test Org"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(0, 0, 1),
	}
	pemCert := EncodeCertificatePEM(cert)
	block, _ := pem.Decode(pemCert)
	assert.NotNil(t, block)
	assert.Equal(t, "CERTIFICATE", block.Type)
}

func TestParsePEMEncodedPrivateKey(t *testing.T) {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pemKey := EncodePrivateKeyPEM(rsaKey)
	parsedKey, err := ParsePEMEncodedPrivateKey(pemKey)
	assert.NoError(t, err)
	assert.NotNil(t, parsedKey)
	assert.Equal(t, pemKey, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(parsedKey)}))
}

func TestNewCACertAndKey(t *testing.T) {
	instName := "test-instance"
	cert, key, err := NewCACertAndKey(instName)

	assert.NoError(t, err)
	assert.NotNil(t, cert)
	assert.NotNil(t, key)
	assert.IsType(t, &x509.Certificate{}, cert)
	assert.IsType(t, &rsa.PrivateKey{}, key)
	assert.Equal(t, "argocd-operator@test-instance", cert.Subject.CommonName)
}

func TestNewSelfSignedCACertificate(t *testing.T) {
	name := "test"
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	cert, err := NewSelfSignedCACertificate(name, key)

	assert.NoError(t, err)
	assert.NotNil(t, cert)
	assert.IsType(t, &x509.Certificate{}, cert)
	assert.Equal(t, "argocd-operator@test", cert.Subject.CommonName)
}

func TestNewTLSCertAndKey(t *testing.T) {
	secName := "test-secret"
	instName := "test-instance"
	namespace := "test-namespace"
	c := &x509.Certificate{}
	k, _ := rsa.GenerateKey(rand.Reader, 2048)
	cert, key, err := NewTLSCertAndKey(secName, instName, namespace, c, k)

	assert.NoError(t, err)
	assert.NotNil(t, cert)
	assert.NotNil(t, key)
	assert.IsType(t, &x509.Certificate{}, cert)
	assert.IsType(t, &rsa.PrivateKey{}, key)
	assert.Equal(t, "test-secret", cert.Subject.CommonName)
}

func TestHasArgoTLSChanged(t *testing.T) {
	actual := &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("actual-cert"),
			"tls.key": []byte("actual-key"),
		},
	}
	expected := &corev1.Secret{
		Data: map[string][]byte{
			"tls.crt": []byte("expected-cert"),
			"tls.key": []byte("expected-key"),
		},
	}

	assert.True(t, HasArgoTLSChanged(actual, expected))

	actual.Data["tls.crt"] = []byte("expected-cert")
	actual.Data["tls.key"] = []byte("expected-key")

	assert.False(t, HasArgoTLSChanged(actual, expected))
}
