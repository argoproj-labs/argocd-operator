// Copyright 2020 Argo CD Operator Developers
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

package common

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

	"golang.org/x/crypto/bcrypt"
)

// CertType defines the type of the cert.
type CertType int

// CertConfig configures how to generate the Cert.
type CertConfig struct {
	// CertName is the name of the cert.
	CertName string
	// Optional CertType. Serving, client or both; defaults to both.
	CertType CertType
	// Optional CommonName is the common name of the cert; defaults to "".
	CommonName string
	// Optional Organization is Organization of the cert; defaults to "".
	Organization []string
	// Optional CA Key, if user wants to provide custom CA key via a file path.
	CAKey string
	// Optional CA Certificate, if user wants to provide custom CA cert via file path.
	CACert string
	// TODO: consider to add passed in SAN fields.
}

// PasswordHasher is an interface type to declare a general-purpose password management tool.
type PasswordHasher interface {
	HashPassword(string) (string, error)
	VerifyPassword(string, string) bool
}

// BcryptPasswordHasher handles password hashing with Bcrypt.  Create with `0` as the work factor to default to bcrypt.DefaultCost at hashing time.  The Cost field represents work factor.
type BcryptPasswordHasher struct {
	Cost int
}

const (
	// ClientAndServingCert defines both client and serving cert.
	ClientAndServingCert CertType = iota
	// ServingCert defines a serving cert.
	ServingCert
	// ClientCert defines a client cert.
	ClientCert
)

// PreferredHashers holds the list of preferred hashing algorithms, in order of most to least preferred.  Any password that does not validate with the primary algorithm will be considered "stale."  DO NOT ADD THE DUMMY HASHER FOR USE IN PRODUCTION.
var preferredHashers = []PasswordHasher{
	BcryptPasswordHasher{},
}

// NewPrivateKey returns randomly generated RSA private key.
func NewPrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, ArgoCDDefaultRSAKeySize)
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
func NewSelfSignedCACertificate(key *rsa.PrivateKey) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		NotBefore:             now.UTC(),
		NotAfter:              now.Add(ArgoCDDuration365Days).UTC(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
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
func NewSignedCertificate(cfg *CertConfig, dnsNames []string, key *rsa.PrivateKey, caCert *x509.Certificate, caKey *rsa.PrivateKey) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	eku := []x509.ExtKeyUsage{}
	switch cfg.CertType {
	case ClientCert:
		eku = append(eku, x509.ExtKeyUsageClientAuth)
	case ServingCert:
		eku = append(eku, x509.ExtKeyUsageServerAuth)
	case ClientAndServingCert:
		eku = append(eku, x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth)
	}
	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:     dnsNames,
		SerialNumber: serial,
		NotBefore:    caCert.NotBefore,
		NotAfter:     time.Now().Add(ArgoCDDuration365Days).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  eku,
	}
	certDERBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

// HashPasswordWithHashers hashes an entered password using the first hasher in the provided list of hashers.
func hashPasswordWithHashers(password string, hashers []PasswordHasher) (string, error) {
	// Even though good hashers will disallow blank passwords, let's be explicit that ALL BLANK PASSWORDS ARE INVALID.  Full stop.
	if password == "" {
		return "", fmt.Errorf("blank passwords are not allowed")
	}
	return hashers[0].HashPassword(password)
}

// VerifyPasswordWithHashers verifies an entered password against a hashed password using one or more algorithms.  It returns whether the hash is "stale" (i.e., was verified using something other than the FIRST hasher specified).
func verifyPasswordWithHashers(password, hashedPassword string, hashers []PasswordHasher) (bool, bool) {
	// Even though good hashers will disallow blank passwords, let's be explicit that ALL BLANK PASSWORDS ARE INVALID.  Full stop.
	if password == "" {
		return false, false
	}

	valid, stale := false, false

	for idx, hasher := range hashers {
		if hasher.VerifyPassword(password, hashedPassword) {
			valid = true
			if idx > 0 {
				stale = true
			}
			break
		}
	}

	return valid, stale
}

// HashPassword hashes against the current preferred hasher.
func HashPassword(password string) (string, error) {
	return hashPasswordWithHashers(password, preferredHashers)
}

// VerifyPassword verifies an entered password against a hashed password and returns whether the hash is "stale" (i.e., was verified using the FIRST preferred hasher above).
func VerifyPassword(password, hashedPassword string) (valid, stale bool) {
	valid, stale = verifyPasswordWithHashers(password, hashedPassword, preferredHashers)
	return
}

// HashPassword creates a one-way digest ("hash") of a password.  In the case of Bcrypt, a pseudorandom salt is included automatically by the underlying library.  For security reasons, the work factor is always at _least_ bcrypt.DefaultCost.
func (h BcryptPasswordHasher) HashPassword(password string) (string, error) {
	cost := h.Cost
	if cost < bcrypt.DefaultCost {
		cost = bcrypt.DefaultCost
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		hashedPassword = []byte("")
	}
	return string(hashedPassword), err
}

// VerifyPassword validates whether a one-way digest ("hash") of a password was created from a given plaintext password.
func (h BcryptPasswordHasher) VerifyPassword(password, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}
