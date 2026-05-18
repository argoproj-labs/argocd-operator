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
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
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

// -------------------- Common Helpers --------------------

const (
	defaultTLSMinVersion = tls.VersionTLS13
	defaultTLSMaxVersion = tls.VersionTLS13
)

// -------------------- TLS Version Maps --------------------

var (
	supportedTLSVersions = map[string]uint16{
		"1.1": tls.VersionTLS11,
		"1.2": tls.VersionTLS12,
		"1.3": tls.VersionTLS13,
	}
	tlsVersionNames = map[uint16]string{
		tls.VersionTLS11: "1.1",
		tls.VersionTLS12: "1.2",
		tls.VersionTLS13: "1.3",
	}
	// Precompute once instead of every validation call
	supportedCipherSuites = buildCipherSuiteMap()
)

func buildCipherSuiteMap() map[string]*tls.CipherSuite {
	m := make(map[string]*tls.CipherSuite)
	for _, cs := range tls.CipherSuites() {
		m[cs.Name] = cs
	}
	return m
}

// -------------------- TLS Version Helpers --------------------

func TLSVersionName(version uint16) string {
	if name, ok := tlsVersionNames[version]; ok {
		return name
	}
	return fmt.Sprintf("unknown (0x%04x)", version)
}

func ParseTLSVersion(v string) (uint16, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, nil
	}
	val, ok := supportedTLSVersions[v]
	if !ok {
		return 0, fmt.Errorf("invalid TLS version %q: supported values are 1.1, 1.2, 1.3", v)
	}
	return val, nil
}

// -------------------- TLS Validation --------------------

func ValidateTLSConfig(minVersion, maxVersion uint16, cipherSuites []string) error {
	// Validate version range
	if minVersion != 0 && maxVersion != 0 && minVersion > maxVersion {
		return fmt.Errorf("minimum TLS version (%s) cannot be higher than maximum TLS version (%s)", TLSVersionName(minVersion), TLSVersionName(maxVersion))
	}
	// No cipher validation needed
	if len(cipherSuites) == 0 {
		return nil
	}
	for _, name := range cipherSuites {
		name = strings.TrimSpace(name)
		cs, ok := supportedCipherSuites[name]
		if !ok {
			return fmt.Errorf("unsupported cipher suite: %s", name)
		}
		// TLS 1.3 ciphers don't need compatibility validation
		if minVersion == tls.VersionTLS13 {
			continue
		}

		if !isCipherCompatible(cs, minVersion, maxVersion) {
			return fmt.Errorf("cipher suite %s is not compatible with TLS versions [%s - %s]", name, TLSVersionName(minVersion), TLSVersionName(maxVersion))
		}
	}
	return nil
}

func isCipherCompatible(cs *tls.CipherSuite, minVersion, maxVersion uint16) bool {
	for _, v := range cs.SupportedVersions {
		if (minVersion == 0 || v >= minVersion) && (maxVersion == 0 || v <= maxVersion) {
			return true
		}
	}
	return false
}

func validateAndParseTLS(tlsCfg *argoproj.ArgoCDTlsConfig) (uint16, uint16, error) {
	if tlsCfg == nil {
		return 0, 0, nil
	}
	minVer, err := ParseTLSVersion(tlsCfg.MinVersion)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid min TLS version: %w", err)
	}
	maxVer, err := ParseTLSVersion(tlsCfg.MaxVersion)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid max TLS version: %w", err)
	}
	if err := ValidateTLSConfig(minVer, maxVer, tlsCfg.CipherSuites); err != nil {
		return 0, 0, fmt.Errorf("invalid TLS configuration: %w", err)
	}
	return minVer, maxVer, nil
}

func joinCiphers(cipherSuites []string) string {
	if len(cipherSuites) == 0 {
		return ""
	}
	return strings.Join(cipherSuites, ":")
}

// -------------------- Argo CD TLS Args ------------

func BuildTLSArgs(tlsCfg *argoproj.ArgoCDTlsConfig) ([]string, error) {
	minVer, maxVer, err := ResolveTLSConfig(tlsCfg)
	if err != nil {
		return nil, err
	}
	args := []string{"--tlsminversion", TLSVersionName(minVer), "--tlsmaxversion", TLSVersionName(maxVer)}
	if tlsCfg != nil {
		if ciphers := joinCiphers(tlsCfg.CipherSuites); ciphers != "" {
			args = append(args, "--tlsciphers", ciphers)
		}
	}
	return args, nil
}

// -------------------- Argo CD Agent TLS Args --------------------

func agentTLSVersion(version uint16) string {
	switch version {
	case tls.VersionTLS11:
		return "tls1.1"
	case tls.VersionTLS12:
		return "tls1.2"
	case tls.VersionTLS13:
		return "tls1.3"
	default:
		return ""
	}
}

func BuildArgoCDAgentTLSArgs(tlsCfg *argoproj.ArgoCDTlsConfig, args map[string]string) (map[string]string, error) {
	minVer, maxVer, err := ResolveTLSConfig(tlsCfg)
	if err != nil {
		return nil, err
	}
	args["--tlsminversion"] = agentTLSVersion(minVer)
	args["--tlsmaxversion"] = agentTLSVersion(maxVer)
	if tlsCfg != nil {
		if ciphers := joinCiphers(tlsCfg.CipherSuites); ciphers != "" {
			args["--tlsciphers"] = ciphers
		}
	}
	return args, nil
}

// -------------------- Redis TLS Args --------------------

func redisTLSVersion(version uint16) string {
	switch version {
	case tls.VersionTLS11:
		return "TLSv1.1"
	case tls.VersionTLS12:
		return "TLSv1.2"
	case tls.VersionTLS13:
		return "TLSv1.3"
	default:
		return ""
	}
}

func buildRedisProtocols(minVersion, maxVersion uint16) []string {
	order := []uint16{tls.VersionTLS11, tls.VersionTLS12, tls.VersionTLS13}
	var protocols []string
	started := false
	for _, v := range order {
		if v == minVersion {
			started = true
		}
		if started {
			protocols = append(protocols, redisTLSVersion(v))
		}
		if v == maxVersion {
			break
		}
	}
	return protocols
}

func BuildRedisArgs(tlsCfg *argoproj.ArgoCDTlsConfig) ([]string, error) {
	minVer, maxVer, err := ResolveTLSConfig(tlsCfg)
	if err != nil {
		return nil, err
	}
	protocols := buildRedisProtocols(minVer, maxVer)
	args := []string{"--tls-protocols", strings.Join(protocols, " ")}
	if tlsCfg == nil || len(tlsCfg.CipherSuites) == 0 {
		return args, nil
	}
	ciphers := joinCiphers(tlsCfg.CipherSuites)
	hasTLS12OrBelow := minVer <= tls.VersionTLS12
	hasTLS13 := maxVer >= tls.VersionTLS13
	if hasTLS12OrBelow {
		args = append(args, "--tls-ciphers", ciphers)
	}
	if hasTLS13 {
		args = append(args, "--tls-ciphersuites", ciphers)
	}
	return args, nil
}

func ResolveTLSConfig(tlsCfg *argoproj.ArgoCDTlsConfig) (uint16, uint16, error) {
	minVer, maxVer, err := validateAndParseTLS(tlsCfg)
	if err != nil {
		return 0, 0, err
	}
	if minVer == 0 {
		minVer = defaultTLSMinVersion
	}
	if maxVer == 0 {
		maxVer = defaultTLSMaxVersion
	}
	return minVer, maxVer, nil
}
