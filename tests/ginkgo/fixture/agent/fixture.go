package argocdagentprincipal

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"sort"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	argov1beta1api "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	k8sFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/k8s"
	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/utils"
)

type PrincipalResources struct {
	PrincipalNamespaceName   string
	ArgoCDAgentPrincipalName string
	ArgoCDName               string
	ServiceAccount           *corev1.ServiceAccount
	Role                     *rbacv1.Role
	RoleBinding              *rbacv1.RoleBinding
	ClusterRole              *rbacv1.ClusterRole
	ClusterRoleBinding       *rbacv1.ClusterRoleBinding
	PrincipalDeployment      *appsv1.Deployment
	PrincipalNetworkPolicy   *networkingv1.NetworkPolicy
	PrincipalRoute           *routev1.Route
	ServicesToDelete         []string
}

type PrincipalSecretsConfig struct {
	PrincipalNamespaceName      string
	PrincipalServiceName        string
	ResourceProxyServiceName    string
	JWTSecretName               string
	PrincipalTLSSecretName      string
	RootCASecretName            string
	ResourceProxyTLSSecretName  string
	AdditionalPrincipalSANs     []string
	AdditionalResourceProxySANs []string
}

type AgentSecretsConfig struct {
	AgentNamespace            *corev1.Namespace
	PrincipalNamespaceName    string
	PrincipalRootCASecretName string
	AgentRootCASecretName     string
	ClientTLSSecretName       string
	ClientCommonName          string
	ClientDNSNames            []string
}

type ClusterRegistrationSecretConfig struct {
	PrincipalNamespaceName    string
	AgentNamespaceName        string
	AgentName                 string
	ResourceProxyServiceName  string
	ResourceProxyPort         int32
	PrincipalRootCASecretName string
	AgentTLSSecretName        string
	Server                    string
}

type AgentSecretNames struct {
	JWTSecretName                  string
	PrincipalTLSSecretName         string
	RootCASecretName               string
	ResourceProxyTLSSecretName     string
	RedisInitialPasswordSecretName string
}

type VerifyExpectedResourcesExistParams struct {
	Namespace                *corev1.Namespace
	ArgoCDAgentPrincipalName string
	ArgoCDName               string
	ServiceAccount           *corev1.ServiceAccount
	Role                     *rbacv1.Role
	RoleBinding              *rbacv1.RoleBinding
	ClusterRole              *rbacv1.ClusterRole
	ClusterRoleBinding       *rbacv1.ClusterRoleBinding
	PrincipalDeployment      *appsv1.Deployment
	PrincipalRoute           *routev1.Route
	PrincipalNetworkPolicy   *networkingv1.NetworkPolicy
	SecretNames              AgentSecretNames
	ServiceNames             []string
	DeploymentNames          []string
	ExpectRoute              *bool
}

func VerifyResourcesDeleted(resources PrincipalResources) {

	By("verifying resources are deleted for principal pod")

	Eventually(resources.ServiceAccount).Should(k8sFixture.NotExistByName())
	Eventually(resources.Role).Should(k8sFixture.NotExistByName())
	Eventually(resources.RoleBinding).Should(k8sFixture.NotExistByName())
	Eventually(resources.ClusterRole).Should(k8sFixture.NotExistByName())
	Eventually(resources.ClusterRoleBinding).Should(k8sFixture.NotExistByName())
	Eventually(resources.PrincipalDeployment).Should(k8sFixture.NotExistByName())
	Eventually(resources.PrincipalNetworkPolicy).Should(k8sFixture.NotExistByName())

	for _, serviceName := range resources.ServicesToDelete {
		if serviceName == "" {
			continue
		}
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: resources.PrincipalNamespaceName,
			},
		}
		Eventually(service).Should(k8sFixture.NotExistByName())
	}

	if fixture.RunningOnOpenShift() {
		Eventually(resources.PrincipalRoute).Should(k8sFixture.NotExistByName())
	}
}

func CreateRequiredSecrets(cfg PrincipalSecretsConfig) {
	k8sClient, _ := utils.GetE2ETestKubeClient()
	ctx := context.Background()

	By("creating required secrets for principal pod")

	jwtKey := generateJWTSigningKey()
	jwtSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.JWTSecretName,
			Namespace: cfg.PrincipalNamespaceName,
		},
		Data: map[string][]byte{
			"jwt.key": jwtKey,
		},
	}
	Expect(k8sClient.Create(ctx, jwtSecret)).To(Succeed())

	caKey, caCert, caCertPEM := generateCertificateAuthority()
	caKeyPEM := encodePrivateKeyToPEM(caKey)

	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.RootCASecretName,
			Namespace: cfg.PrincipalNamespaceName,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": caCertPEM,
			"tls.key": caKeyPEM,
			"ca.crt":  caCertPEM,
		},
	}
	Expect(k8sClient.Create(ctx, caSecret)).To(Succeed())

	principalDNS, principalIPs := aggregateSANs(cfg.PrincipalNamespaceName, cfg.PrincipalServiceName, cfg.AdditionalPrincipalSANs)
	principalCertPEM, principalKeyPEM := issueCertificate(caCert, caKey, certificateRequest{
		CommonName:  cfg.PrincipalServiceName,
		DNSNames:    principalDNS,
		IPAddresses: principalIPs,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	createTLSSecret(ctx, k8sClient, cfg.PrincipalNamespaceName, cfg.PrincipalTLSSecretName, principalCertPEM, principalKeyPEM, caCertPEM)

	resourceProxyDNS, resourceProxyIPs := aggregateSANs(cfg.PrincipalNamespaceName, cfg.ResourceProxyServiceName, cfg.AdditionalResourceProxySANs)
	resourceProxyCertPEM, resourceProxyKeyPEM := issueCertificate(caCert, caKey, certificateRequest{
		CommonName:  cfg.ResourceProxyServiceName,
		DNSNames:    resourceProxyDNS,
		IPAddresses: resourceProxyIPs,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})
	createTLSSecret(ctx, k8sClient, cfg.PrincipalNamespaceName, cfg.ResourceProxyTLSSecretName, resourceProxyCertPEM, resourceProxyKeyPEM, caCertPEM)
}

func CreateRequiredAgentSecrets(cfg AgentSecretsConfig) {
	k8sClient, _ := utils.GetE2ETestKubeClient()
	ctx := context.Background()

	agentRootCASecretName := cfg.AgentRootCASecretName
	if agentRootCASecretName == "" {
		agentRootCASecretName = cfg.PrincipalRootCASecretName
	}

	var principalCASecret corev1.Secret
	Expect(k8sClient.Get(ctx, types.NamespacedName{
		Name:      cfg.PrincipalRootCASecretName,
		Namespace: cfg.PrincipalNamespaceName,
	}, &principalCASecret)).To(Succeed())

	caCertPEM := principalCASecret.Data["tls.crt"]
	Expect(caCertPEM).ToNot(BeEmpty(), "CA certificate must be present in principal namespace secret")
	caKeyPEM := principalCASecret.Data["tls.key"]
	Expect(caKeyPEM).ToNot(BeEmpty(), "CA private key must be present in principal namespace secret")

	caCert := parseCertificate(caCertPEM)
	caKey := parsePrivateKey(caKeyPEM)

	clientDNS, clientIPs := aggregateClientSANs(cfg.ClientDNSNames)
	clientCertPEM, clientKeyPEM := issueCertificate(caCert, caKey, certificateRequest{
		CommonName:  cfg.ClientCommonName,
		DNSNames:    clientDNS,
		IPAddresses: clientIPs,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})

	createTLSSecret(ctx, k8sClient, cfg.AgentNamespace.Name, cfg.ClientTLSSecretName, clientCertPEM, clientKeyPEM, caCertPEM)

	// Propagate CA certificate without private key to the agent namespace
	propagatedCASecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agentRootCASecretName,
			Namespace: cfg.AgentNamespace.Name,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"tls.crt": caCertPEM,
			"ca.crt":  caCertPEM,
		},
	}
	Expect(k8sClient.Create(ctx, propagatedCASecret)).To(Succeed())
}

func CreateClusterRegistrationSecret(cfg ClusterRegistrationSecretConfig) {
	k8sClient, _ := utils.GetE2ETestKubeClient()
	ctx := context.Background()

	server := cfg.Server
	if server == "" {
		port := cfg.ResourceProxyPort
		if port == 0 {
			port = 9090
		}
		host := fmt.Sprintf("%s.%s.svc", cfg.ResourceProxyServiceName, cfg.PrincipalNamespaceName)
		server = fmt.Sprintf("https://%s:%d?agentName=%s", host, port, cfg.AgentName)
	}

	var caSecret corev1.Secret
	Expect(k8sClient.Get(ctx, types.NamespacedName{
		Name:      cfg.PrincipalRootCASecretName,
		Namespace: cfg.PrincipalNamespaceName,
	}, &caSecret)).To(Succeed())

	caData := caSecret.Data["ca.crt"]
	if len(caData) == 0 {
		caData = caSecret.Data["tls.crt"]
	}
	Expect(caData).ToNot(BeEmpty(), "CA certificate missing from principal root secret")

	var agentTLSSecret corev1.Secret
	Expect(k8sClient.Get(ctx, types.NamespacedName{
		Name:      cfg.AgentTLSSecretName,
		Namespace: cfg.AgentNamespaceName,
	}, &agentTLSSecret)).To(Succeed())

	clientCert := agentTLSSecret.Data["tls.crt"]
	Expect(clientCert).ToNot(BeEmpty(), "agent TLS certificate missing")
	clientKey := agentTLSSecret.Data["tls.key"]
	Expect(clientKey).ToNot(BeEmpty(), "agent TLS private key missing")

	configPayload, err := json.Marshal(map[string]any{
		"tlsClientConfig": map[string]any{
			"insecure": false,
			"certData": base64.StdEncoding.EncodeToString(clientCert),
			"keyData":  base64.StdEncoding.EncodeToString(clientKey),
			"caData":   base64.StdEncoding.EncodeToString(caData),
		},
	})
	Expect(err).ToNot(HaveOccurred())

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cluster-%s", cfg.AgentName),
			Namespace: cfg.PrincipalNamespaceName,
			Labels: map[string]string{
				common.ArgoCDSecretTypeLabel:               "cluster",
				"argocd-agent.argoproj-labs.io/agent-name": cfg.AgentName,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"name":   []byte(cfg.AgentName),
			"server": []byte(server),
			"config": configPayload,
		},
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())
}

func VerifyExpectedResourcesExist(params VerifyExpectedResourcesExistParams) {
	shouldExpectRoute := true
	if params.ExpectRoute != nil {
		shouldExpectRoute = *params.ExpectRoute
	}

	By("verifying expected resources exist")

	if params.SecretNames.RedisInitialPasswordSecretName != "" {
		Eventually(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      params.SecretNames.RedisInitialPasswordSecretName,
				Namespace: params.Namespace.Name,
			},
		}, "30s", "2s").Should(k8sFixture.ExistByName())
	}

	Eventually(params.ServiceAccount).Should(k8sFixture.ExistByName())
	Eventually(params.Role).Should(k8sFixture.ExistByName())
	Eventually(params.RoleBinding).Should(k8sFixture.ExistByName())
	Eventually(params.ClusterRole).Should(k8sFixture.ExistByName())
	Eventually(params.ClusterRoleBinding).Should(k8sFixture.ExistByName())
	Eventually(params.PrincipalNetworkPolicy).Should(k8sFixture.ExistByName())

	for _, serviceName := range params.ServiceNames {
		if serviceName == "" {
			continue
		}
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: params.Namespace.Name,
			},
		}
		Eventually(service).Should(k8sFixture.ExistByName(), "Service '%s' should exist in namespace '%s'", serviceName, params.Namespace.Name)

		if serviceName != params.ArgoCDAgentPrincipalName {
			Expect(string(service.Spec.Type)).To(Equal("ClusterIP"), "Service '%s' should have ClusterIP type", serviceName)
		}
	}

	if shouldExpectRoute && fixture.RunningOnOpenShift() {
		Eventually(params.PrincipalRoute).Should(k8sFixture.ExistByName())
	}

	for _, deploymentName := range params.DeploymentNames {
		if deploymentName == "" {
			continue
		}
		depl := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: params.Namespace.Name,
			},
		}
		Eventually(depl).Should(k8sFixture.ExistByName(), "Deployment '%s' should exist in namespace '%s'", deploymentName, params.Namespace.Name)
	}

	Eventually(params.PrincipalDeployment).Should(k8sFixture.ExistByName())
	Eventually(params.PrincipalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/component", string(argov1beta1api.AgentComponentTypePrincipal)))
	Eventually(params.PrincipalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/managed-by", params.ArgoCDName))
	Eventually(params.PrincipalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/name", params.ArgoCDAgentPrincipalName))
	Eventually(params.PrincipalDeployment).Should(k8sFixture.HaveLabelWithValue("app.kubernetes.io/part-of", "argocd-agent"))
}

func VerifyLogs(deploymentName, namespace string, requiredMessages []string) {
	Eventually(func() bool {
		logOutput, err := osFixture.ExecCommandWithOutputParam(false, true, "kubectl", "logs",
			"deployment/"+deploymentName, "-n", namespace, "--tail=200")
		if err != nil {
			GinkgoWriter.Println("Error getting agent logs: ", err)
			return false
		}

		for _, msg := range requiredMessages {
			if !strings.Contains(logOutput, msg) {
				GinkgoWriter.Println("Expected agent log not found:", msg)
				return false
			}
		}
		return true
	}, "120s", "5s").Should(BeTrue(), "Agent should process cluster cache updates")
}

type certificateRequest struct {
	CommonName  string
	DNSNames    []string
	IPAddresses []net.IP
	ExtKeyUsage []x509.ExtKeyUsage
}

func generateCertificateAuthority() (*rsa.PrivateKey, *x509.Certificate, []byte) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).ToNot(HaveOccurred())

	template := x509.Certificate{
		SerialNumber:          randomSerialNumber(),
		Subject:               pkix.Name{CommonName: "argocd-agent-ca"},
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

func issueCertificate(caCert *x509.Certificate, caKey *rsa.PrivateKey, req certificateRequest) ([]byte, []byte) {
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
	keyPEM := encodePrivateKeyToPEM(key)

	return certPEM, keyPEM
}

func createTLSSecret(ctx context.Context, k8sClient client.Client, namespace, secretName string, certPEM, keyPEM, caCertPEM []byte) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": certPEM,
			"tls.key": keyPEM,
			"ca.crt":  caCertPEM,
		},
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())
}

func generateJWTSigningKey() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).ToNot(HaveOccurred())

	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	Expect(err).ToNot(HaveOccurred())

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func encodePrivateKeyToPEM(key *rsa.PrivateKey) []byte {
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	Expect(err).ToNot(HaveOccurred())

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func parseCertificate(certPEM []byte) *x509.Certificate {
	block, _ := pem.Decode(certPEM)
	Expect(block).ToNot(BeNil(), "invalid certificate data")
	cert, err := x509.ParseCertificate(block.Bytes)
	Expect(err).ToNot(HaveOccurred())
	return cert
}

func parsePrivateKey(keyPEM []byte) *rsa.PrivateKey {
	block, _ := pem.Decode(keyPEM)
	Expect(block).ToNot(BeNil(), "invalid private key data")
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	Expect(err).ToNot(HaveOccurred())

	privateKey, ok := parsedKey.(*rsa.PrivateKey)
	Expect(ok).To(BeTrue(), "private key is not RSA")
	return privateKey
}

func randomSerialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	Expect(err).ToNot(HaveOccurred())
	return serialNumber
}

func aggregateSANs(namespace, serviceName string, additional []string) ([]string, []net.IP) {
	defaults := buildDefaultSANs(serviceName, namespace)
	return aggregateSANLists(defaults, additional)
}

func aggregateClientSANs(additional []string) ([]string, []net.IP) {
	return aggregateSANLists(nil, additional)
}

func aggregateSANLists(defaults, additional []string) ([]string, []net.IP) {
	dnsSet := map[string]struct{}{}
	ipSet := map[string]struct{}{}
	var dnsNames []string
	var ipAddresses []net.IP

	addEntry := func(entry string) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return
		}
		if ip := net.ParseIP(entry); ip != nil {
			key := ip.String()
			if _, found := ipSet[key]; !found {
				ipSet[key] = struct{}{}
				ipAddresses = append(ipAddresses, ip)
			}
			return
		}
		if _, found := dnsSet[entry]; !found {
			dnsSet[entry] = struct{}{}
			dnsNames = append(dnsNames, entry)
		}
	}

	for _, entry := range defaults {
		addEntry(entry)
	}
	for _, entry := range additional {
		addEntry(entry)
	}

	sort.Strings(dnsNames)
	sort.Slice(ipAddresses, func(i, j int) bool {
		return bytes.Compare(ipAddresses[i], ipAddresses[j]) < 0
	})

	return dnsNames, ipAddresses
}

func buildDefaultSANs(serviceName, namespace string) []string {
	if serviceName == "" || namespace == "" {
		return nil
	}
	return []string{
		serviceName,
		fmt.Sprintf("%s.%s", serviceName, namespace),
		fmt.Sprintf("%s.%s.svc", serviceName, namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace),
	}
}
