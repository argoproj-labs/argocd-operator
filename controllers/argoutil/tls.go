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
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/argoproj-labs/argocd-operator/common"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

// NewPrivateKey returns randomly generated RSA private key.
func NewPrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, common.ArgoCDDefaultRSAKeySize)
}

// EncodePrivateKeyPEM encodes the given private key pem and returns bytes (base64).
func EncodePrivateKeyPEM(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// EncodeCertificatePEM encodes the given certificate pem and returns bytes (base64).
func EncodeCertificatePEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})
}

// ParsePEMEncodedCert parses a certificate from the given pemdata
func ParsePEMEncodedCert(pemdata []byte) (*x509.Certificate, error) {
	decoded, _ := pem.Decode(pemdata)
	if decoded == nil {
		return nil, errors.New("no PEM data found")
	}
	return x509.ParseCertificate(decoded.Bytes)
}

// ParsePEMEncodedPrivateKey parses a private key from given pemdata
func ParsePEMEncodedPrivateKey(pemdata []byte) (*rsa.PrivateKey, error) {
	decoded, _ := pem.Decode(pemdata)
	if decoded == nil {
		return nil, errors.New("no PEM data found")
	}
	return x509.ParsePKCS1PrivateKey(decoded.Bytes)
}

// NewSelfSignedCACertificate returns a self-signed CA certificate based on given configuration and private key.
// The certificate has one-year lease.
func NewSelfSignedCACertificate(name string, key *rsa.PrivateKey) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             now.UTC(),
		NotAfter:              now.Add(common.ArgoCDDuration365Days).UTC(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		Subject:               pkix.Name{CommonName: fmt.Sprintf("argocd-operator@%s", name)},
	}
	certDERBytes, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

// NewSignedCertificate signs a certificate using the given private key, CA and returns a signed certificate.
// The certificate could be used for both client and server auth.
// The certificate has one-year lease.
func NewSignedCertificate(cfg *certmanagerv1.CertificateSpec, dnsNames []string, key *rsa.PrivateKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	eku := []x509.ExtKeyUsage{}
	eku = append(eku, x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth)
	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Subject.Organizations,
		},
		DNSNames:     dnsNames,
		SerialNumber: serial,
		NotBefore:    caCert.NotBefore,
		NotAfter:     time.Now().Add(common.ArgoCDDuration365Days).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  eku,
	}
	certDERBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}
