package gitserver

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/ssh"

	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

type sshKeyPair struct {
	privateKeyPEM []byte
	publicKey     string
}

func generateSSHKeyPair() sshKeyPair {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	Expect(err).NotTo(HaveOccurred())

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	Expect(err).NotTo(HaveOccurred())

	privateKeyBlock, err := ssh.MarshalPrivateKey(privateKey, "")
	Expect(err).NotTo(HaveOccurred())

	return sshKeyPair{
		privateKeyPEM: pem.EncodeToMemory(privateKeyBlock),
		publicKey:     string(ssh.MarshalAuthorizedKey(sshPublicKey)),
	}
}

func generateTLSSecretData(domain string, podName string, namespace string) map[string][]byte {
	key, err := argoutil.NewPrivateKey()
	Expect(err).NotTo(HaveOccurred())

	caKey, err := argoutil.NewPrivateKey()
	Expect(err).NotTo(HaveOccurred())
	caCert, err := argoutil.NewSelfSignedCACertificate(domain, caKey)
	Expect(err).NotTo(HaveOccurred())

	certSpec := &certmanagerv1.CertificateSpec{
		CommonName: domain,
		Subject: &certmanagerv1.X509Subject{
			Organizations: []string{domain},
		},
	}
	dnsNames := []string{
		podName,
		fmt.Sprintf("%s.%s", podName, namespace),
		fmt.Sprintf("%s.%s.svc", podName, namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", podName, namespace),
	}
	cert, err := argoutil.NewSignedCertificate(certSpec, dnsNames, key, caCert, caKey)
	Expect(err).NotTo(HaveOccurred())

	return map[string][]byte{
		"tls.crt": argoutil.EncodeCertificatePEM(cert),
		"tls.key": argoutil.EncodePrivateKeyPEM(key),
		"ca.crt":  argoutil.EncodeCertificatePEM(caCert),
	}
}
