package gitserver

import (
	"encoding/base64"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	argocdutil "github.com/argoproj-labs/argocd-operator/controllers/argocd"
	osFixture "github.com/argoproj-labs/argocd-operator/tests/ginkgo/fixture/os"
	corev1 "k8s.io/api/core/v1"
)

const (
	giteaImage      = "ghcr.io/go-gitea/gitea:1.26.4-rootless"
	giteaCustomPath = "/data/gitea"
)

func giteaEnvVars(domain, internalToken string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "USER_UID", Value: "1000"},
		{Name: "USER_GID", Value: "1000"},
		{Name: "GITEA_CUSTOM", Value: giteaCustomPath},
		{Name: "GITEA_APP_NAME", Value: "Gitea E2E Git Server"},
		{Name: "GITEA__run_mode", Value: "prod"},
		{Name: "GITEA__database__DB_TYPE", Value: "sqlite3"},
		{Name: "GITEA__database__PATH", Value: "/data/gitea/gitea.db"},
		{Name: "GITEA__repository__DEFAULT_BRANCH", Value: "main"},
		{Name: "GITEA__server__DOMAIN", Value: domain},
		{Name: "GITEA__server__SSH_DOMAIN", Value: domain},
		{Name: "GITEA__server__SSH_USER", Value: giteaSSHLogin},
		{Name: "GITEA__server__HTTP_PORT", Value: fmt.Sprintf("%d", httpPort)},
		{Name: "GITEA__server__ROOT_URL", Value: fmt.Sprintf("https://%s:%d/", domain, httpPort)},
		{Name: "GITEA__server__PROTOCOL", Value: "https"},
		{Name: "GITEA__server__CERT_FILE", Value: "/etc/gitea/certs/tls.crt"},
		{Name: "GITEA__server__KEY_FILE", Value: "/etc/gitea/certs/tls.key"},
		{Name: "GITEA__server__LOCAL_ROOT_URL", Value: fmt.Sprintf("https://127.0.0.1:%d/", httpPort)},
		{Name: "GITEA__server__START_SSH_SERVER", Value: "true"},
		{Name: "GITEA__server__SSH_PORT", Value: fmt.Sprintf("%d", sshServicePort)},
		{Name: "GITEA__server__SSH_LISTEN_PORT", Value: fmt.Sprintf("%d", sshPort)},
		{Name: "GITEA__server__DISABLE_SSH", Value: "false"},
		{Name: "GITEA__server__LFS_START_SERVER", Value: "false"},
		{Name: "GITEA__security__INSTALL_LOCK", Value: "true"},
		{Name: "GITEA__security__INTERNAL_TOKEN", Value: internalToken},
		{Name: "GITEA__security__SECRET_KEY", Value: argocdutil.GenerateRandomString(24)},
		{Name: "GITEA__oauth2__JWT_SECRET", Value: argocdutil.GenerateRandomString(24)},
		{Name: "GITEA__lfs__LFS_JWT_SECRET", Value: argocdutil.GenerateRandomString(24)},
		{Name: "GITEA__service__DISABLE_REGISTRATION", Value: "true"},
		{Name: "GITEA__service__REQUIRE_SIGNIN_VIEW", Value: "false"},
		{Name: "GITEA__mailer__ENABLED", Value: "false"},
		{Name: "GITEA__openid__ENABLE_OPENID_SIGNIN", Value: "false"},
		{Name: "GITEA__openid__ENABLE_OPENID_SIGNUP", Value: "false"},
	}
}

func configureGiteaAdmin(server Server) {
	By("configuring Gitea admin user and SSH key")

	Eventually(func() error {
		_, err := execInGiteaPod(server.namespace, "gitea", "admin", "user", "list")
		return err
	}, "30s", "5s").Should(Succeed())

	Eventually(func() error {
		out, err := execInGiteaPod(server.namespace,
			"gitea", "admin", "user", "create",
			"--username", gitUsername,
			"--password", server.httpPassword,
			"--email", "gituser@test.local",
			"--admin",
			"--must-change-password=false",
		)
		if err != nil && (strings.Contains(out, "already exists") || strings.Contains(out, "user already")) {
			return nil
		}
		if err != nil {
			GinkgoWriter.Printf("gitea admin user create failed: %v: %s\n", err, out)
		}
		return err
	}, "30s", "5s").Should(Succeed())

	Eventually(func() error {
		out, err := giteaAPIPost(server.namespace, server.httpPassword,
			"/api/v1/user/keys",
			fmt.Sprintf(`{"title":"e2e-git-ssh-key","key":"%s"}`, strings.TrimSpace(server.sshPublicKey)),
		)
		if err != nil && strings.Contains(out, "already") {
			return nil
		}
		if err != nil {
			GinkgoWriter.Printf("gitea add SSH key failed: %v: %s\n", err, out)
		}
		return err
	}, "30s", "5s").Should(Succeed())
}

func execInGiteaPod(namespace string, args ...string) (string, error) {
	execArgs := []string{"kubectl", "exec", "-n", namespace, "pod/" + serverName, "-c", serverName, "--"}
	execArgs = append(execArgs, args...)
	return osFixture.ExecCommandWithOutputParam(false, false, execArgs...)
}

func giteaAPIPost(namespace, password, path, body string) (string, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(gitUsername + ":" + password))
	return execInGiteaPod(namespace, giteaWgetArgs("Basic "+auth, "POST", path, body)...)
}

func giteaAPIGet(namespace, password, path string) (string, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(gitUsername + ":" + password))
	return execInGiteaPod(namespace, giteaWgetArgs("Basic "+auth, "GET", path, "")...)
}

func giteaWgetArgs(auth, method, path, body string) []string {
	args := []string{
		"wget",
		"-q", "--no-check-certificate",
		"--header=Authorization: " + auth,
		"-O", "-",
	}
	if method == "POST" {
		args = append(args,
			"--header=Content-Type: application/json",
			"--post-data="+body,
		)
	}
	args = append(args, fmt.Sprintf("https://127.0.0.1:%d%s", httpPort, path))
	return args
}
