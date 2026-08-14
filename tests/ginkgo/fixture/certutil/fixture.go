package certutil

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	. "github.com/onsi/gomega"
)

type CertificateRequest struct {
	CommonName  string
	DNSNames    []string
	IPAddresses []net.IP
	ExtKeyUsage []x509.ExtKeyUsage
}

func GenerateCertificateAuthority(subject string) (*rsa.PrivateKey, *x509.Certificate, []byte) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).ToNot(HaveOccurred())

	template := x509.Certificate{
		SerialNumber:          randomSerialNumber(),
		Subject:               pkix.Name{CommonName: subject},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	Expect(err).ToNot(HaveOccurred())

	cert, err := x509.ParseCertificate(certDER)
	Expect(err).ToNot(HaveOccurred())

	return privateKey, cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func IssueCertificate(caCert *x509.Certificate, caKey *rsa.PrivateKey, req CertificateRequest) ([]byte, []byte) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).ToNot(HaveOccurred())

	template := x509.Certificate{
		SerialNumber: randomSerialNumber(),
		Subject: pkix.Name{
			CommonName: req.CommonName,
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: req.ExtKeyUsage,
		DNSNames:    req.DNSNames,
		IPAddresses: req.IPAddresses,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, caCert, &key.PublicKey, caKey)
	Expect(err).ToNot(HaveOccurred())

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := EncodePrivateKeyToPEM(key)

	return certPEM, keyPEM
}

func randomSerialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	Expect(err).ToNot(HaveOccurred())
	return serialNumber
}

func EncodePrivateKeyToPEM(key *rsa.PrivateKey) []byte {
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	Expect(err).ToNot(HaveOccurred())

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}
