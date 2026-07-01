package gitserver

import (
	"context"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/argoproj-labs/argocd-operator/common"
	argocdutil "github.com/argoproj-labs/argocd-operator/controllers/argocd"
	"github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture"
	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"
	podFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/pod"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	serverName = "e2e-gitserver"

	httpPort       = int32(3000)
	sshPort        = int32(2222) // rootless image listens on 2222
	sshServicePort = int32(22)   // service maps 22 -> 2222 for callers
	gitUsername    = "gituser"
	giteaSSHLogin  = "git" // rootless builtin SSH authenticates as RUN_USER (git), not the Gitea account name
)

// Server exposes connection details for a Gitea instance started in the test namespace.
type Server struct {
	namespace   string
	serviceName string

	domain        string
	httpURL       string
	httpUsername  string
	httpPassword  string
	sshPrivateKey []byte
	sshPublicKey  string
	sshKnownHosts string
	sshKeyFile    string
	caCert        []byte
}

func (s *Server) ensureSSHKeyFile() (string, error) {
	if s.sshKeyFile != "" {
		return s.sshKeyFile, nil
	}

	f, err := os.CreateTemp("", "argocd-operator-gitserver-ssh-key-")
	if err != nil {
		return "", err
	}

	if _, err := f.Write(s.getSSHPrivateKey()); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}

	s.sshKeyFile = f.Name()
	return s.sshKeyFile, nil
}

func (s *Server) removeSSHKeyFile() {
	if s.sshKeyFile == "" {
		return
	}
	_ = os.Remove(s.sshKeyFile)
	s.sshKeyFile = ""
}

func (s *Server) getSSHPrivateKey() []byte {
	return append([]byte(nil), s.sshPrivateKey...)
}

// StartServer deploys a functional Git instance with HTTPS and SSH enabled in the given namespace.
func StartServer(ctx context.Context, k8sClient client.Client, ns *corev1.Namespace) (server *Server, cleanup func()) {
	Expect(ns).ToNot(BeNil())

	By("Deploying Git server")
	clusterDomain := fmt.Sprintf("%s.%s.svc.cluster.local", serverName, ns.Name)

	tls := generateTLSSecretData(clusterDomain, serverName, ns.Name)
	httpPassword := argocdutil.GenerateRandomString(24)
	internalToken := argocdutil.GenerateRandomString(24)
	sshKeys := generateSSHKeyPair()

	server = &Server{
		namespace:     ns.Name,
		serviceName:   serverName,
		httpUsername:  gitUsername,
		httpPassword:  httpPassword,
		sshPrivateKey: sshKeys.privateKeyPEM,
		sshPublicKey:  strings.TrimSpace(sshKeys.publicKey),
		caCert:        tls["ca.crt"],
	}

	tlsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName + "-tls",
			Namespace: ns.Name,
			Labels:    fixture.NamespaceLabels,
		},
		Type: corev1.SecretTypeTLS,
		Data: tls,
	}
	Expect(k8sClient.Create(ctx, tlsSecret)).To(Succeed())

	httpCredentialsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName + "-http-credentials",
			Namespace: ns.Name,
			Labels:    fixture.NamespaceLabels,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"username": gitUsername,
			"password": httpPassword,
		},
	}
	Expect(k8sClient.Create(ctx, httpCredentialsSecret)).To(Succeed())

	sshCredentialsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName + "-ssh-credentials",
			Namespace: ns.Name,
			Labels:    fixture.NamespaceLabels,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ssh-privatekey": sshKeys.privateKeyPEM,
			"ssh-publickey":  []byte(strings.TrimSpace(sshKeys.publicKey)),
		},
	}
	Expect(k8sClient.Create(ctx, sshCredentialsSecret)).To(Succeed())

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName,
			Namespace: ns.Name,
			Labels: map[string]string{
				fixture.E2ETestLabelsKey:       fixture.E2ETestLabelsValue,
				"app.kubernetes.io/name":       serverName,
				"app.kubernetes.io/component":  "git-server",
				"app.kubernetes.io/instance":   serverName,
				"app.kubernetes.io/managed-by": "argocd-operator-e2e",
			},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				FSGroup: ptr.To(int64(1000)),
			},
			Containers: []corev1.Container{
				{
					Name:            serverName,
					Image:           giteaImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Env:             giteaEnvVars(clusterDomain, internalToken),
					Ports: []corev1.ContainerPort{
						{Name: "http", ContainerPort: httpPort, Protocol: corev1.ProtocolTCP},
						{Name: "ssh", ContainerPort: sshPort, Protocol: corev1.ProtocolTCP},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "data", MountPath: "/data"},
						{Name: "tls", MountPath: "/etc/gitea/certs", ReadOnly: true},
						{Name: "http-credentials", MountPath: "/etc/gitea/credentials/http", ReadOnly: true},
						{Name: "ssh-credentials", MountPath: "/etc/gitea/credentials/ssh", ReadOnly: true},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstr.FromInt32(httpPort),
							},
						},
						InitialDelaySeconds: 10,
						PeriodSeconds:       5,
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  ptr.To(int64(1000)),
						RunAsGroup: ptr.To(int64(1000)),
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "tls",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: tlsSecret.Name},
					},
				},
				{
					Name: "http-credentials",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: httpCredentialsSecret.Name},
					},
				},
				{
					Name: "ssh-credentials",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: sshCredentialsSecret.Name},
					},
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName,
			Namespace: ns.Name,
			Labels:    fixture.NamespaceLabels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				"app.kubernetes.io/name": serverName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       httpPort,
					TargetPort: intstr.FromString("http"),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "ssh",
					Port:       sshServicePort,
					TargetPort: intstr.FromInt32(sshPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, service)).To(Succeed())

	Eventually(pod, "2m", "10s").Should(podFixture.HavePhase(corev1.PodRunning))

	By("exposing Git server outside the cluster")
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(service), service)).To(Succeed())
		g.Expect(service.Status.LoadBalancer.Ingress).NotTo(BeEmpty())
	}, "5m", "5s").Should(Succeed())

	ingress := service.Status.LoadBalancer.Ingress[0]
	if ingress.Hostname != "" {
		server.domain = ingress.Hostname
	} else {
		server.domain = ingress.IP
	}
	server.httpURL = fmt.Sprintf("https://%s:%d", server.domain, httpPort)
	GinkgoWriter.Printf("Git server public endpoint: %s\n", server.httpURL)

	configureGiteaAdmin(server)

	By("waiting for Git server SSH endpoint")
	server.sshKnownHosts = discoverSSHKnownHosts(server.domain)

	cleanup = func() {
		server.removeSSHKeyFile()

		for _, obj := range []client.Object{service, pod, sshCredentialsSecret, httpCredentialsSecret, tlsSecret} {
			err := k8sClient.Delete(ctx, obj)
			if err != nil && !apierrors.IsNotFound(err) {
				GinkgoWriter.Println("gitserver cleanup:", client.ObjectKeyFromObject(obj), err)
			}
		}
	}

	By("registering repository credentials for Argo CD to use")
	Expect(k8sClient.Create(ctx, server.sshRepoPullCredentialsSecret(ns.Name))).To(Succeed())
	Expect(k8sClient.Create(ctx, server.sshRepoPushCredentialsSecret(ns.Name))).To(Succeed())

	return server, cleanup
}

func (s *Server) sshRepoURLPrefix() string {
	return fmt.Sprintf("ssh://%s@%s:%d/%s/", giteaSSHLogin, s.domain, sshServicePort, s.httpUsername)
}

// SSHKnownHosts returns ssh-keyscan output collected while starting the server.
func (s *Server) SSHKnownHosts() string {
	Expect(s.sshKnownHosts).NotTo(BeEmpty())
	return s.sshKnownHosts
}

func discoverSSHKnownHosts(domain string) string {
	var knownHosts string
	Eventually(func(g Gomega) {
		out, err := osFixture.ExecCommandWithOutputParam(false, false,
			"ssh-keyscan",
			"-p", fmt.Sprintf("%d", sshServicePort),
			"-T", "5",
			domain,
		)
		g.Expect(err).NotTo(HaveOccurred(), out)
		parsed := parseSSHKnownHosts(out)
		g.Expect(parsed).NotTo(BeEmpty(), "ssh-keyscan returned no host keys for %s: %q", domain, out)
		knownHosts = parsed
	}, "2m", "5s").Should(Succeed())
	return knownHosts
}

func parseSSHKnownHosts(output string) string {
	var hosts []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(strings.Fields(line)) >= 3 {
			hosts = append(hosts, line)
		}
	}
	if len(hosts) == 0 {
		return ""
	}
	return strings.Join(hosts, "\n") + "\n"
}

func (s *Server) sshRepoPullCredentialsSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName + "-argocd-ssh-repo-creds",
			Namespace: namespace,
			Labels: map[string]string{
				common.ArgoCDSecretTypeLabel: "repo-creds",
			},
		},
		StringData: map[string]string{
			"type":          "git",
			"url":           s.sshRepoURLPrefix(),
			"sshPrivateKey": string(s.getSSHPrivateKey()),
			"insecure":      "true",
		},
	}
}

func (s *Server) sshRepoPushCredentialsSecret(namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serverName + "-argocd-ssh-repo-write-creds",
			Namespace: namespace,
			Labels: map[string]string{
				common.ArgoCDSecretTypeLabel: "repo-write-creds",
			},
		},
		StringData: map[string]string{
			"type":          "git",
			"url":           s.sshRepoURLPrefix(),
			"sshPrivateKey": string(s.getSSHPrivateKey()),
			"insecure":      "true",
		},
	}
}

func (s *Server) CreateRepo(repoName string) Repo {
	if _, err := giteaAPIPost(s.namespace, s.httpPassword,
		fmt.Sprintf("/api/v1/admin/users/%s/repos", gitUsername),
		fmt.Sprintf(`{"name":"%s","private":false}`, repoName),
	); err != nil {
		GinkgoWriter.Println("gitea repo create returned error (may already exist):", err)
	}

	Eventually(func() error {
		output, err := giteaAPIGet(s.namespace, s.httpPassword,
			fmt.Sprintf("/api/v1/repos/%s/%s", gitUsername, repoName),
		)
		if err != nil {
			return err
		}
		if !strings.Contains(output, repoName) {
			return fmt.Errorf("repository %q not found in API response", repoName)
		}
		return nil
	}, "30s", "5s").Should(Succeed())

	return Repo{
		server:   s,
		repoName: repoName,
	}
}
